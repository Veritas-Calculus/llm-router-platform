package middleware

import (
	"fmt"
	"strings"

	"llm-router-platform/internal/config"
	"llm-router-platform/pkg/jwtsign"
)

// BuildJWTSigner constructs the process-wide JWT signer from JWTConfig. When
// no algorithm is set (the default) it falls back to HS256 with the legacy
// JWT.Secret so existing deployments keep working without a config change.
//
// To migrate to RS256/EdDSA, see docs/jwt-asymmetric-rotation.md.
func BuildJWTSigner(cfg config.JWTConfig) (*jwtsign.Signer, error) {
	algo := strings.ToUpper(strings.TrimSpace(cfg.Algorithm))
	if algo == "" {
		algo = "HS256"
	}

	switch algo {
	case "HS256":
		if cfg.Secret == "" {
			return nil, fmt.Errorf("JWT.Secret is required for HS256")
		}
		kid := cfg.KeyID
		if kid == "" {
			kid = "legacy"
		}
		return jwtsign.New(jwtsign.KeyMaterial{KID: kid, Algorithm: jwtsign.HS256, HSSecret: []byte(cfg.Secret)})

	case "RS256":
		return buildAsymmetric(cfg, jwtsign.RS256)

	case "EDDSA", "ED25519":
		return buildAsymmetric(cfg, jwtsign.EdDSA)
	}

	return nil, fmt.Errorf("unsupported JWT.Algorithm %q", cfg.Algorithm)
}

func buildAsymmetric(cfg config.JWTConfig, algo jwtsign.Algorithm) (*jwtsign.Signer, error) {
	if cfg.PrivateKeyPEM == "" {
		return nil, fmt.Errorf("JWT.PrivateKeyPEM is required for %s", algo)
	}
	if cfg.KeyID == "" {
		return nil, fmt.Errorf("JWT.KeyID is required for asymmetric signing")
	}
	current := jwtsign.KeyMaterial{
		KID:        cfg.KeyID,
		Algorithm:  algo,
		PrivatePEM: []byte(cfg.PrivateKeyPEM),
		PublicPEM:  []byte(cfg.PublicKeyPEM),
	}
	previous := make([]jwtsign.KeyMaterial, 0, len(cfg.PreviousKeysPEM))
	for kid, pem := range cfg.PreviousKeysPEM {
		previous = append(previous, jwtsign.KeyMaterial{
			KID:        kid,
			Algorithm:  algo,
			PublicPEM:  []byte(pem),
		})
	}
	return jwtsign.New(current, previous...)
}
