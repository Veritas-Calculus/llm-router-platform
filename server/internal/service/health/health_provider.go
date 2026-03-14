package health

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetProvidersHealth returns health status of all active providers.
func (s *Service) GetProvidersHealth(ctx context.Context) ([]ProviderHealthStatus, error) {
	providers, err := s.providerRepo.GetActive(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]ProviderHealthStatus, len(providers))
	for i, p := range providers {
		history, _ := s.healthHistoryRepo.GetByTarget(ctx, "provider", p.ID, 10)

		successCount := 0
		var lastCheck time.Time
		var lastResponseTime int64
		isHealthy := true
		var errorMsg string

		for j, h := range history {
			if j == 0 {
				lastCheck = h.CheckedAt
				lastResponseTime = h.ResponseTime
				isHealthy = h.IsHealthy
				errorMsg = h.ErrorMessage
			}
			if h.IsHealthy {
				successCount++
			}
		}

		successRate := float64(0)
		if len(history) > 0 {
			successRate = float64(successCount) / float64(len(history))
		}

		statuses[i] = ProviderHealthStatus{
			ID:           p.ID,
			Name:         p.Name,
			BaseURL:      p.BaseURL,
			IsActive:     p.IsActive,
			IsHealthy:    isHealthy,
			UseProxy:     p.UseProxy,
			ResponseTime: lastResponseTime,
			LastCheck:    lastCheck,
			SuccessRate:  successRate,
			ErrorMessage: errorMsg,
		}
	}

	return statuses, nil
}

// CheckSingleProvider checks health of a specific provider.
// It uses one of the provider's API keys to create a client and test connectivity.
func (s *Service) CheckSingleProvider(ctx context.Context, id uuid.UUID) (*ProviderHealthStatus, error) {
	p, err := s.providerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var healthy bool
	var latency time.Duration
	var errorMsg string

	// Get an active API key for this provider (if it requires one)
	var apiKey *models.ProviderAPIKey
	if p.RequiresAPIKey {
		keys, err := s.providerKeyRepo.GetActiveByProvider(ctx, p.ID)
		if err != nil || len(keys) == 0 {
			healthy = false
			errorMsg = "no active API keys for provider"
		} else {
			apiKey = &keys[0] // Use the first active key for health check
		}
	}

	if errorMsg == "" {
		s.logger.Info("creating provider client for health check",
			zap.String("provider", p.Name),
			zap.Bool("has_api_key", apiKey != nil),
			zap.String("base_url", p.BaseURL),
			zap.Bool("use_proxy", p.UseProxy))

		// Create client dynamically
		client, err := s.getProviderClient(p, apiKey)
		if err != nil {
			healthy = false
			errorMsg = "failed to create provider client: " + err.Error()
			s.logger.Error("failed to create provider client", zap.Error(err))
		} else {
			// Check health using proxy if enabled
			if p.UseProxy {
				s.logger.Info("checking health with proxy", zap.String("provider", p.Name))
				healthy, latency, errorMsg = s.checkWithProxy(ctx, p, apiKey)
			} else {
				s.logger.Info("checking health directly", zap.String("provider", p.Name))
				healthy, latency, err = client.CheckHealth(ctx)
				if err != nil {
					errorMsg = err.Error()
					s.logger.Error("health check failed", zap.String("provider", p.Name), zap.Error(err))
				} else {
					s.logger.Info("health check completed", zap.String("provider", p.Name), zap.Bool("healthy", healthy), zap.Duration("latency", latency))
				}
			}
		}
	}

	history := &models.HealthHistory{
		TargetType:   "provider",
		TargetID:     p.ID,
		IsHealthy:    healthy,
		ResponseTime: latency.Milliseconds(),
		ErrorMessage: errorMsg,
		CheckedAt:    time.Now(),
	}
	_ = s.healthHistoryRepo.Create(ctx, history)

	if !healthy && s.alertNotifier != nil {
		_ = s.alertNotifier.Notify(ctx, "provider", p.ID, "health_check_failed", "Provider health check failed: "+errorMsg)
	}

	successRate := s.calculateSuccessRate(ctx, "provider", p.ID)

	return &ProviderHealthStatus{
		ID:           p.ID,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		IsActive:     p.IsActive,
		IsHealthy:    healthy,
		UseProxy:     p.UseProxy,
		ResponseTime: latency.Milliseconds(),
		LastCheck:    time.Now(),
		SuccessRate:  successRate,
		ErrorMessage: errorMsg,
	}, nil
}

// checkWithProxy performs a health check using a proxy.
func (s *Service) checkWithProxy(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey) (bool, time.Duration, string) {
	var proxyInfo *models.Proxy

	// Use provider's default proxy if set, otherwise use any active proxy
	if p.DefaultProxyID != nil {
		proxy, err := s.proxyRepo.GetByID(ctx, *p.DefaultProxyID)
		if err != nil {
			s.logger.Warn("failed to get default proxy, falling back to active proxies",
				zap.String("provider", p.Name),
				zap.String("proxy_id", p.DefaultProxyID.String()),
				zap.Error(err))
		} else if proxy.IsActive {
			proxyInfo = proxy
		}
	}

	// If no default proxy or it's inactive, get any active proxy
	if proxyInfo == nil {
		proxies, err := s.proxyRepo.GetActive(ctx)
		if err != nil || len(proxies) == 0 {
			return false, 0, "no active proxy available"
		}
		proxyInfo = &proxies[0]
	}

	proxyURL, err := url.Parse(proxyInfo.URL)
	if err != nil {
		return false, 0, "invalid proxy URL"
	}

	s.logger.Info("using proxy for health check",
		zap.String("provider", p.Name),
		zap.String("proxy_url", proxyInfo.URL))

	// Create HTTP client with proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(p.Timeout) * time.Second,
	}

	// Decrypt API key if available
	var decryptedKey string
	if apiKey != nil && apiKey.EncryptedAPIKey != "" {
		decryptedKey, err = crypto.Decrypt(apiKey.EncryptedAPIKey)
		if err != nil {
			return false, 0, "failed to decrypt API key: " + err.Error()
		}
	}

	// Determine health check endpoint based on provider
	healthURL := s.resolveHealthURL(p.Name, p.BaseURL, decryptedKey)

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false, 0, err.Error()
	}

	// Add authorization headers for providers that need them
	s.setAuthHeaders(req, p.Name, decryptedKey)

	resp, err := httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return false, latency, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	return s.evaluateHealthResponse(p.Name, decryptedKey, resp, latency)
}

// resolveHealthURL returns the health check endpoint for a given provider.
func (s *Service) resolveHealthURL(providerName, baseURL, decryptedKey string) string {
	switch providerName {
	case "openai", "lmstudio", "vllm":
		return baseURL + "/models"
	case "ollama":
		return baseURL + "/api/tags"
	case "anthropic":
		return baseURL + "/v1/messages"
	case "google":
		u := baseURL + "/v1beta/models"
		if decryptedKey != "" {
			u += "?key=" + decryptedKey
		}
		return u
	default:
		return baseURL + "/models"
	}
}

// setAuthHeaders adds appropriate authorization headers for the given provider.
func (s *Service) setAuthHeaders(req *http.Request, providerName, decryptedKey string) {
	switch providerName {
	case "openai", "lmstudio", "vllm":
		if decryptedKey != "" {
			req.Header.Set("Authorization", "Bearer "+decryptedKey)
		}
	case "anthropic":
		if decryptedKey != "" {
			req.Header.Set("x-api-key", decryptedKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	}
}

// evaluateHealthResponse interprets the HTTP response for a health check.
func (s *Service) evaluateHealthResponse(providerName, decryptedKey string, resp *http.Response, latency time.Duration) (bool, time.Duration, string) {
	// For Anthropic without API key, a 401 means the endpoint is reachable
	if providerName == "anthropic" && decryptedKey == "" && resp.StatusCode == http.StatusUnauthorized {
		return true, latency, ""
	}

	// For Google without API key, a 400/403 means the endpoint is reachable
	if providerName == "google" && decryptedKey == "" && (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusForbidden) {
		return true, latency, ""
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return false, latency, "API returned status " + resp.Status + ": " + string(respBody)
	}

	return true, latency, ""
}

// CheckAllProviders runs health checks on all active providers.
func (s *Service) CheckAllProviders(ctx context.Context) error {
	providers, err := s.providerRepo.GetActive(ctx)
	if err != nil {
		return err
	}

	for _, p := range providers {
		_, _ = s.CheckSingleProvider(ctx, p.ID)
	}

	return nil
}
