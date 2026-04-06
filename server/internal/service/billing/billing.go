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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ─── Prometheus Billing Metrics ─────────────────────────────────────────
var (
	billingRecordErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "billing_record_errors_total",
			Help:      "Total number of billing usage recording failures.",
		},
		[]string{"operation"}, // "record_usage", "record_usage_and_deduct"
	)
	billingDeductErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "billing_deduct_errors_total",
			Help:      "Total number of balance deduction failures.",
		},
	)
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

// UpdateUsageTokens updates an existing usage log with final token counts and status.
// Used for streaming requests to ensure usage is recorded even if the stream is interrupted.
func (s *Service) UpdateUsageTokens(ctx context.Context, logID uuid.UUID, requestTokens, responseTokens int, statusCode int, latencyMs int64, errorMessage string) error {
	log, err := s.usageRepo.GetByID(ctx, logID)
	if err != nil {
		return err
	}

	log.RequestTokens = requestTokens
	log.ResponseTokens = responseTokens
	log.TotalTokens = requestTokens + responseTokens
	log.StatusCode = statusCode
	log.ErrorMessage = errorMessage
	log.IsSuccess = statusCode >= 200 && statusCode < 300
	log.Latency = latencyMs

	if log.ModelID != uuid.Nil {
		model, err := s.modelRepo.GetByID(ctx, log.ModelID)
		if err == nil {
			log.Cost = s.calculateCost(model, log.RequestTokens, log.ResponseTokens)
		}
	}

	err = s.usageRepo.Update(ctx, log)

	// Refresh redis cache — use org-scoped key matching GetUsageSummary read path
	if s.redis != nil && err == nil && log.IsSuccess {
		s.incrUsageCache(ctx, log)
	}

	return err
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

	// Refresh redis cache — use org-scoped key matching GetUsageSummary read path
	if s.redis != nil && err == nil {
		s.incrUsageCache(ctx, log)
	}
	if err != nil {
		billingRecordErrorsTotal.WithLabelValues("record_usage").Inc()
	}

	return err
}

// RecordUsageAndDeduct atomically records API usage and deducts the cost from
// the user's balance in a single database transaction. This prevents the race
// condition where usage is recorded but balance deduction fails (or vice versa)
// due to a process crash between the two operations.
//
// If balanceSvc is nil or cost is zero, it behaves identically to RecordUsage.
func (s *Service) RecordUsageAndDeduct(ctx context.Context, log *models.UsageLog, balanceSvc *BalanceService, userID uuid.UUID, description string) error {
	// Calculate cost first (outside transaction — read-only)
	if log.ModelID != uuid.Nil {
		model, err := s.modelRepo.GetByID(ctx, log.ModelID)
		if err == nil {
			log.Cost = s.calculateCost(model, log.RequestTokens, log.ResponseTokens)
		}
	}

	// If no balance service or zero cost, fall back to simple insert
	if balanceSvc == nil || log.Cost <= 0 {
		err := s.usageRepo.Create(ctx, log)
		if s.redis != nil && err == nil {
			s.incrUsageCache(ctx, log)
		}
		return err
	}

	// Atomic transaction: insert usage log + deduct balance
	err := balanceSvc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Insert usage log within the transaction
		if err := tx.Create(log).Error; err != nil {
			return err
		}

		// 2. Lock user row and deduct balance
		var user models.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance -= log.Cost
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		// 3. Record the transaction
		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      -log.Cost,
			Balance:     user.Balance,
			Description: description,
			ReferenceID: log.ID.String(),
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		// 4. Low balance alert (async, non-transactional)
		balanceSvc.sendLowBalanceAlert(ctx, userID, user.Email, user.Name, user.Balance)

		return nil
	})

	// Refresh redis cache outside transaction
	if s.redis != nil && err == nil {
		s.incrUsageCache(ctx, log)
	}
	if err != nil {
		billingRecordErrorsTotal.WithLabelValues("record_usage_and_deduct").Inc()
	}

	return err
}

// sendLowBalanceAlert sends a low-balance warning email if the user's balance drops below $1,
// with a 24-hour cooldown per user to avoid spam.
func (bs *BalanceService) sendLowBalanceAlert(ctx context.Context, userID uuid.UUID, email, name string, balance float64) {
	if bs.redis == nil || bs.emailSvc == nil || balance >= 1.0 {
		return
	}

	cacheKey := fmt.Sprintf("quota_warn:balance:%s", userID.String())
	if err := bs.redis.Get(ctx, cacheKey).Err(); err != redis.Nil {
		return // Already sent recently or Redis error
	}

	bs.logger.Info("sending low balance warning email", zap.String("userID", userID.String()), zap.Float64("balance", balance))
	go func(to, uname string, currentBalance float64) {
		if err := bs.emailSvc.SendQuotaWarningEmail(to, uname, fmt.Sprintf("$%.2f", currentBalance), "$1.00"); err != nil {
			bs.logger.Error("failed to send quota warning email", zap.Error(err))
		}
	}(email, name, balance)
	bs.redis.Set(ctx, cacheKey, "1", 24*time.Hour)
}

// incrUsageCache increments the Redis usage cache using org-scoped keys
// that match the format read by GetUsageSummary.
func (s *Service) incrUsageCache(ctx context.Context, log *models.UsageLog) {
	// Look up the org ID from the project
	orgID, err := s.usageRepo.GetOrgIDByProjectID(ctx, log.ProjectID)
	if err != nil || orgID == uuid.Nil {
		return
	}

	now := time.Now()
	monthStr := fmt.Sprintf("%d-%02d", now.Year(), now.Month())
	key := fmt.Sprintf("billing:usage:org:%s:%s", orgID.String(), monthStr)

	pipe := s.redis.Pipeline()
	pipe.HIncrBy(ctx, key, "total_requests", 1)
	pipe.HIncrBy(ctx, key, "total_tokens", int64(log.TotalTokens))
	pipe.HIncrByFloat(ctx, key, "total_cost", log.Cost)
	pipe.Expire(ctx, key, 32*24*time.Hour)
	_, _ = pipe.Exec(ctx)
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
	MCPCallCount  int64   `json:"mcp_call_count"`
	MCPErrorCount int64   `json:"mcp_error_count"`
}

// GetUsageSummary returns aggregated usage for an organization or project.
func (s *Service) GetUsageSummary(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, channel *string, startTime, endTime time.Time) (*UsageSummary, error) {
	now := time.Now()
	isCurrentMonth := startTime.Year() == now.Year() && startTime.Month() == now.Month()

	// Redis cache is only populated at the org level (no project/channel dims),
	// so only attempt a cache hit when no sub-filters are applied.
	useCache := s.redis != nil && isCurrentMonth && projectID == nil && (channel == nil || *channel == "")

	if useCache {
		monthStr := fmt.Sprintf("%d-%02d", now.Year(), now.Month())
		key := fmt.Sprintf("billing:usage:org:%s:%s", orgID.String(), monthStr)

		res, err := s.redis.HGetAll(ctx, key).Result()
		if err == nil && len(res) > 0 {
			reqs, _ := strconv.ParseInt(res["total_requests"], 10, 64)
			tokens, _ := strconv.ParseInt(res["total_tokens"], 10, 64)
			cost, _ := strconv.ParseFloat(res["total_cost"], 64)
			successRate, _ := strconv.ParseFloat(res["success_rate"], 64)

			return &UsageSummary{
				TotalRequests: reqs,
				TotalTokens:   tokens,
				TotalCost:     cost,
				SuccessRate:   successRate,
			}, nil
		}
	}

	row, err := s.usageRepo.AggregateByTimeRange(ctx, &orgID, projectID, channel, startTime, endTime)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		TotalRequests: row.TotalRequests,
		TotalTokens:   row.TotalTokens,
		TotalCost:     row.TotalCost,
		AvgLatency:    row.AvgLatency,
		ErrorCount:    row.ErrorCount,
		MCPCallCount:  row.MCPCallCount,
		MCPErrorCount: row.MCPErrorCount,
	}
	if row.TotalRequests > 0 {
		summary.SuccessRate = float64(row.SuccessCount) / float64(row.TotalRequests) * 100
	}

	if useCache && summary.TotalRequests > 0 {
		monthStr := fmt.Sprintf("%d-%02d", now.Year(), now.Month())
		key := fmt.Sprintf("billing:usage:org:%s:%s", orgID.String(), monthStr)
		pipe := s.redis.Pipeline()
		pipe.HSet(ctx, key, "total_requests", summary.TotalRequests)
		pipe.HSet(ctx, key, "total_tokens", summary.TotalTokens)
		pipe.HSet(ctx, key, "total_cost", summary.TotalCost)
		pipe.HSet(ctx, key, "success_rate", summary.SuccessRate)
		pipe.Expire(ctx, key, 30*time.Second)
		_, _ = pipe.Exec(ctx)
	}

	return summary, nil
}

// GetSystemUsageSummary returns aggregated usage for all users (system-wide).
func (s *Service) GetSystemUsageSummary(ctx context.Context, channel *string, startTime, endTime time.Time) (*UsageSummary, error) {
	row, err := s.usageRepo.AggregateByTimeRange(ctx, nil, nil, channel, startTime, endTime)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		TotalRequests: row.TotalRequests,
		TotalTokens:   row.TotalTokens,
		TotalCost:     row.TotalCost,
		AvgLatency:    row.AvgLatency,
		ErrorCount:    row.ErrorCount,
		MCPCallCount:  row.MCPCallCount,
		MCPErrorCount: row.MCPErrorCount,
	}
	if row.TotalRequests > 0 {
		summary.SuccessRate = float64(row.SuccessCount) / float64(row.TotalRequests) * 100
	}

	return summary, nil
}

// DailyUsage represents daily usage data.
type DailyUsage struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// GetDailyUsage returns daily usage statistics (SQL aggregation).
func (s *Service) GetDailyUsage(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, channel *string, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	rows, err := s.usageRepo.AggregateDailyByTimeRange(ctx, &orgID, projectID, channel, startTime, endTime)
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
func (s *Service) GetSystemDailyUsage(ctx context.Context, channel *string, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	rows, err := s.usageRepo.AggregateDailyByTimeRange(ctx, nil, nil, channel, startTime, endTime)
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
	SuccessRate  float64   `json:"success_rate"`
	AvgLatency   float64   `json:"avg_latency_ms"`
}

func mapProviderRows(rows []repository.ProviderUsageRow) []ProviderUsage {
	result := make([]ProviderUsage, len(rows))
	for i, r := range rows {
		result[i] = ProviderUsage{
			ProviderID: r.ProviderID, ProviderName: r.ProviderName,
			Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost,
			SuccessRate: r.SuccessRate, AvgLatency: r.AvgLatency,
		}
	}
	return result
}

// GetUsageByProvider returns usage grouped by provider (SQL aggregation).
func (s *Service) GetUsageByProvider(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, channel *string, startTime, endTime time.Time) ([]ProviderUsage, error) {
	rows, err := s.usageRepo.AggregateByProviderByTimeRange(ctx, &orgID, projectID, channel, startTime, endTime)
	if err != nil {
		return nil, err
	}
	return mapProviderRows(rows), nil
}

// GetSystemUsageByProvider returns usage grouped by provider for all users (SQL aggregation).
func (s *Service) GetSystemUsageByProvider(ctx context.Context, channel *string, startTime, endTime time.Time) ([]ProviderUsage, error) {
	rows, err := s.usageRepo.AggregateByProviderByTimeRange(ctx, nil, nil, channel, startTime, endTime)
	if err != nil {
		return nil, err
	}
	return mapProviderRows(rows), nil
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
func (s *Service) GetUsageByModel(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, channel *string, startTime, endTime time.Time) ([]ModelUsage, error) {
	rows, err := s.usageRepo.AggregateByModelByTimeRange(ctx, &orgID, projectID, channel, startTime, endTime)
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
func (s *Service) GetSystemUsageByModel(ctx context.Context, channel *string, startTime, endTime time.Time) ([]ModelUsage, error) {
	rows, err := s.usageRepo.AggregateByModelByTimeRange(ctx, nil, nil, channel, startTime, endTime)
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

// GetRecentUsage returns recent usage logs with proper pagination.
func (s *Service) GetRecentUsage(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, page, limit int) ([]models.UsageLog, int64, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -30)

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	logs, err := s.usageRepo.GetByOrgOrProjectPaginated(ctx, &orgID, projectID, startTime, endTime, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, _ := s.usageRepo.CountByOrgOrProject(ctx, &orgID, projectID, startTime, endTime)

	// Set IsSuccess based on StatusCode
	for i := range logs {
		logs[i].IsSuccess = logs[i].StatusCode >= 200 && logs[i].StatusCode < 300
	}

	return logs, total, nil
}
