// Package billing provides billing and usage tracking.
package billing

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles billing and usage tracking.
type Service struct {
	usageRepo *repository.UsageLogRepository
	modelRepo *repository.ModelRepository
	redis     *redis.Client
	logger    *zap.Logger
}

// NewService creates a new billing service.
func NewService(
	usageRepo *repository.UsageLogRepository,
	modelRepo *repository.ModelRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
) *Service {
	return &Service{
		usageRepo: usageRepo,
		modelRepo: modelRepo,
		redis:     redisClient,
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

	err := s.usageRepo.Create(ctx, log)

	// Refresh redis cache asynchronously
	if s.redis != nil && err == nil {
		now := time.Now()
		monthStr := fmt.Sprintf("%d-%02d", now.Year(), now.Month())
		key := fmt.Sprintf("billing:usage:%s:%s", log.UserID.String(), monthStr)

		pipe := s.redis.Pipeline()
		pipe.HIncrBy(ctx, key, "total_requests", 1)
		pipe.HIncrBy(ctx, key, "total_tokens", int64(log.TotalTokens))
		pipe.HIncrByFloat(ctx, key, "total_cost", log.Cost)
		pipe.Expire(ctx, key, 32*24*time.Hour)
		_, _ = pipe.Exec(ctx)
	}

	return err
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
	now := time.Now()
	isCurrentMonth := startTime.Year() == now.Year() && startTime.Month() == now.Month()

	// Try Redis cache for current month
	if s.redis != nil && isCurrentMonth {
		monthStr := fmt.Sprintf("%d-%02d", now.Year(), now.Month())
		key := fmt.Sprintf("billing:usage:%s:%s", userID.String(), monthStr)

		res, err := s.redis.HGetAll(ctx, key).Result()
		if err == nil && len(res) > 0 {
			reqs, _ := strconv.ParseInt(res["total_requests"], 10, 64)
			tokens, _ := strconv.ParseInt(res["total_tokens"], 10, 64)
			cost, _ := strconv.ParseFloat(res["total_cost"], 64)

			return &UsageSummary{
				TotalRequests: reqs,
				TotalTokens:   tokens,
				TotalCost:     cost,
			}, nil
		}
	}

	// SQL aggregation — no full-row load
	row, err := s.usageRepo.AggregateByTimeRange(ctx, &userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		TotalRequests: row.TotalRequests,
		TotalTokens:   row.TotalTokens,
		TotalCost:     row.TotalCost,
		AvgLatency:    row.AvgLatency,
		ErrorCount:    row.ErrorCount,
	}
	if row.TotalRequests > 0 {
		summary.SuccessRate = float64(row.SuccessCount) / float64(row.TotalRequests) * 100
	}

	// Backfill Redis cache
	if s.redis != nil && isCurrentMonth {
		monthStr := fmt.Sprintf("%d-%02d", now.Year(), now.Month())
		key := fmt.Sprintf("billing:usage:%s:%s", userID.String(), monthStr)

		pipe := s.redis.Pipeline()
		pipe.HSet(ctx, key, "total_requests", summary.TotalRequests)
		pipe.HSet(ctx, key, "total_tokens", summary.TotalTokens)
		pipe.HSet(ctx, key, "total_cost", summary.TotalCost)
		pipe.Expire(ctx, key, 32*24*time.Hour)
		_, _ = pipe.Exec(ctx)
	}

	return summary, nil
}

// GetSystemUsageSummary returns aggregated usage for all users (system-wide).
func (s *Service) GetSystemUsageSummary(ctx context.Context, startTime, endTime time.Time) (*UsageSummary, error) {
	row, err := s.usageRepo.AggregateByTimeRange(ctx, nil, startTime, endTime)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		TotalRequests: row.TotalRequests,
		TotalTokens:   row.TotalTokens,
		TotalCost:     row.TotalCost,
		AvgLatency:    row.AvgLatency,
		ErrorCount:    row.ErrorCount,
	}
	if row.TotalRequests > 0 {
		summary.SuccessRate = float64(row.SuccessCount) / float64(row.TotalRequests) * 100
	}

	return summary, nil
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

// GetDailyUsage returns daily usage statistics (SQL aggregation).
func (s *Service) GetDailyUsage(ctx context.Context, userID uuid.UUID, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	rows, err := s.usageRepo.AggregateDailyByTimeRange(ctx, &userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]DailyUsage, len(rows))
	for i, r := range rows {
		result[i] = DailyUsage{Date: r.Date, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost}
	}
	return result, nil
}

// GetSystemDailyUsage returns daily usage statistics for all users (SQL aggregation).
func (s *Service) GetSystemDailyUsage(ctx context.Context, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	rows, err := s.usageRepo.AggregateDailyByTimeRange(ctx, nil, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]DailyUsage, len(rows))
	for i, r := range rows {
		result[i] = DailyUsage{Date: r.Date, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost}
	}
	return result, nil
}

// ProviderUsage represents usage per provider.
type ProviderUsage struct {
	ProviderID   uuid.UUID `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	Requests     int64     `json:"requests"`
	Tokens       int64     `json:"tokens"`
	Cost         float64   `json:"cost"`
}

// GetUsageByProvider returns usage grouped by provider (SQL aggregation).
func (s *Service) GetUsageByProvider(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time) ([]ProviderUsage, error) {
	rows, err := s.usageRepo.AggregateByProviderByTimeRange(ctx, &userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]ProviderUsage, len(rows))
	for i, r := range rows {
		result[i] = ProviderUsage{ProviderID: r.ProviderID, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost}
	}
	return result, nil
}

// GetSystemUsageByProvider returns usage grouped by provider for all users (SQL aggregation).
func (s *Service) GetSystemUsageByProvider(ctx context.Context, startTime, endTime time.Time) ([]ProviderUsage, error) {
	rows, err := s.usageRepo.AggregateByProviderByTimeRange(ctx, nil, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]ProviderUsage, len(rows))
	for i, r := range rows {
		result[i] = ProviderUsage{ProviderID: r.ProviderID, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost}
	}
	return result, nil
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

// GetUsageByModel returns usage grouped by model name (SQL aggregation).
func (s *Service) GetUsageByModel(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time) ([]ModelUsage, error) {
	rows, err := s.usageRepo.AggregateByModelByTimeRange(ctx, &userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]ModelUsage, len(rows))
	for i, r := range rows {
		result[i] = ModelUsage{
			ModelID:      r.ModelID,
			ModelName:    r.ModelName,
			Requests:     r.Requests,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			TotalTokens:  r.TotalTokens,
			Cost:         r.Cost,
		}
	}
	return result, nil
}

// GetSystemUsageByModel returns usage grouped by model for all users (SQL aggregation).
func (s *Service) GetSystemUsageByModel(ctx context.Context, startTime, endTime time.Time) ([]ModelUsage, error) {
	rows, err := s.usageRepo.AggregateByModelByTimeRange(ctx, nil, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]ModelUsage, len(rows))
	for i, r := range rows {
		result[i] = ModelUsage{
			ModelID:      r.ModelID,
			ModelName:    r.ModelName,
			Requests:     r.Requests,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			TotalTokens:  r.TotalTokens,
			Cost:         r.Cost,
		}
	}
	return result, nil
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

	// Set IsSuccess based on StatusCode
	for i := range logs {
		logs[i].IsSuccess = logs[i].StatusCode >= 200 && logs[i].StatusCode < 300
	}

	return logs, nil
}
