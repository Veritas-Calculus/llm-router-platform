// Package jwtsign provides a small abstraction over the JWT signing/verification
// machinery so the server can run on HS256 (the legacy default), RS256, or
// EdDSA without sprinkling algorithm-aware code throughout the codebase.
//
// Why this exists: the audit (SEC §5.4 M4) flagged that the platform's
// access + refresh tokens were signed with HS256 + a single static secret,
// so anyone with read access to the secret (env file, container image,
// backup, log accident) could forge tokens for any user. Asymmetric signing
// removes that single point of compromise — the signing key lives only where
// tokens are minted, while verifiers (middleware, resolvers) need only the
// public key.
//
// Design goals:
//   - Backwards compatible: an existing HS256 deployment keeps working with
//     no config change.
//   - Token header always carries `kid` so multiple verification keys can be
//     used concurrently (zero-downtime rotation).
//   - Verification accepts the current key + any number of previous keys so
//     a key rotation doesn't invalidate live sessions.
package jwtsign

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Algorithm names the supported JWT signature algorithms.
type Algorithm string

const (
	HS256 Algorithm = "HS256"
	RS256 Algorithm = "RS256"
	EdDSA Algorithm = "EdDSA"
)

// KeyMaterial holds one signing key with its identifier. For HS256 the key
// is a shared symmetric secret. For RS256/EdDSA it is the PEM-encoded
// private key for signing OR the PEM-encoded public key for verify-only use.
type KeyMaterial struct {
	KID        string    // header key id, e.g. "v1" or a UUID
	Algorithm  Algorithm
	PrivatePEM []byte    // only set on the signer's key
	PublicPEM  []byte    // required for verify-only keys (rotation, JWKS)
	HSSecret   []byte    // HS256 only
}

// Signer mints tokens with a fixed key + algorithm and verifies tokens against
// the configured key set (current + previous, for rotation).
type Signer struct {
	current    *resolvedKey
	verifyKeys map[string]*resolvedKey // kid → key
}

type resolvedKey struct {
	algorithm  Algorithm
	method     jwt.SigningMethod
	signingKey interface{} // *rsa.PrivateKey / ed25519.PrivateKey / []byte
	verifyKey  interface{} // *rsa.PublicKey  / ed25519.PublicKey  / []byte
	kid        string
}

// New builds a Signer. `current` is required (used for signing). `previous`
// is optional — any keys named here will be accepted at verify time so a
// rotation doesn't invalidate live tokens until they naturally expire.
func New(current KeyMaterial, previous ...KeyMaterial) (*Signer, error) {
	cur, err := resolve(current)
	if err != nil {
		return nil, fmt.Errorf("current key: %w", err)
	}
	verify := map[string]*resolvedKey{cur.kid: cur}
	for _, p := range previous {
		k, err := resolve(p)
		if err != nil {
			return nil, fmt.Errorf("previous key %s: %w", p.KID, err)
		}
		verify[k.kid] = k
	}
	return &Signer{current: cur, verifyKeys: verify}, nil
}

// FromHS256 is a convenience constructor mirroring the legacy single-secret
// HS256 deployment. Used by NewFromLegacySecret callers that haven't yet
// configured an explicit algorithm — keeps the rollout incremental.
func FromHS256(secret string) (*Signer, error) {
	if secret == "" {
		return nil, errors.New("HS256 secret is empty")
	}
	return New(KeyMaterial{KID: "legacy", Algorithm: HS256, HSSecret: []byte(secret)})
}

// Sign mints a token whose header includes `kid=<current.KID>` and `alg=<current.Algorithm>`.
func (s *Signer) Sign(claims jwt.Claims) (string, error) {
	tok := jwt.NewWithClaims(s.current.method, claims)
	tok.Header["kid"] = s.current.kid
	return tok.SignedString(s.current.signingKey)
}

// Parse verifies the token signature against any of the configured keys and
// returns the parsed token. The verifier picks the key whose `kid` matches
// the token header; if no kid is set (legacy tokens), it falls back to the
// current key — preserves compatibility with tokens minted before this
// package existed.
func (s *Signer) Parse(tokenStr string, claims jwt.Claims) (*jwt.Token, error) {
	allowed := make([]string, 0, len(s.verifyKeys))
	seen := map[string]bool{}
	for _, k := range s.verifyKeys {
		if !seen[string(k.algorithm)] {
			allowed = append(allowed, string(k.algorithm))
			seen[string(k.algorithm)] = true
		}
	}

	return jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// jwt.WithValidMethods already enforces the algorithm allow-list, so
		// we just need to pick the right key for the kid the token claims.
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return s.current.verifyKey, nil
		}
		k, ok := s.verifyKeys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid %q", kid)
		}
		if string(k.algorithm) != t.Method.Alg() {
			return nil, fmt.Errorf("kid %q expects %s, got %s", kid, k.algorithm, t.Method.Alg())
		}
		return k.verifyKey, nil
	}, jwt.WithValidMethods(allowed))
}

// CurrentKID returns the kid the next Sign call will stamp into the header.
// Useful for exposing a public JWKS endpoint that lists current + previous
// keys.
func (s *Signer) CurrentKID() string { return s.current.kid }

// VerifyPublicKey returns the public verification material for a given kid.
// Returns nil if unknown. The caller is responsible for encoding to JWK if
// it's serving a JWKS endpoint.
func (s *Signer) VerifyPublicKey(kid string) crypto.PublicKey {
	if k, ok := s.verifyKeys[kid]; ok {
		switch v := k.verifyKey.(type) {
		case *rsa.PublicKey:
			return v
		case ed25519.PublicKey:
			return v
		}
	}
	return nil
}

func resolve(m KeyMaterial) (*resolvedKey, error) {
	if m.KID == "" {
		return nil, errors.New("KID must be set")
	}
	switch m.Algorithm {
	case HS256:
		if len(m.HSSecret) == 0 {
			return nil, errors.New("HS256 requires HSSecret")
		}
		return &resolvedKey{
			algorithm:  HS256,
			method:     jwt.SigningMethodHS256,
			signingKey: m.HSSecret,
			verifyKey:  m.HSSecret,
			kid:        m.KID,
		}, nil
	case RS256:
		priv, pub, err := parseRSA(m.PrivatePEM, m.PublicPEM)
		if err != nil {
			return nil, err
		}
		return &resolvedKey{
			algorithm:  RS256,
			method:     jwt.SigningMethodRS256,
			signingKey: priv, // may be nil for verify-only keys
			verifyKey:  pub,
			kid:        m.KID,
		}, nil
	case EdDSA:
		priv, pub, err := parseEd25519(m.PrivatePEM, m.PublicPEM)
		if err != nil {
			return nil, err
		}
		return &resolvedKey{
			algorithm:  EdDSA,
			method:     jwt.SigningMethodEdDSA,
			signingKey: priv,
			verifyKey:  pub,
			kid:        m.KID,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", m.Algorithm)
	}
}

func parseRSA(privatePEM, publicPEM []byte) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	var (
		priv *rsa.PrivateKey
		pub  *rsa.PublicKey
	)
	if len(privatePEM) > 0 {
		block, _ := pem.Decode(privatePEM)
		if block == nil {
			return nil, nil, errors.New("private key PEM is not parseable")
		}
		var err error
		// Try PKCS1, fall back to PKCS8.
		priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				return nil, nil, fmt.Errorf("RSA private key: %v / %v", err, err2)
			}
			var ok bool
			priv, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				return nil, nil, errors.New("PKCS8 key is not RSA")
			}
		}
		pub = &priv.PublicKey
	}
	if len(publicPEM) > 0 {
		block, _ := pem.Decode(publicPEM)
		if block == nil {
			return nil, nil, errors.New("public key PEM is not parseable")
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("public key: %w", err)
		}
		p, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, nil, errors.New("public key is not RSA")
		}
		pub = p
	}
	if pub == nil {
		return nil, nil, errors.New("RS256 requires a private or public key PEM")
	}
	return priv, pub, nil
}

func parseEd25519(privatePEM, publicPEM []byte) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	var (
		priv ed25519.PrivateKey
		pub  ed25519.PublicKey
	)
	if len(privatePEM) > 0 {
		block, _ := pem.Decode(privatePEM)
		if block == nil {
			return nil, nil, errors.New("Ed25519 private key PEM is not parseable")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("Ed25519 private key: %w", err)
		}
		var ok bool
		priv, ok = parsed.(ed25519.PrivateKey)
		if !ok {
			return nil, nil, errors.New("PKCS8 key is not Ed25519")
		}
		pub = priv.Public().(ed25519.PublicKey)
	}
	if len(publicPEM) > 0 {
		block, _ := pem.Decode(publicPEM)
		if block == nil {
			return nil, nil, errors.New("Ed25519 public key PEM is not parseable")
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("Ed25519 public key: %w", err)
		}
		p, ok := parsed.(ed25519.PublicKey)
		if !ok {
			return nil, nil, errors.New("public key is not Ed25519")
		}
		pub = p
	}
	if pub == nil {
		return nil, nil, errors.New("EdDSA requires a private or public key PEM")
	}
	return priv, pub, nil
}
