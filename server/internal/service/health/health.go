// Package health provides health check functionality.
package health

import (
	"context"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles health checks for API keys, proxies, and providers.
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

// ─── Status Types ───────────────────────────────────────────────────────

// APIKeyHealthStatus represents health status of an API key.
type APIKeyHealthStatus struct {
	ID           uuid.UUID `json:"id"`
	ProviderID   uuid.UUID `json:"provider_id"`
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

// ─── Provider Client Helpers ────────────────────────────────────────────

// getProviderClient creates a provider client dynamically using a ProviderAPIKey.
func (s *Service) getProviderClient(p *models.Provider, apiKey *models.ProviderAPIKey) (provider.Client, error) {
	// First try registry for local providers (Ollama, LM Studio)
	if client, ok := s.providerRegistry.Get(p.Name); ok {
		return client, nil
	}

	// Create client dynamically with the provider API key
	var decryptedKey string
	if apiKey != nil && apiKey.EncryptedAPIKey != "" {
		var err error
		decryptedKey, err = crypto.Decrypt(apiKey.EncryptedAPIKey)
		if err != nil {
			return nil, err
		}
	}

	cfg := &config.ProviderConfig{
		APIKey:  decryptedKey,
		BaseURL: p.BaseURL,
	}

	return s.createProviderClient(p.Name, cfg)
}

// createProviderClient creates a provider client based on provider name.
// Delegates to the shared factory in the provider package.
func (s *Service) createProviderClient(name string, cfg *config.ProviderConfig) (provider.Client, error) {
	return provider.NewClientByName(name, cfg, s.logger)
}

// ─── Alert Management ───────────────────────────────────────────────────

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
