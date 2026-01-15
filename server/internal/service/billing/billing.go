// Package billing provides billing and usage tracking.
package billing

import (
	"context"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles billing and usage tracking.
type Service struct {
	usageRepo *repository.UsageLogRepository
	modelRepo *repository.ModelRepository
	logger    *zap.Logger
}

// NewService creates a new billing service.
func NewService(
	usageRepo *repository.UsageLogRepository,
	modelRepo *repository.ModelRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		usageRepo: usageRepo,
		modelRepo: modelRepo,
		logger:    logger,
	}
}

// RecordUsage records API usage.
func (s *Service) RecordUsage(ctx context.Context, log *models.UsageLog) error {
	if log.ModelID != uuid.Nil {
		model, err := s.modelRepo.GetByID(ctx, log.ModelID)
		if err == nil {
			log.Cost = s.calculateCost(model, log.RequestTokens, log.ResponseTokens)
		}
	}

	return s.usageRepo.Create(ctx, log)
}

// calculateCost calculates the cost for token usage.
func (s *Service) calculateCost(model *models.Model, inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) / 1000 * model.InputPricePer1K
	outputCost := float64(outputTokens) / 1000 * model.OutputPricePer1K
	return inputCost + outputCost
}

// UsageSummary represents aggregated usage data.
type UsageSummary struct {
	TotalRequests int64   `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	AvgLatency    float64 `json:"avg_latency"`
	SuccessRate   float64 `json:"success_rate"`
	ErrorCount    int64   `json:"error_count"`
}

// GetUsageSummary returns aggregated usage for a user.
func (s *Service) GetUsageSummary(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time) (*UsageSummary, error) {
	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateLogs(logs), nil
}

// GetSystemUsageSummary returns aggregated usage for all users (system-wide).
func (s *Service) GetSystemUsageSummary(ctx context.Context, startTime, endTime time.Time) (*UsageSummary, error) {
	logs, err := s.usageRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateLogs(logs), nil
}

// aggregateLogs aggregates usage logs into a summary.
func (s *Service) aggregateLogs(logs []models.UsageLog) *UsageSummary {
	summary := &UsageSummary{}
	var totalLatency int64
	var successCount int64

	for _, log := range logs {
		summary.TotalRequests++
		summary.TotalTokens += int64(log.TotalTokens)
		summary.TotalCost += log.Cost
		totalLatency += log.Latency

		if log.StatusCode >= 200 && log.StatusCode < 300 {
			successCount++
		} else {
			summary.ErrorCount++
		}
	}

	if summary.TotalRequests > 0 {
		summary.AvgLatency = float64(totalLatency) / float64(summary.TotalRequests)
		summary.SuccessRate = float64(successCount) / float64(summary.TotalRequests) * 100
	}

	return summary
}

// DailyUsage represents daily usage data.
type DailyUsage struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// GetDailyUsage returns daily usage statistics.
func (s *Service) GetDailyUsage(ctx context.Context, userID uuid.UUID, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateDailyLogs(logs), nil
}

// GetSystemDailyUsage returns daily usage statistics for all users (system-wide).
func (s *Service) GetSystemDailyUsage(ctx context.Context, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	logs, err := s.usageRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateDailyLogs(logs), nil
}

// aggregateDailyLogs aggregates logs by day.
func (s *Service) aggregateDailyLogs(logs []models.UsageLog) []DailyUsage {
	dailyMap := make(map[string]*DailyUsage)

	for _, log := range logs {
		date := log.CreatedAt.Format("2006-01-02")
		if _, ok := dailyMap[date]; !ok {
			dailyMap[date] = &DailyUsage{Date: date}
		}
		dailyMap[date].Requests++
		dailyMap[date].Tokens += int64(log.TotalTokens)
		dailyMap[date].Cost += log.Cost
	}

	result := make([]DailyUsage, 0, len(dailyMap))
	for _, usage := range dailyMap {
		result = append(result, *usage)
	}

	return result
}

// ProviderUsage represents usage per provider.
type ProviderUsage struct {
	ProviderID   uuid.UUID `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	Requests     int64     `json:"requests"`
	Tokens       int64     `json:"tokens"`
	Cost         float64   `json:"cost"`
}

// GetUsageByProvider returns usage grouped by provider.
func (s *Service) GetUsageByProvider(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time) ([]ProviderUsage, error) {
	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateProviderLogs(logs), nil
}

// GetSystemUsageByProvider returns usage grouped by provider for all users (system-wide).
func (s *Service) GetSystemUsageByProvider(ctx context.Context, startTime, endTime time.Time) ([]ProviderUsage, error) {
	logs, err := s.usageRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateProviderLogs(logs), nil
}

// aggregateProviderLogs aggregates logs by provider.
func (s *Service) aggregateProviderLogs(logs []models.UsageLog) []ProviderUsage {
	providerMap := make(map[uuid.UUID]*ProviderUsage)

	for _, log := range logs {
		if _, ok := providerMap[log.ProviderID]; !ok {
			providerMap[log.ProviderID] = &ProviderUsage{ProviderID: log.ProviderID}
		}
		providerMap[log.ProviderID].Requests++
		providerMap[log.ProviderID].Tokens += int64(log.TotalTokens)
		providerMap[log.ProviderID].Cost += log.Cost
	}

	result := make([]ProviderUsage, 0, len(providerMap))
	for _, usage := range providerMap {
		result = append(result, *usage)
	}

	return result
}

// ModelUsage represents usage per model.
type ModelUsage struct {
	ModelID      uuid.UUID `json:"model_id"`
	ModelName    string    `json:"model_name"`
	Requests     int64     `json:"requests"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	TotalTokens  int64     `json:"total_tokens"`
	Cost         float64   `json:"cost"`
}

// GetUsageByModel returns usage grouped by model name.
func (s *Service) GetUsageByModel(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time) ([]ModelUsage, error) {
	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateModelLogs(logs), nil
}

// GetSystemUsageByModel returns usage grouped by model for all users (system-wide).
func (s *Service) GetSystemUsageByModel(ctx context.Context, startTime, endTime time.Time) ([]ModelUsage, error) {
	logs, err := s.usageRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return s.aggregateModelLogs(logs), nil
}

// aggregateModelLogs aggregates logs by model name.
func (s *Service) aggregateModelLogs(logs []models.UsageLog) []ModelUsage {
	// Group by model name (works for both registered models and dynamic ones like Ollama)
	modelMap := make(map[string]*ModelUsage)

	for _, log := range logs {
		modelName := log.ModelName
		if modelName == "" {
			continue
		}
		if _, ok := modelMap[modelName]; !ok {
			modelMap[modelName] = &ModelUsage{
				ModelID:   log.ModelID,
				ModelName: modelName,
			}
		}
		modelMap[modelName].Requests++
		modelMap[modelName].InputTokens += int64(log.RequestTokens)
		modelMap[modelName].OutputTokens += int64(log.ResponseTokens)
		modelMap[modelName].TotalTokens += int64(log.TotalTokens)
		modelMap[modelName].Cost += log.Cost
	}

	result := make([]ModelUsage, 0, len(modelMap))
	for _, usage := range modelMap {
		result = append(result, *usage)
	}

	return result
}

// GetRecentUsage returns recent usage logs.
func (s *Service) GetRecentUsage(ctx context.Context, userID uuid.UUID, limit int) ([]models.UsageLog, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -30)

	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	if len(logs) > limit {
		logs = logs[:limit]
	}

	return logs, nil
}
