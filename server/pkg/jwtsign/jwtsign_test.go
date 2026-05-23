package jwtsign

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func genRSA(t *testing.T) []byte {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genRSA: %v", err)
	}
	bytes := x509.MarshalPKCS1PrivateKey(k)
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: bytes})
}

func genEd25519(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genEd: %v", err)
	}
	bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: bytes})
}

type claims struct {
	jwt.RegisteredClaims
	Sub string `json:"sub"`
}

func makeClaims() *claims {
	return &claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Sub: "user-1",
	}
}

func TestRoundTripHS256(t *testing.T) {
	s, err := New(KeyMaterial{KID: "v1", Algorithm: HS256, HSSecret: []byte("super-secret-32-byte-key-zzzzzz")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok, err := s.Sign(makeClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	var got claims
	parsed, err := s.Parse(tok, &got)
	if err != nil || !parsed.Valid {
		t.Fatalf("Parse: %v valid=%v", err, parsed != nil && parsed.Valid)
	}
	if got.Sub != "user-1" {
		t.Fatalf("sub mismatch: %q", got.Sub)
	}
	if parsed.Header["kid"] != "v1" {
		t.Fatalf("kid not in header: %v", parsed.Header)
	}
}

func TestRoundTripRS256(t *testing.T) {
	s, err := New(KeyMaterial{KID: "rsa-1", Algorithm: RS256, PrivatePEM: genRSA(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok, err := s.Sign(makeClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	var got claims
	parsed, err := s.Parse(tok, &got)
	if err != nil || !parsed.Valid {
		t.Fatalf("Parse: %v valid=%v", err, parsed != nil && parsed.Valid)
	}
}

func TestRoundTripEd25519(t *testing.T) {
	s, err := New(KeyMaterial{KID: "ed-1", Algorithm: EdDSA, PrivatePEM: genEd25519(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok, err := s.Sign(makeClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	var got claims
	if _, err := s.Parse(tok, &got); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}

func TestKeyRotationAcceptsPrevious(t *testing.T) {
	// Mint with v1, then rotate to v2 keeping v1 as a verify-only key. A
	// pre-rotation token must still verify.
	priv1 := genRSA(t)
	priv2 := genRSA(t)

	old, err := New(KeyMaterial{KID: "v1", Algorithm: RS256, PrivatePEM: priv1})
	if err != nil {
		t.Fatalf("New old: %v", err)
	}
	tok, err := old.Sign(makeClaims())
	if err != nil {
		t.Fatalf("Sign old: %v", err)
	}

	// New signer: v2 current, v1 still accepted for verification.
	rotated, err := New(
		KeyMaterial{KID: "v2", Algorithm: RS256, PrivatePEM: priv2},
		KeyMaterial{KID: "v1", Algorithm: RS256, PrivatePEM: priv1},
	)
	if err != nil {
		t.Fatalf("New rotated: %v", err)
	}
	if _, err := rotated.Parse(tok, &claims{}); err != nil {
		t.Fatalf("rotated.Parse: %v", err)
	}
}

func TestUnknownKidRejected(t *testing.T) {
	priv1 := genRSA(t)
	priv2 := genRSA(t)
	old, _ := New(KeyMaterial{KID: "v1", Algorithm: RS256, PrivatePEM: priv1})
	tok, _ := old.Sign(makeClaims())

	// New signer that doesn't know about v1.
	notRotated, _ := New(KeyMaterial{KID: "v2", Algorithm: RS256, PrivatePEM: priv2})
	if _, err := notRotated.Parse(tok, &claims{}); err == nil {
		t.Fatal("expected unknown-kid rejection")
	}
}

func TestAlgorithmMismatch(t *testing.T) {
	hs, _ := New(KeyMaterial{KID: "v1", Algorithm: HS256, HSSecret: []byte("super-secret-32-byte-key-zzzzzz")})
	tok, _ := hs.Sign(makeClaims())

	rs, _ := New(KeyMaterial{KID: "v1", Algorithm: RS256, PrivatePEM: genRSA(t)})
	if _, err := rs.Parse(tok, &claims{}); err == nil {
		t.Fatal("expected algorithm mismatch rejection")
	}
}
