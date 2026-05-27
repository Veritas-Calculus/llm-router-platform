package captcha

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

// hCaptcha siteverify endpoint. Documented at https://docs.hcaptcha.com/.
const hcaptchaVerifyURL = "https://api.hcaptcha.com/siteverify"

// hcaptchaBackend verifies hCaptcha tokens via the public siteverify API.
type hcaptchaBackend struct {
	secretKey string
	client    *http.Client
	logger    *zap.Logger
}

type hcaptchaResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func newHCaptchaBackend(logger *zap.Logger, secret string, timeout time.Duration) *hcaptchaBackend {
	// SSRF-safe client: siteverify endpoint is public, but we still funnel
	// through SafeHTTPClient so a future env-driven URL override can't
	// pivot to a private address.
	return &hcaptchaBackend{
		secretKey: secret,
		client:    sanitize.SafeHTTPClient(false, timeout),
		logger:    logger,
	}
}

func (h *hcaptchaBackend) verify(ctx context.Context, token, remoteIP string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("captcha token required")
	}
	if h.secretKey == "" {
		return fmt.Errorf("captcha backend not configured")
	}

	form := url.Values{
		"secret":   {h.secretKey},
		"response": {token},
	}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hcaptchaVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		h.logger.Error("hcaptcha request build failed", zap.Error(err))
		return fmt.Errorf("captcha verification failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("hcaptcha siteverify request failed", zap.Error(err))
		return fmt.Errorf("captcha verification failed")
	}
	defer func() { _ = resp.Body.Close() }()

	var out hcaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		h.logger.Error("hcaptcha response decode failed", zap.Error(err))
		return fmt.Errorf("captcha verification failed")
	}
	if !out.Success {
		h.logger.Warn("hcaptcha verification failed",
			zap.Strings("error_codes", out.ErrorCodes),
			zap.String("remote_ip", sanitize.LogValue(sanitize.MaskIP(remoteIP))))
		return fmt.Errorf("captcha verification failed, please try again")
	}
	return nil
}
