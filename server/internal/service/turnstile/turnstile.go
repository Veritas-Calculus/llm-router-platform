// Package turnstile verifies Cloudflare Turnstile CAPTCHA tokens.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// New creates a Turnstile verification service.
func New(logger *zap.Logger, enabled bool, secretKey string) *Service {
	return &Service{
		secretKey: secretKey,
		enabled:   enabled,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// verifyResponse represents Cloudflare's siteverify API response.
type verifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify validates a Turnstile token with Cloudflare's siteverify endpoint.
// If Turnstile is disabled, it always returns nil (permits the request).
func (s *Service) Verify(ctx context.Context, token string, remoteIP string) error {
	if !s.enabled {
		return nil // Turnstile not configured, skip verification
	}

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, nil)
	if err != nil {
		s.logger.Error("failed to create turnstile verify request", zap.Error(err))
		return fmt.Errorf("CAPTCHA verification failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = http.NoBody
	// Use PostForm-style body
	resp, err := s.client.PostForm(verifyURL, form)
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
			zap.String("remote_ip", sanitize.MaskIP(remoteIP)))
		return fmt.Errorf("CAPTCHA verification failed, please try again")
	}

	return nil
}

