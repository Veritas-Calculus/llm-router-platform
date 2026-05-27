// Package captcha provides a pluggable interface for server-side captcha
// verification with multiple backends (Cloudflare Turnstile, hCaptcha, and
// a dev backend for local development).
//
// The Service.Verify method is the single entry point used by registration
// and other anti-abuse paths. The concrete backend is selected at startup
// via the CAPTCHA_PROVIDER environment variable and is opaque to callers.
//
// SECURITY: when a real backend is configured (turnstile / hcaptcha), an
// empty or invalid token always rejects. The dev backend exists purely so
// local docker-compose runs don't require a Cloudflare account; it must
// never be the default in production deployments.
package captcha

import (
	"context"
	"fmt"
	"strings"
	"time"

	"llm-router-platform/internal/service/turnstile"
	"llm-router-platform/pkg/sanitize"

	"go.uber.org/zap"
)

// Provider identifies the active captcha backend.
type Provider string

const (
	// ProviderDev accepts a single literal bypass token. Intended for local
	// docker-compose development only.
	ProviderDev Provider = "dev"
	// ProviderTurnstile uses Cloudflare Turnstile siteverify.
	ProviderTurnstile Provider = "turnstile"
	// ProviderHCaptcha uses hCaptcha siteverify.
	ProviderHCaptcha Provider = "hcaptcha"
	// ProviderDisabled accepts every request without validation. Different
	// from "dev" — dev requires the literal bypass token; disabled accepts
	// the empty string too. Useful for tests and certain self-hosted setups
	// behind a strict private network.
	ProviderDisabled Provider = "disabled"
)

// Config holds the captcha service configuration. SiteKey is returned to
// the frontend via the public captchaConfig query so the SPA can render
// the matching widget.
type Config struct {
	Provider   Provider
	SiteKey    string
	SecretKey  string
	DevBypass  string // literal accepted by ProviderDev
}

// Service verifies captcha tokens against the configured backend.
type Service struct {
	cfg       Config
	turnstile *turnstile.Service // reused when Provider == ProviderTurnstile
	hcaptcha  *hcaptchaBackend   // lazily allocated for ProviderHCaptcha
	logger    *zap.Logger
}

// New returns a new captcha service. If cfg.Provider is unknown or empty,
// the service falls back to ProviderDev (with a warning).
func New(logger *zap.Logger, cfg Config) *Service {
	switch cfg.Provider {
	case ProviderTurnstile, ProviderHCaptcha, ProviderDev, ProviderDisabled:
		// known provider, fall through
	default:
		logger.Warn("unknown CAPTCHA_PROVIDER, falling back to dev",
			zap.String("provider", sanitize.LogValue(string(cfg.Provider))))
		cfg.Provider = ProviderDev
	}
	s := &Service{cfg: cfg, logger: logger}
	if cfg.Provider == ProviderTurnstile {
		s.turnstile = turnstile.New(logger, true, cfg.SecretKey)
	}
	if cfg.Provider == ProviderHCaptcha {
		s.hcaptcha = newHCaptchaBackend(logger, cfg.SecretKey, 5*time.Second)
	}
	if cfg.Provider == ProviderDev && cfg.DevBypass == "" {
		// Force a non-empty default so the bypass token can't be matched by an
		// empty payload from a misbehaving client.
		s.cfg.DevBypass = "dev-ok"
	}
	return s
}

// Provider returns the configured provider name (for diagnostics / public
// captcha config endpoint).
func (s *Service) Provider() Provider { return s.cfg.Provider }

// SiteKey returns the public site key used by the frontend widget. Empty
// string when the dev/disabled backend is active.
func (s *Service) SiteKey() string { return s.cfg.SiteKey }

// Enabled reports whether captcha verification is actually performed for
// real backends. The disabled backend reports false; dev reports true
// because it still requires a token match.
func (s *Service) Enabled() bool {
	switch s.cfg.Provider {
	case ProviderTurnstile, ProviderHCaptcha, ProviderDev:
		return true
	default:
		return false
	}
}

// Verify checks the captcha token. Returns an error to reject the request.
// remoteIP is the client IP if known — passed through to the upstream
// siteverify for additional anti-replay protection.
func (s *Service) Verify(ctx context.Context, token, remoteIP string) error {
	switch s.cfg.Provider {
	case ProviderDisabled:
		return nil
	case ProviderDev:
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("captcha token required")
		}
		if token != s.cfg.DevBypass {
			return fmt.Errorf("invalid captcha token")
		}
		return nil
	case ProviderTurnstile:
		// Forward to the existing turnstile package so we don't duplicate the
		// SSRF-safe HTTP client and error normalization. We call VerifyForced
		// instead of Verify because the captcha service has already decided a
		// solve is required at this point.
		return s.turnstile.VerifyForced(ctx, token, remoteIP)
	case ProviderHCaptcha:
		return s.hcaptcha.verify(ctx, token, remoteIP)
	default:
		// Unreachable due to constructor normalization.
		return fmt.Errorf("captcha provider not configured")
	}
}
