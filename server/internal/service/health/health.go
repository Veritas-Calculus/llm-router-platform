// Package health provides health check functionality.
package health

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles health checks for API keys and proxies.
type Service struct {
	apiKeyRepo        *repository.APIKeyRepository
	providerKeyRepo   *repository.ProviderAPIKeyRepository
	proxyRepo         *repository.ProxyRepository
	providerRepo      *repository.ProviderRepository
	healthHistoryRepo *repository.HealthHistoryRepository
	alertNotifier     *AlertNotifier
	providerRegistry  *provider.Registry
	proxyService      *proxy.Service
	logger            *zap.Logger
}

// NewService creates a new health service.
func NewService(
	apiKeyRepo *repository.APIKeyRepository,
	providerKeyRepo *repository.ProviderAPIKeyRepository,
	proxyRepo *repository.ProxyRepository,
	providerRepo *repository.ProviderRepository,
	healthHistoryRepo *repository.HealthHistoryRepository,
	alertNotifier *AlertNotifier,
	providerRegistry *provider.Registry,
	proxyService *proxy.Service,
	logger *zap.Logger,
) *Service {
	return &Service{
		apiKeyRepo:        apiKeyRepo,
		providerKeyRepo:   providerKeyRepo,
		proxyRepo:         proxyRepo,
		providerRepo:      providerRepo,
		healthHistoryRepo: healthHistoryRepo,
		alertNotifier:     alertNotifier,
		providerRegistry:  providerRegistry,
		proxyService:      proxyService,
		logger:            logger,
	}
}

// APIKeyHealthStatus represents health status of an API key.
type APIKeyHealthStatus struct {
	ID           uuid.UUID `json:"id"`
	ProviderName string    `json:"provider_name"`
	KeyPrefix    string    `json:"key_prefix"`
	IsActive     bool      `json:"is_active"`
	IsHealthy    bool      `json:"is_healthy"`
	LastCheck    time.Time `json:"last_check"`
	ResponseTime int64     `json:"response_time"`
	SuccessRate  float64   `json:"success_rate"`
}

// ProxyHealthStatus represents health status of a proxy.
type ProxyHealthStatus struct {
	ID           uuid.UUID `json:"id"`
	URL          string    `json:"url"`
	Type         string    `json:"type"`
	Region       string    `json:"region"`
	IsActive     bool      `json:"is_active"`
	IsHealthy    bool      `json:"is_healthy"`
	ResponseTime int64     `json:"response_time"`
	LastCheck    time.Time `json:"last_check"`
	SuccessRate  float64   `json:"success_rate"`
}

// ProviderHealthStatus represents health status of a provider.
type ProviderHealthStatus struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	BaseURL      string    `json:"base_url"`
	IsActive     bool      `json:"is_active"`
	IsHealthy    bool      `json:"is_healthy"`
	UseProxy     bool      `json:"use_proxy"`
	ResponseTime int64     `json:"response_time"`
	LastCheck    time.Time `json:"last_check"`
	SuccessRate  float64   `json:"success_rate"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// GetAPIKeysHealth returns health status of all API keys.
func (s *Service) GetAPIKeysHealth(ctx context.Context) ([]APIKeyHealthStatus, error) {
	keys, err := s.providerKeyRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]APIKeyHealthStatus, len(keys))
	for i, key := range keys {
		history, _ := s.healthHistoryRepo.GetByTarget(ctx, "api_key", key.ID, 10)

		successCount := 0
		var lastCheck time.Time
		var lastResponseTime int64
		isHealthy := true

		for j, h := range history {
			if j == 0 {
				lastCheck = h.CheckedAt
				lastResponseTime = h.ResponseTime
				isHealthy = h.IsHealthy
			}
			if h.IsHealthy {
				successCount++
			}
		}

		successRate := float64(0)
		if len(history) > 0 {
			successRate = float64(successCount) / float64(len(history))
		}

		statuses[i] = APIKeyHealthStatus{
			ID:           key.ID,
			ProviderName: key.Provider.Name,
			KeyPrefix:    key.KeyPrefix,
			IsActive:     key.IsActive,
			IsHealthy:    isHealthy,
			LastCheck:    lastCheck,
			ResponseTime: lastResponseTime,
			SuccessRate:  successRate,
		}
	}

	return statuses, nil
}

// CheckSingleAPIKey checks health of a specific API key.
func (s *Service) CheckSingleAPIKey(ctx context.Context, id uuid.UUID) (*APIKeyHealthStatus, error) {
	key, err := s.providerKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	client, ok := s.providerRegistry.Get(key.Provider.Name)
	if !ok {
		return &APIKeyHealthStatus{
			ID:        key.ID,
			KeyPrefix: key.KeyPrefix,
			IsActive:  key.IsActive,
			IsHealthy: false,
		}, nil
	}

	healthy, latency, _ := client.CheckHealth(ctx)

	history := &models.HealthHistory{
		TargetType:   "api_key",
		TargetID:     key.ID,
		IsHealthy:    healthy,
		ResponseTime: latency.Milliseconds(),
		CheckedAt:    time.Now(),
	}
	_ = s.healthHistoryRepo.Create(ctx, history)

	if !healthy && s.alertNotifier != nil {
		_ = s.alertNotifier.Notify(ctx, "api_key", key.ID, "health_check_failed", "API key health check failed")
	}

	// Calculate success rate from history
	histories, _ := s.healthHistoryRepo.GetByTarget(ctx, "api_key", key.ID, 10)
	successCount := 0
	for _, h := range histories {
		if h.IsHealthy {
			successCount++
		}
	}
	successRate := float64(0)
	if len(histories) > 0 {
		successRate = float64(successCount) / float64(len(histories))
	}

	return &APIKeyHealthStatus{
		ID:           key.ID,
		ProviderName: key.Provider.Name,
		KeyPrefix:    key.KeyPrefix,
		IsActive:     key.IsActive,
		IsHealthy:    healthy,
		LastCheck:    time.Now(),
		ResponseTime: latency.Milliseconds(),
		SuccessRate:  successRate,
	}, nil
}

// GetProxiesHealth returns health status of all proxies.
func (s *Service) GetProxiesHealth(ctx context.Context) ([]ProxyHealthStatus, error) {
	proxies, err := s.proxyRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]ProxyHealthStatus, len(proxies))
	for i, p := range proxies {
		history, _ := s.healthHistoryRepo.GetByTarget(ctx, "proxy", p.ID, 10)

		successCount := 0
		var lastCheck time.Time
		var lastResponseTime int64
		isHealthy := true

		for j, h := range history {
			if j == 0 {
				lastCheck = h.CheckedAt
				lastResponseTime = h.ResponseTime
				isHealthy = h.IsHealthy
			}
			if h.IsHealthy {
				successCount++
			}
		}

		successRate := float64(0)
		if len(history) > 0 {
			successRate = float64(successCount) / float64(len(history))
		}

		statuses[i] = ProxyHealthStatus{
			ID:           p.ID,
			URL:          p.URL,
			Type:         p.Type,
			Region:       p.Region,
			IsActive:     p.IsActive,
			IsHealthy:    isHealthy,
			ResponseTime: lastResponseTime,
			LastCheck:    lastCheck,
			SuccessRate:  successRate,
		}
	}

	return statuses, nil
}

// CheckSingleProxy checks health of a specific proxy.
func (s *Service) CheckSingleProxy(ctx context.Context, id uuid.UUID) (*ProxyHealthStatus, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	healthy, latency, _ := s.proxyService.CheckHealth(ctx, id)

	history := &models.HealthHistory{
		TargetType:   "proxy",
		TargetID:     proxy.ID,
		IsHealthy:    healthy,
		ResponseTime: latency.Milliseconds(),
		CheckedAt:    time.Now(),
	}
	_ = s.healthHistoryRepo.Create(ctx, history)

	if !healthy && s.alertNotifier != nil {
		_ = s.alertNotifier.Notify(ctx, "proxy", proxy.ID, "health_check_failed", "Proxy health check failed")
	}

	// Calculate success rate from history
	histories, _ := s.healthHistoryRepo.GetByTarget(ctx, "proxy", proxy.ID, 10)
	successCount := 0
	for _, h := range histories {
		if h.IsHealthy {
			successCount++
		}
	}
	successRate := float64(0)
	if len(histories) > 0 {
		successRate = float64(successCount) / float64(len(histories))
	}

	return &ProxyHealthStatus{
		ID:           proxy.ID,
		URL:          proxy.URL,
		Type:         proxy.Type,
		Region:       proxy.Region,
		IsActive:     proxy.IsActive,
		IsHealthy:    healthy,
		ResponseTime: latency.Milliseconds(),
		LastCheck:    time.Now(),
		SuccessRate:  successRate,
	}, nil
}

// GetHealthHistory returns recent health check history.
func (s *Service) GetHealthHistory(ctx context.Context, targetType string, limit int) ([]models.HealthHistory, error) {
	return s.healthHistoryRepo.GetRecent(ctx, targetType, limit)
}

// GetAlerts returns alerts with pagination.
func (s *Service) GetAlerts(ctx context.Context, status string, page, pageSize int) ([]models.Alert, int64, error) {
	if s.alertNotifier == nil {
		return nil, 0, nil
	}
	return s.alertNotifier.GetAlerts(ctx, status, page, pageSize)
}

// AcknowledgeAlert marks an alert as acknowledged.
func (s *Service) AcknowledgeAlert(ctx context.Context, alertID uuid.UUID) error {
	if s.alertNotifier == nil {
		return nil
	}
	return s.alertNotifier.AcknowledgeAlert(ctx, alertID)
}

// ResolveAlert marks an alert as resolved.
func (s *Service) ResolveAlert(ctx context.Context, alertID uuid.UUID) error {
	if s.alertNotifier == nil {
		return nil
	}
	return s.alertNotifier.ResolveAlert(ctx, alertID)
}

// UpdateAlertConfig updates alert configuration.
func (s *Service) UpdateAlertConfig(ctx context.Context, config *models.AlertConfig) error {
	if s.alertNotifier == nil {
		return nil
	}
	return s.alertNotifier.UpdateAlertConfig(ctx, config)
}

// GetAlertConfig returns alert configuration for a target.
func (s *Service) GetAlertConfig(ctx context.Context, targetType string, targetID uuid.UUID) (*models.AlertConfig, error) {
	if s.alertNotifier == nil {
		return nil, nil
	}
	return s.alertNotifier.GetAlertConfigByTarget(ctx, targetType, targetID)
}

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
func (s *Service) CheckSingleProvider(ctx context.Context, id uuid.UUID) (*ProviderHealthStatus, error) {
	p, err := s.providerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var healthy bool
	var latency time.Duration
	var errorMsg string

	client, ok := s.providerRegistry.Get(p.Name)
	if !ok {
		healthy = false
		errorMsg = "provider not registered"
	} else {
		// Check health using proxy if enabled
		if p.UseProxy {
			healthy, latency, errorMsg = s.checkWithProxy(ctx, p)
		} else {
			var err error
			healthy, latency, err = client.CheckHealth(ctx)
			if err != nil {
				errorMsg = err.Error()
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

	// Calculate success rate from history
	histories, _ := s.healthHistoryRepo.GetByTarget(ctx, "provider", p.ID, 10)
	successCount := 0
	for _, h := range histories {
		if h.IsHealthy {
			successCount++
		}
	}
	successRate := float64(0)
	if len(histories) > 0 {
		successRate = float64(successCount) / float64(len(histories))
	}

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
func (s *Service) checkWithProxy(ctx context.Context, p *models.Provider) (bool, time.Duration, string) {
	// Get an active proxy
	proxies, err := s.proxyRepo.GetActive(ctx)
	if err != nil || len(proxies) == 0 {
		return false, 0, "no active proxy available"
	}

	// Use the first active proxy
	proxyInfo := proxies[0]
	proxyURL, err := url.Parse(proxyInfo.URL)
	if err != nil {
		return false, 0, "invalid proxy URL"
	}

	// Create HTTP client with proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(p.Timeout) * time.Second,
	}

	// Determine health check endpoint based on provider
	var healthURL string
	switch p.Name {
	case "openai", "lmstudio":
		healthURL = p.BaseURL + "/models"
	case "ollama":
		healthURL = p.BaseURL + "/api/tags"
	case "anthropic":
		// Anthropic doesn't have a public health endpoint, use messages with minimal request
		healthURL = p.BaseURL + "/v1/messages"
	default:
		healthURL = p.BaseURL + "/models"
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false, 0, err.Error()
	}

	resp, err := httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return false, latency, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	// For Anthropic, a 401 means the endpoint is reachable
	if p.Name == "anthropic" && resp.StatusCode == http.StatusUnauthorized {
		return true, latency, ""
	}

	return resp.StatusCode == http.StatusOK, latency, ""
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
