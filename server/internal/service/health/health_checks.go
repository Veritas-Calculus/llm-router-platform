package health

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

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
			ProviderID:   key.ProviderID,
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

// CheckSingleAPIKey checks health of a specific provider API key.
// This uses the ProviderAPIKey to create a client and test the actual provider.
func (s *Service) CheckSingleAPIKey(ctx context.Context, id uuid.UUID) (*APIKeyHealthStatus, error) {
	key, err := s.providerKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get the provider for this API key
	p, err := s.providerRepo.GetByID(ctx, key.ProviderID)
	if err != nil {
		return &APIKeyHealthStatus{
			ID:        key.ID,
			KeyPrefix: key.KeyPrefix,
			IsActive:  key.IsActive,
			IsHealthy: false,
		}, nil
	}

	// Create client dynamically using the provider API key
	client, err := s.getProviderClient(p, key)
	if err != nil {
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
	if err := s.healthHistoryRepo.Create(ctx, history); err != nil {
		s.logger.Error("failed to record health check history",
			zap.String("target_type", "api_key"),
			zap.String("target_id", key.ID.String()),
			zap.Error(err))
	}

	if !healthy && s.alertNotifier != nil {
		if err := s.alertNotifier.Notify(ctx, "api_key", key.ID, "health_check_failed", "API key health check failed"); err != nil {
			s.logger.Error("failed to send health check alert",
				zap.String("target_type", "api_key"),
				zap.String("target_id", key.ID.String()),
				zap.Error(err))
		}
	}

	successRate := s.calculateSuccessRate(ctx, "api_key", key.ID)

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
	if err := s.healthHistoryRepo.Create(ctx, history); err != nil {
		s.logger.Error("failed to record health check history",
			zap.String("target_type", "proxy"),
			zap.String("target_id", proxy.ID.String()),
			zap.Error(err))
	}

	if !healthy && s.alertNotifier != nil {
		if err := s.alertNotifier.Notify(ctx, "proxy", proxy.ID, "health_check_failed", "Proxy health check failed"); err != nil {
			s.logger.Error("failed to send health check alert",
				zap.String("target_type", "proxy"),
				zap.String("target_id", proxy.ID.String()),
				zap.Error(err))
		}
	}

	successRate := s.calculateSuccessRate(ctx, "proxy", proxy.ID)

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

// calculateSuccessRate computes the rolling success rate from the last 10 health records.
func (s *Service) calculateSuccessRate(ctx context.Context, targetType string, targetID uuid.UUID) float64 {
	histories, _ := s.healthHistoryRepo.GetByTarget(ctx, targetType, targetID, 10)
	successCount := 0
	for _, h := range histories {
		if h.IsHealthy {
			successCount++
		}
	}
	if len(histories) == 0 {
		return 0
	}
	return float64(successCount) / float64(len(histories))
}
