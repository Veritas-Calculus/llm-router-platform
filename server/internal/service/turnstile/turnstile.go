// Package turnstile verifies Cloudflare Turnstile CAPTCHA tokens.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"llm-router-platform/pkg/sanitize"

	"go.uber.org/zap"
)

const verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Service validates Cloudflare Turnstile tokens server-side.
type Service struct {
	secretKey string
	enabled   bool
	client    *http.Client
	logger    *zap.Logger
}

// New creates a Turnstile verification service. Uses the SSRF-safe HTTP
// client so a misconfiguration that ever pointed verifyURL elsewhere can't
// pivot to internal services via DNS rebinding.
func New(logger *zap.Logger, enabled bool, secretKey string) *Service {
	return &Service{
		secretKey: secretKey,
		enabled:   enabled,
		client:    sanitize.SafeHTTPClient(false, 5*time.Second),
		logger:    logger,
	}
}

// verifyResponse represents Cloudflare's siteverify API response.
type verifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Enabled reports whether Turnstile is configured. Used by the login flow to
// decide whether to even attempt a verification call when the soft per-email
// failure counter has not been tripped.
func (s *Service) Enabled() bool { return s.enabled }

// VerifyForced behaves like Verify but never short-circuits on !enabled —
// the caller (login limiter) has decided that a CAPTCHA solve is required
// regardless of the global Turnstile-disabled config, typically because
// the per-email failure threshold has been crossed. If Turnstile isn't
// configured at all (no secret key), VerifyForced refuses the request
// rather than silently accepting — that's the safe default for the
// "brute-force defense kicked in" path.
func (s *Service) VerifyForced(ctx context.Context, token string, remoteIP string) error {
	if s.secretKey == "" {
		return fmt.Errorf("too many failed attempts; CAPTCHA required but not configured on the server")
	}
	return s.verify(ctx, token, remoteIP)
}

// Verify validates a Turnstile token with Cloudflare's siteverify endpoint.
// If Turnstile is disabled, it always returns nil (permits the request).
func (s *Service) Verify(ctx context.Context, token string, remoteIP string) error {
	if !s.enabled {
		return nil // Turnstile not configured, skip verification
	}
	return s.verify(ctx, token, remoteIP)
}

func (s *Service) verify(ctx context.Context, token string, remoteIP string) error {
	if token == "" {
		return fmt.Errorf("CAPTCHA verification required")
	}

	form := url.Values{
		"secret":   {s.secretKey},
		"response": {token},
	}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		s.logger.Error("failed to create turnstile verify request", zap.Error(err))
		return fmt.Errorf("CAPTCHA verification failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("turnstile verify request failed", zap.Error(err))
		// Fail closed: if we can't reach Cloudflare, reject the request
		return fmt.Errorf("CAPTCHA verification failed")
	}
	defer func() { _ = resp.Body.Close() }()

	var result verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.logger.Error("failed to decode turnstile response", zap.Error(err))
		return fmt.Errorf("CAPTCHA verification failed")
	}

	if !result.Success {
		s.logger.Warn("turnstile verification failed",
			zap.Strings("errors", result.ErrorCodes),
			zap.String("remote_ip", sanitize.LogValue(sanitize.MaskIP(remoteIP))))
		return fmt.Errorf("CAPTCHA verification failed, please try again")
	}

	return nil
}
