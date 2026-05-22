// Package billing provides billing and usage tracking.
//
// Note on the OrgID/UserID convention used across this package: the Order,
// Transaction, Subscription and Budget models all carry an `org_id` column,
// but in personal-account contexts the code stores the user_id in that column.
// All write paths and the corresponding repository read paths use the same
// convention, so the data is internally consistent — but the field name is
// misleading. Treat `Transaction.OrgID = userID` etc. as the billing principal
// for that record. Multi-user organizations sharing a single subscription is
// not currently supported by the billing layer.
package billing

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/quota"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockTimeout bounds how long a deduction transaction will wait on the user
// row's FOR UPDATE lock. Longer than this and we'd rather fail the request
// than hold a request goroutine indefinitely (deadlock or stuck peer).
const lockTimeout = 5 * time.Second

// roundCost rounds a USD amount to 6 decimal places. We sum many small
// per-token charges; without rounding the float64 representation drifts and
// reports start disagreeing with the underlying transaction log.
func roundCost(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

// lockTimeoutCounter is filled in by middleware to avoid an import cycle with
// the metrics defined in api/middleware. It is set by SetLockTimeoutCounter.
type lockTimeoutObserver func(operation string)

var lockTimeoutObserve lockTimeoutObserver

// SetLockTimeoutObserver wires a metric callback for FOR UPDATE timeouts.
// Called from cmd/server during bootstrap.
func SetLockTimeoutObserver(fn func(operation string)) { lockTimeoutObserve = fn }

func recordLockTimeout(op string) {
	if lockTimeoutObserve != nil {
		lockTimeoutObserve(op)
	}
}

// paymentAmountMismatchObserve mirrors lockTimeoutObserve for payment webhook
// amount-mismatch events. Wired from cmd/server bootstrap.
var paymentAmountMismatchObserve func(provider string)

// SetPaymentAmountMismatchObserver wires a metric callback for the payment
// amount mismatch counter. Mismatches must always be zero in a healthy system.
func SetPaymentAmountMismatchObserver(fn func(provider string)) {
	paymentAmountMismatchObserve = fn
}

func recordPaymentAmountMismatch(provider string) {
	if paymentAmountMismatchObserve != nil {
		paymentAmountMismatchObserve(provider)
	}
}

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

	s.calculateUsageLogCost(ctx, log)

	err = s.usageRepo.Update(ctx, log)

	// Refresh redis cache — use org-scoped key matching GetUsageSummary read path
	if s.redis != nil && err == nil && log.IsSuccess {
		s.incrUsageCache(ctx, log)
		// Note: no per-user IncrementUsage call here because UpdateUsageTokens
		// is the no-balance-service variant; the deduct path
		// (UpdateUsageTokensAndDeduct) handles the per-user counter.
	}

	return err
}

// UpdateUsageTokensAndDeduct finalizes a pre-recorded streaming usage log and
// deducts the computed cost in the same transaction when billing data is available.
func (s *Service) UpdateUsageTokensAndDeduct(ctx context.Context, logID uuid.UUID, requestTokens, responseTokens int, statusCode int, latencyMs int64, errorMessage string, balanceSvc *BalanceService, userID uuid.UUID, description string) error {
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

	s.calculateUsageLogCost(ctx, log)

	// 499 (client closed request) is not a success for analytics, but the
	// upstream provider already billed us for the tokens we streamed before
	// the client went away. Treat it as billable so dropping the connection
	// can't be used to consume tokens for free; only real upstream failures
	// (5xx, 4xx other than 499) skip deduction.
	billable := log.IsSuccess || statusCode == 499

	if balanceSvc == nil || !billable || log.Cost <= 0 {
		err := s.usageRepo.Update(ctx, log)
		if s.redis != nil && err == nil && billable {
			s.incrUsageCache(ctx, log)
			quota.IncrementUsage(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
		}
		return err
	}

	// Bound the FOR UPDATE wait so a stuck peer can't pin a request goroutine.
	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	var (
		alertEmail   string
		alertName    string
		alertBalance float64
		shouldAlert  bool
	)

	err = balanceSvc.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(log).Error; err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance = roundCost(user.Balance - log.Cost)
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      roundCost(-log.Cost),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: log.ID.String(),
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		// Capture alert state for after-commit dispatch — emails sent inside
		// the transaction would still go out on rollback.
		if user.Balance < 1.0 {
			alertEmail = user.Email
			alertName = user.Name
			alertBalance = user.Balance
			shouldAlert = true
		}
		return nil
	})

	if err != nil {
		if txCtx.Err() != nil {
			recordLockTimeout("update_usage_and_deduct")
		}
		billingRecordErrorsTotal.WithLabelValues("update_usage_and_deduct").Inc()
		return err
	}

	if shouldAlert {
		balanceSvc.sendLowBalanceAlert(ctx, userID, alertEmail, alertName, alertBalance)
	}

	if s.redis != nil {
		s.incrUsageCache(ctx, log)
		quota.IncrementUsage(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
	}
	return nil
}

// RecordUsage records API usage.
//
// Used for the streaming pre-record (status=Processing, tokens=0) and for
// requests that don't require balance deduction. The per-user quota counter is
// not incremented here because tokens=0 in the pre-record case; the deduct
// path increments it once final tokens are known.
func (s *Service) RecordUsage(ctx context.Context, log *models.UsageLog) error {
	s.calculateUsageLogCost(ctx, log)

	err := s.usageRepo.Create(ctx, log)

	// Refresh redis cache — use org-scoped key matching GetUsageSummary read path
	if s.redis != nil && err == nil {
		s.incrUsageCache(ctx, log)
		// Increment per-user quota only if this is a final record (has tokens).
		// The streaming pre-record has tokens=0; the finalize call will count it.
		if log.TotalTokens > 0 && log.UserID != uuid.Nil {
			quota.IncrementUsage(ctx, s.redis, s.logger, log.UserID.String(), int64(log.TotalTokens), log.Cost)
		}
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
	s.calculateUsageLogCost(ctx, log)

	// If no balance service or zero cost, fall back to simple insert.
	// Still bump the per-user quota counter so token-limit checks work for
	// free plans / zero-cost requests.
	if balanceSvc == nil || log.Cost <= 0 {
		err := s.usageRepo.Create(ctx, log)
		if s.redis != nil && err == nil {
			s.incrUsageCache(ctx, log)
			if log.TotalTokens > 0 {
				quota.IncrementUsage(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
			}
		}
		return err
	}

	// Atomic transaction: insert usage log + deduct balance
	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	var (
		alertEmail   string
		alertName    string
		alertBalance float64
		shouldAlert  bool
	)

	err := balanceSvc.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		// 1. Insert usage log within the transaction
		if err := tx.Create(log).Error; err != nil {
			return err
		}

		// 2. Lock user row and deduct balance
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance = roundCost(user.Balance - log.Cost)
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		// 3. Record the transaction
		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      roundCost(-log.Cost),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: log.ID.String(),
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		// Capture alert state; dispatch only on commit.
		if user.Balance < 1.0 {
			alertEmail = user.Email
			alertName = user.Name
			alertBalance = user.Balance
			shouldAlert = true
		}
		return nil
	})

	if err != nil {
		if txCtx.Err() != nil {
			recordLockTimeout("record_usage_and_deduct")
		}
		billingRecordErrorsTotal.WithLabelValues("record_usage_and_deduct").Inc()
		return err
	}

	if shouldAlert {
		balanceSvc.sendLowBalanceAlert(ctx, userID, alertEmail, alertName, alertBalance)
	}

	// Refresh redis caches outside the transaction.
	if s.redis != nil {
		s.incrUsageCache(ctx, log)
		quota.IncrementUsage(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
	}
	return nil
}

func (s *Service) calculateUsageLogCost(ctx context.Context, log *models.UsageLog) {
	model, err := s.modelForUsageLog(ctx, log)
	if err != nil || model == nil {
		return
	}

	log.ModelID = model.ID
	customerCharge := roundCost(s.calculateCustomerCharge(model, log))
	log.Cost = customerCharge
	log.CustomerCharge = customerCharge
	log.ProviderCost = roundCost(s.calculateProviderCost(model, log, customerCharge))
}

func (s *Service) modelForUsageLog(ctx context.Context, log *models.UsageLog) (*models.Model, error) {
	if s.modelRepo == nil {
		return nil, nil
	}
	if log.ModelID != uuid.Nil {
		return s.modelRepo.GetByID(ctx, log.ModelID)
	}
	if log.ProviderID != uuid.Nil && log.ModelName != "" {
		if model, err := s.modelRepo.GetByProviderAndName(ctx, log.ProviderID, log.ModelName); err == nil {
			return model, nil
		}
	}
	if log.ModelName != "" {
		return s.modelRepo.GetByName(ctx, log.ModelName)
	}
	return nil, nil
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
// that match the format read by GetUsageSummary. Failures are logged + counted
// but never returned: the caller has already committed to the DB and the cache
// is best-effort. Persistent failures show up in the
// llm_router_billing_quota_cache_increment_failures_total counter.
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
	if _, err := pipe.Exec(ctx); err != nil {
		quota.IncrementFailuresTotal.Inc()
		s.logger.Warn("billing usage cache increment failed",
			zap.String("org_id", orgID.String()),
			zap.Error(err),
		)
	}
}

// calculateCustomerCharge calculates the customer-facing charge for every populated metering dimension.
func (s *Service) calculateCustomerCharge(model *models.Model, log *models.UsageLog) float64 {
	inputCost := float64(log.RequestTokens) / 1000 * model.InputPricePer1K
	outputCost := float64(log.ResponseTokens) / 1000 * model.OutputPricePer1K

	durationSeconds := float64(log.DurationMs) / 1000
	secondCost := durationSeconds * model.PricePerSecond
	minuteCost := durationSeconds / 60 * model.PricePerMinute
	imageCost := float64(log.ItemCount) * model.PricePerImage

	return inputCost + outputCost + secondCost + minuteCost + imageCost
}

// calculateProviderCost calculates upstream provider cost. If provider-side
// rates are not configured yet, fall back to the customer charge so margin is
// conservative instead of falsely showing 100%.
func (s *Service) calculateProviderCost(model *models.Model, log *models.UsageLog, fallback float64) float64 {
	hasProviderRates := model.ProviderInputCostPer1K > 0 ||
		model.ProviderOutputCostPer1K > 0 ||
		model.ProviderCostPerSecond > 0 ||
		model.ProviderCostPerImage > 0 ||
		model.ProviderCostPerMinute > 0
	if !hasProviderRates {
		return fallback
	}

	inputCost := float64(log.RequestTokens) / 1000 * model.ProviderInputCostPer1K
	outputCost := float64(log.ResponseTokens) / 1000 * model.ProviderOutputCostPer1K

	durationSeconds := float64(log.DurationMs) / 1000
	secondCost := durationSeconds * model.ProviderCostPerSecond
	minuteCost := durationSeconds / 60 * model.ProviderCostPerMinute
	imageCost := float64(log.ItemCount) * model.ProviderCostPerImage

	return inputCost + outputCost + secondCost + minuteCost + imageCost
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
func (s *Service) GetRecentUsage(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, channel *string, page, limit int) ([]models.UsageLog, int64, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -30)

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	logs, err := s.usageRepo.GetByOrgOrProjectPaginated(ctx, &orgID, projectID, channel, startTime, endTime, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, _ := s.usageRepo.CountByOrgOrProject(ctx, &orgID, projectID, channel, startTime, endTime)

	// Set IsSuccess based on StatusCode
	for i := range logs {
		logs[i].IsSuccess = logs[i].StatusCode >= 200 && logs[i].StatusCode < 300
	}

	return logs, total, nil
}
