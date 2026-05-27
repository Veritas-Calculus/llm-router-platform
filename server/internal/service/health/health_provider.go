package health

import (
	"context"
	"time"

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
		client, err := s.getProviderClient(ctx, p, apiKey)
		if err != nil {
			healthy = false
			errorMsg = "failed to create provider client: " + err.Error()
			s.logger.Error("failed to create provider client", zap.Error(err))
		} else {
			healthy, latency, err = client.CheckHealth(ctx)
			if err != nil {
				errorMsg = err.Error()
				s.logger.Error("health check failed", zap.String("provider", p.Name), zap.Error(err))
			} else {
				s.logger.Info("health check completed", zap.String("provider", p.Name), zap.Bool("healthy", healthy), zap.Duration("latency", latency))
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
