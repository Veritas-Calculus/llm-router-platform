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
	"net/http"
	"strconv"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/quota"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockTimeout bounds how long a deduction transaction will wait on the user
// row's FOR UPDATE lock. Longer than this and we'd rather fail the request
// than hold a request goroutine indefinitely (deadlock or stuck peer).
const lockTimeout = 5 * time.Second

var (
	decimalThousand     = decimal.NewFromInt(1000)
	decimalMinuteMs     = decimal.NewFromInt(60_000)
	lowBalanceThreshold = decimal.NewFromInt(1)
)

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

	if balanceSvc == nil || !billable || !log.Cost.IsPositive() {
		err := s.usageRepo.Update(ctx, log)
		if s.redis != nil && err == nil && billable {
			s.incrUsageCache(ctx, log)
			quota.IncrementUsageMoney(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
		}
		return err
	}

	// Bound the FOR UPDATE wait so a stuck peer can't pin a request goroutine.
	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	var (
		alertEmail   string
		alertName    string
		alertBalance decimal.Decimal
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

		user.Balance = models.MoneySub(user.Balance, log.Cost)
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance", user.Balance).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      models.MoneyNeg(log.Cost),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: log.ID.String(),
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		// Capture alert state for after-commit dispatch — emails sent inside
		// the transaction would still go out on rollback.
		if user.Balance.Cmp(lowBalanceThreshold) < 0 {
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
		quota.IncrementUsageMoney(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
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
			quota.IncrementUsageMoney(ctx, s.redis, s.logger, log.UserID.String(), int64(log.TotalTokens), log.Cost)
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
	if balanceSvc == nil || !log.Cost.IsPositive() {
		err := s.usageRepo.Create(ctx, log)
		if s.redis != nil && err == nil {
			s.incrUsageCache(ctx, log)
			if log.TotalTokens > 0 {
				quota.IncrementUsageMoney(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
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
		alertBalance decimal.Decimal
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

		user.Balance = models.MoneySub(user.Balance, log.Cost)
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance", user.Balance).Error; err != nil {
			return err
		}

		// 3. Record the transaction
		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      models.MoneyNeg(log.Cost),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: log.ID.String(),
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		// Capture alert state; dispatch only on commit.
		if user.Balance.Cmp(lowBalanceThreshold) < 0 {
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
		quota.IncrementUsageMoney(ctx, s.redis, s.logger, userID.String(), int64(log.TotalTokens), log.Cost)
	}
	return nil
}

func (s *Service) calculateUsageLogCost(ctx context.Context, log *models.UsageLog) {
	model, err := s.modelForUsageLog(ctx, log)
	if err != nil || model == nil {
		return
	}

	log.ModelID = model.ID
	customerCharge := s.calculateCustomerCharge(model, log)
	log.Cost = customerCharge
	log.CustomerCharge = customerCharge
	log.ProviderCost = s.calculateProviderCost(model, log, customerCharge)
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
func (bs *BalanceService) sendLowBalanceAlert(ctx context.Context, userID uuid.UUID, email, name string, balance decimal.Decimal) {
	if bs.redis == nil || bs.emailSvc == nil || balance.Cmp(lowBalanceThreshold) >= 0 {
		return
	}

	cacheKey := fmt.Sprintf("quota_warn:balance:%s", userID.String())
	if err := bs.redis.Get(ctx, cacheKey).Err(); err != redis.Nil {
		return // Already sent recently or Redis error
	}

	balanceText := balance.Round(models.MoneyScale).StringFixed(2)
	bs.logger.Info("sending low balance warning email", zap.String("userID", userID.String()), zap.String("balance", balanceText))
	go func(to, uname, currentBalance string) {
		if err := bs.emailSvc.SendQuotaWarningEmail(to, uname, "$"+currentBalance, "$1.00"); err != nil {
			bs.logger.Error("failed to send quota warning email", zap.Error(err))
		}
	}(email, name, balanceText)
	bs.redis.Set(ctx, cacheKey, "1", 24*time.Hour)
}

// incrUsageCache increments the Redis usage cache using org-scoped keys
// that match the format read by GetUsageSummary. Failures are logged + counted
// but never returned: the caller has already committed to the DB and the cache
// is best-effort. Persistent failures show up in the
// llm_router_billing_quota_cache_increment_failures_total counter.
func (s *Service) incrUsageCache(ctx context.Context, log *models.UsageLog) {
	if !shouldIncrementUsageSummaryCache(log) {
		return
	}

	// Look up the org ID from the project
	orgID, err := s.usageRepo.GetOrgIDByProjectID(ctx, log.ProjectID)
	if err != nil || orgID == uuid.Nil {
		return
	}

	key := usageSummaryCacheKey(orgID, time.Now())
	customerCharge := usageLogCustomerCharge(log)

	pipe := s.redis.Pipeline()
	pipe.HIncrBy(ctx, key, usageCacheTotalRequests, 1)
	pipe.HIncrBy(ctx, key, usageCacheTotalTokens, int64(log.TotalTokens))
	pipe.HIncrBy(ctx, key, usageCacheTotalCostUnits, models.MoneyToUnits(customerCharge))
	pipe.HIncrBy(ctx, key, usageCacheLatencySum, log.Latency)
	if log.StatusCode >= 200 && log.StatusCode < 300 {
		pipe.HIncrBy(ctx, key, usageCacheSuccessCount, 1)
		pipe.HIncrBy(ctx, key, usageCacheErrorCount, 0)
	} else {
		pipe.HIncrBy(ctx, key, usageCacheSuccessCount, 0)
		pipe.HIncrBy(ctx, key, usageCacheErrorCount, 1)
	}
	pipe.HIncrBy(ctx, key, usageCacheMCPCallCount, int64(log.MCPCallCount))
	pipe.HIncrBy(ctx, key, usageCacheMCPErrorCount, int64(log.MCPErrorCount))
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
func (s *Service) calculateCustomerCharge(model *models.Model, log *models.UsageLog) decimal.Decimal {
	return tokenCost(log.RequestTokens, model.InputPricePer1K).
		Add(tokenCost(log.ResponseTokens, model.OutputPricePer1K)).
		Add(durationMsCost(log.DurationMs, model.PricePerSecond, model.PricePerMinute)).
		Add(itemCost(log.ItemCount, model.PricePerImage)).
		Round(models.MoneyScale)
}

// calculateProviderCost calculates upstream provider cost. If provider-side
// rates are not configured yet, fall back to the customer charge so margin is
// conservative instead of falsely showing 100%.
func (s *Service) calculateProviderCost(model *models.Model, log *models.UsageLog, fallback decimal.Decimal) decimal.Decimal {
	hasProviderRates := model.ProviderInputCostPer1K.IsPositive() ||
		model.ProviderOutputCostPer1K.IsPositive() ||
		model.ProviderCostPerSecond.IsPositive() ||
		model.ProviderCostPerImage.IsPositive() ||
		model.ProviderCostPerMinute.IsPositive()
	if !hasProviderRates {
		return fallback
	}

	return tokenCost(log.RequestTokens, model.ProviderInputCostPer1K).
		Add(tokenCost(log.ResponseTokens, model.ProviderOutputCostPer1K)).
		Add(durationMsCost(log.DurationMs, model.ProviderCostPerSecond, model.ProviderCostPerMinute)).
		Add(itemCost(log.ItemCount, model.ProviderCostPerImage)).
		Round(models.MoneyScale)
}

func tokenCost(tokens int, pricePer1K decimal.Decimal) decimal.Decimal {
	if tokens == 0 || pricePer1K.IsZero() {
		return decimal.Zero
	}
	return decimal.NewFromInt(int64(tokens)).
		Mul(pricePer1K).
		Div(decimalThousand)
}

func durationMsCost(durationMs int64, pricePerSecond, pricePerMinute decimal.Decimal) decimal.Decimal {
	if durationMs == 0 {
		return decimal.Zero
	}
	duration := decimal.NewFromInt(durationMs)
	cost := decimal.Zero
	if !pricePerSecond.IsZero() {
		cost = cost.Add(duration.Div(decimalThousand).Mul(pricePerSecond))
	}
	if !pricePerMinute.IsZero() {
		cost = cost.Add(duration.Div(decimalMinuteMs).Mul(pricePerMinute))
	}
	return cost
}

func itemCost(count int, pricePerItem decimal.Decimal) decimal.Decimal {
	if count == 0 || pricePerItem.IsZero() {
		return decimal.Zero
	}
	return decimal.NewFromInt(int64(count)).Mul(pricePerItem)
}

// UsageSummary represents aggregated usage data.
type UsageSummary struct {
	TotalRequests int64           `json:"total_requests"`
	TotalTokens   int64           `json:"total_tokens"`
	TotalCost     decimal.Decimal `json:"total_cost"`
	AvgLatency    float64         `json:"avg_latency"`
	SuccessCount  int64           `json:"success_count"`
	SuccessRate   float64         `json:"success_rate"`
	ErrorCount    int64           `json:"error_count"`
	MCPCallCount  int64           `json:"mcp_call_count"`
	MCPErrorCount int64           `json:"mcp_error_count"`
}

const (
	usageCacheTotalRequests  = "total_requests"
	usageCacheTotalTokens    = "total_tokens"
	usageCacheTotalCost      = "total_cost"
	usageCacheTotalCostUnits = "total_cost_units"
	usageCacheLatencySum     = "latency_sum"
	usageCacheSuccessCount   = "success_count"
	usageCacheErrorCount     = "error_count"
	usageCacheMCPCallCount   = "mcp_call_count"
	usageCacheMCPErrorCount  = "mcp_error_count"
)

func usageSummaryCacheKey(orgID uuid.UUID, t time.Time) string {
	return fmt.Sprintf("billing:usage:org:%s:%d-%02d", orgID.String(), t.Year(), t.Month())
}

func usageSummaryFromCache(res map[string]string) (*UsageSummary, bool) {
	totalRequests, ok := cachedInt64(res, usageCacheTotalRequests)
	if !ok {
		return nil, false
	}
	totalTokens, ok := cachedInt64(res, usageCacheTotalTokens)
	if !ok {
		return nil, false
	}
	totalCost, ok := cachedMoney(res, usageCacheTotalCostUnits, usageCacheTotalCost)
	if !ok {
		return nil, false
	}
	latencySum, ok := cachedInt64(res, usageCacheLatencySum)
	if !ok {
		return nil, false
	}
	successCount, ok := cachedInt64(res, usageCacheSuccessCount)
	if !ok {
		return nil, false
	}
	errorCount, ok := cachedInt64(res, usageCacheErrorCount)
	if !ok {
		return nil, false
	}
	mcpCallCount, ok := cachedInt64(res, usageCacheMCPCallCount)
	if !ok {
		return nil, false
	}
	mcpErrorCount, ok := cachedInt64(res, usageCacheMCPErrorCount)
	if !ok {
		return nil, false
	}

	summary := &UsageSummary{
		TotalRequests: totalRequests,
		TotalTokens:   totalTokens,
		TotalCost:     totalCost,
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
		MCPCallCount:  mcpCallCount,
		MCPErrorCount: mcpErrorCount,
	}
	if totalRequests > 0 {
		summary.AvgLatency = float64(latencySum) / float64(totalRequests)
		summary.SuccessRate = float64(successCount) / float64(totalRequests) * 100
	}
	return summary, true
}

func cachedInt64(res map[string]string, field string) (int64, bool) {
	raw, ok := res[field]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil
}

func cachedMoney(res map[string]string, unitsField, fallbackField string) (decimal.Decimal, bool) {
	if raw, ok := res[unitsField]; ok {
		units, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return models.MoneyFromUnits(units), true
		}
	}
	raw, ok := res[fallbackField]
	if !ok {
		return decimal.Zero, false
	}
	v, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, false
	}
	return v.Round(models.MoneyScale), true
}

func shouldIncrementUsageSummaryCache(log *models.UsageLog) bool {
	return log != nil && log.StatusCode != http.StatusProcessing
}

func usageLogCustomerCharge(log *models.UsageLog) decimal.Decimal {
	if !log.CustomerCharge.IsZero() {
		return log.CustomerCharge.Round(models.MoneyScale)
	}
	return log.Cost.Round(models.MoneyScale)
}

// GetUsageSummary returns aggregated usage for an organization or project.
func (s *Service) GetUsageSummary(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, channel *string, startTime, endTime time.Time) (*UsageSummary, error) {
	now := time.Now()
	isCurrentMonth := startTime.Year() == now.Year() && startTime.Month() == now.Month()

	// Redis cache is only populated at the org level (no project/channel dims),
	// so only attempt a cache hit when no sub-filters are applied.
	useCache := s.redis != nil && isCurrentMonth && projectID == nil && (channel == nil || *channel == "")

	if useCache {
		key := usageSummaryCacheKey(orgID, now)
		res, err := s.redis.HGetAll(ctx, key).Result()
		if err == nil {
			if summary, ok := usageSummaryFromCache(res); ok {
				return summary, nil
			}
		}
	}

	row, err := s.usageRepo.AggregateByTimeRange(ctx, &orgID, projectID, channel, startTime, endTime)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		TotalRequests: row.TotalRequests,
		TotalTokens:   row.TotalTokens,
		TotalCost:     row.TotalCost.Round(models.MoneyScale),
		AvgLatency:    row.AvgLatency,
		SuccessCount:  row.SuccessCount,
		ErrorCount:    row.ErrorCount,
		MCPCallCount:  row.MCPCallCount,
		MCPErrorCount: row.MCPErrorCount,
	}
	if row.TotalRequests > 0 {
		summary.SuccessRate = float64(row.SuccessCount) / float64(row.TotalRequests) * 100
	}

	if useCache && summary.TotalRequests > 0 {
		key := usageSummaryCacheKey(orgID, now)
		pipe := s.redis.Pipeline()
		pipe.HSet(ctx, key, map[string]interface{}{
			usageCacheTotalRequests:  summary.TotalRequests,
			usageCacheTotalTokens:    summary.TotalTokens,
			usageCacheTotalCost:      summary.TotalCost.StringFixed(models.MoneyScale),
			usageCacheTotalCostUnits: models.MoneyToUnits(summary.TotalCost),
			usageCacheLatencySum:     int64(summary.AvgLatency * float64(summary.TotalRequests)),
			usageCacheSuccessCount:   summary.SuccessCount,
			usageCacheErrorCount:     summary.ErrorCount,
			usageCacheMCPCallCount:   summary.MCPCallCount,
			usageCacheMCPErrorCount:  summary.MCPErrorCount,
		})
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
		TotalCost:     row.TotalCost.Round(models.MoneyScale),
		AvgLatency:    row.AvgLatency,
		SuccessCount:  row.SuccessCount,
		ErrorCount:    row.ErrorCount,
		MCPCallCount:  row.MCPCallCount,
		MCPErrorCount: row.MCPErrorCount,
	}
	if row.TotalRequests > 0 {
		summary.SuccessRate = float64(row.SuccessCount) / float64(row.TotalRequests) * 100
	}

	return summary, nil
}

// GetUserUsageSummary returns aggregated usage for a single user.
func (s *Service) GetUserUsageSummary(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time) (*UsageSummary, error) {
	row, err := s.usageRepo.AggregateByUserTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		TotalRequests: row.TotalRequests,
		TotalTokens:   row.TotalTokens,
		TotalCost:     row.TotalCost.Round(models.MoneyScale),
		AvgLatency:    row.AvgLatency,
		SuccessCount:  row.SuccessCount,
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
	Date     string          `json:"date"`
	Requests int64           `json:"requests"`
	Tokens   int64           `json:"tokens"`
	Cost     decimal.Decimal `json:"cost"`
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
		result[i] = DailyUsage{Date: r.Date, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost.Round(models.MoneyScale)}
	}
	return result, nil
}

// GetUserDailyUsage returns daily usage statistics for a single user.
func (s *Service) GetUserDailyUsage(ctx context.Context, userID uuid.UUID, days int) ([]DailyUsage, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	rows, err := s.usageRepo.AggregateDailyByUserTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]DailyUsage, len(rows))
	for i, r := range rows {
		result[i] = DailyUsage{Date: r.Date, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost.Round(models.MoneyScale)}
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
		result[i] = DailyUsage{Date: r.Date, Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost.Round(models.MoneyScale)}
	}
	return result, nil
}

// ProviderUsage represents usage per provider.
type ProviderUsage struct {
	ProviderID   uuid.UUID       `json:"provider_id"`
	ProviderName string          `json:"provider_name"`
	Requests     int64           `json:"requests"`
	Tokens       int64           `json:"tokens"`
	Cost         decimal.Decimal `json:"cost"`
	SuccessRate  float64         `json:"success_rate"`
	AvgLatency   float64         `json:"avg_latency_ms"`
}

func mapProviderRows(rows []repository.ProviderUsageRow) []ProviderUsage {
	result := make([]ProviderUsage, len(rows))
	for i, r := range rows {
		result[i] = ProviderUsage{
			ProviderID: r.ProviderID, ProviderName: r.ProviderName,
			Requests: r.Requests, Tokens: r.Tokens, Cost: r.Cost.Round(models.MoneyScale),
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
	ModelID      uuid.UUID       `json:"model_id"`
	ModelName    string          `json:"model_name"`
	Requests     int64           `json:"requests"`
	InputTokens  int64           `json:"input_tokens"`
	OutputTokens int64           `json:"output_tokens"`
	TotalTokens  int64           `json:"total_tokens"`
	Cost         decimal.Decimal `json:"cost"`
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
			Cost:         r.Cost.Round(models.MoneyScale),
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
			Cost:         r.Cost.Round(models.MoneyScale),
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
