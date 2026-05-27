// Package captcha provides a pluggable interface for server-side captcha
// verification with multiple backends (Cloudflare Turnstile, hCaptcha, and
// a dev backend for local development).
//
// The Service.Verify method is the single entry point used by registration
// and other anti-abuse paths. The concrete backend is chosen at request
// time via the settings.Registry (DB-backed, 5-min cached). Credentials
// (site key / secret key / dev bypass) stay in env vars because they're
// paired-with-secrets and not safe to push through the admin UI.
//
// SECURITY: when a real backend is configured (turnstile / hcaptcha), an
// empty or invalid token always rejects. The dev backend exists purely so
// local docker-compose runs don't require a Cloudflare account; it must
// never be the default in production deployments.
//
// Fail-closed: if the Registry returns a provider whose backend can't be
// initialized (e.g. switched to hcaptcha but no CAPTCHA_SECRET_KEY in env),
// Verify rejects every token with a clear log warning. We deliberately do
// NOT silently fall back to dev — that would let a misconfigured admin
// disable captcha by typo.
package captcha

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// Config holds the captcha service's static credentials. The active
// Provider is no longer carried here — it lives in the DB-backed
// settings.Registry and is read per-Verify. SiteKey is what the frontend
// renders the widget with; SecretKey is the server-side credential.
type Config struct {
	SiteKey   string
	SecretKey string
	DevBypass string // literal accepted by ProviderDev
}

// providerSource is the contract the captcha service uses to resolve the
// active provider. The real implementation is *settings.Registry; tests
// inject a fake that returns a constant string.
type providerSource interface {
	CaptchaProvider(ctx context.Context) string
}

// staticProvider is the default providerSource used when no Registry is
// wired — returns a single literal. Used in tests that build a Service
// without bringing up the full settings stack.
type staticProvider struct{ value string }

func (s staticProvider) CaptchaProvider(_ context.Context) string { return s.value }

// Service verifies captcha tokens against the configured backend.
type Service struct {
	cfg      Config
	provider providerSource

	// Backends are lazily allocated on first use and cached. Switching
	// providers via the admin UI is a no-op until the next Verify call;
	// no shutdown is required because both turnstile and hcaptcha are
	// stateless HTTP clients.
	mu        sync.Mutex
	turnstile *turnstile.Service
	hcaptcha  *hcaptchaBackend

	logger *zap.Logger
}

// New returns a new captcha service. The provider field is resolved per-
// Verify from the supplied providerSource (typically a settings.Registry).
// Pass nil for source to fall back to the dev provider — useful for unit
// tests that don't need the full settings stack.
func New(logger *zap.Logger, cfg Config, source providerSource) *Service {
	if source == nil {
		source = staticProvider{value: string(ProviderDev)}
	}
	if cfg.DevBypass == "" {
		// Force a non-empty default so the bypass token can't be matched
		// by an empty payload from a misbehaving client.
		cfg.DevBypass = "dev-ok"
	}
	return &Service{cfg: cfg, provider: source, logger: logger}
}

// resolveProvider returns the active provider after normalization. Unknown
// values are treated as ProviderDev with a warning — symmetric with the
// previous constructor behaviour.
func (s *Service) resolveProvider(ctx context.Context) Provider {
	raw := strings.ToLower(strings.TrimSpace(s.provider.CaptchaProvider(ctx)))
	switch Provider(raw) {
	case ProviderTurnstile, ProviderHCaptcha, ProviderDev, ProviderDisabled:
		return Provider(raw)
	}
	s.logger.Warn("captcha: unknown provider from settings, falling back to dev",
		zap.String("provider", sanitize.LogValue(raw)))
	return ProviderDev
}

// ProviderForCurrent returns the active provider name. Used by the public
// CaptchaConfig GraphQL resolver so the SPA can pick the matching widget.
func (s *Service) ProviderForCurrent(ctx context.Context) Provider {
	return s.resolveProvider(ctx)
}

// SiteKey returns the public site key used by the frontend widget.
func (s *Service) SiteKey() string { return s.cfg.SiteKey }

// EnabledForCurrent reports whether captcha verification is actually
// performed under the *current* provider. Disabled reports false; dev
// reports true because it still requires a token match.
func (s *Service) EnabledForCurrent(ctx context.Context) bool {
	switch s.resolveProvider(ctx) {
	case ProviderTurnstile, ProviderHCaptcha, ProviderDev:
		return true
	default:
		return false
	}
}

// Verify checks the captcha token. Returns an error to reject the request.
// remoteIP is the client IP if known — passed through to the upstream
// siteverify for additional anti-replay protection.
//
// On uninitialized backends (e.g. provider=hcaptcha but no SecretKey
// configured) Verify rejects fail-closed with a logged warning. The
// alternative — silently falling back to dev — would let a typo in the
// admin UI disable captcha across the platform.
func (s *Service) Verify(ctx context.Context, token, remoteIP string) error {
	p := s.resolveProvider(ctx)
	switch p {
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
		backend := s.getTurnstile()
		if backend == nil {
			s.logger.Warn("captcha: turnstile selected but no secret key configured; rejecting fail-closed")
			return fmt.Errorf("captcha verification required")
		}
		return backend.VerifyForced(ctx, token, remoteIP)
	case ProviderHCaptcha:
		backend := s.getHCaptcha()
		if backend == nil {
			s.logger.Warn("captcha: hcaptcha selected but no secret key configured; rejecting fail-closed")
			return fmt.Errorf("captcha verification required")
		}
		return backend.verify(ctx, token, remoteIP)
	default:
		// Unreachable: resolveProvider normalizes to one of the four above.
		return fmt.Errorf("captcha provider not configured")
	}
}

// getTurnstile returns the cached turnstile backend, creating one on
// first use. Returns nil if no SecretKey is configured (fail-closed path).
func (s *Service) getTurnstile() *turnstile.Service {
	if s.cfg.SecretKey == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turnstile == nil {
		s.turnstile = turnstile.New(s.logger, true, s.cfg.SecretKey)
	}
	return s.turnstile
}

// getHCaptcha returns the cached hcaptcha backend, creating one on first
// use. Returns nil if no SecretKey is configured.
func (s *Service) getHCaptcha() *hcaptchaBackend {
	if s.cfg.SecretKey == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hcaptcha == nil {
		s.hcaptcha = newHCaptchaBackend(s.logger, s.cfg.SecretKey, 5*time.Second)
	}
	return s.hcaptcha
}
