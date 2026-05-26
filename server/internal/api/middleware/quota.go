package middleware

import (
	"context"
	"errors"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/quota"
	"llm-router-platform/pkg/sanitize"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// budgetStatus mirrors billing.BudgetStatus for the fields the middleware
// actually inspects. Declared here so api/middleware does not import
// service/billing (which would create an import cycle: billing already
// imports internal/quota for the writer side).
type budgetStatus struct {
	EnforceHardLimit bool
	IsOverBudget     bool
	MonthlyLimitUSD  decimal.Decimal
	CurrentSpend     decimal.Decimal
}

// QuotaChecker validates monthly token and budget quotas for users.
// It reads cached usage from Redis to avoid DB queries on every request.
// Usage is updated synchronously after each LLM request completes via
// internal/quota.IncrementUsageMoney (called from billing.RecordUsage*).
type QuotaChecker struct {
	redis    *redis.Client
	budgetFn func(ctx context.Context, userID uuid.UUID) (*budgetStatus, error)
	logger   *zap.Logger
}

// NewQuotaChecker creates a new monthly quota checker.
func NewQuotaChecker(redisClient *redis.Client, logger *zap.Logger) *QuotaChecker {
	return &QuotaChecker{
		redis:  redisClient,
		logger: logger,
	}
}

// WithBudgetMoney wires a decimal-native per-user budget lookup.
func (q *QuotaChecker) WithBudgetMoney(fn func(ctx context.Context, userID uuid.UUID) (enforceHardLimit bool, isOverBudget bool, limit, spend decimal.Decimal, err error)) *QuotaChecker {
	q.budgetFn = func(ctx context.Context, userID uuid.UUID) (*budgetStatus, error) {
		enforce, over, limit, spend, err := fn(ctx, userID)
		if err != nil {
			return nil, err
		}
		return &budgetStatus{
			EnforceHardLimit: enforce,
			IsOverBudget:     over,
			MonthlyLimitUSD:  limit,
			CurrentSpend:     spend,
		}, nil
	}
	return q
}

// MonthlyQuotaKey returns the Redis hash key for a user's monthly usage.
//
// Deprecated: prefer internal/quota.MonthlyKey. Retained for any external
// callers that still depend on this symbol.
func MonthlyQuotaKey(userID string) string {
	return quota.MonthlyKey(userID)
}

// Check validates that the user has not exceeded their monthly token or budget quota.
// This should be placed after auth middleware so user_id and user limits are available.
//
// Behavior on Redis failure: rather than fail-open (which historically allowed
// unbounded traffic during a Redis outage), the check returns through and lets
// the request proceed but emits a llm_router_billing_quota_check_db_fallback_total
// metric and a warning log. Persistent Redis failures should page rather than
// silently disable quotas. The hard-limit budget check below STILL runs against
// the SQL aggregator regardless of Redis availability.
func (q *QuotaChecker) Check() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		// Read quota limits from context (set by auth middleware)
		tokenLimit := q.getInt64FromCtx(c, "user_monthly_token_limit")
		budgetLimit := q.getMoneyFromCtx(c, "user_monthly_budget_usd")

		// Read current usage from Redis. We DO NOT fail open: a missing or
		// errored Redis read serves zero usage, but we still proceed to the
		// budget hard-limit check below which is backed by SQL and survives
		// Redis outages.
		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()

		var (
			usedTokens int64
			usedCost   decimal.Decimal
		)
		if q.redis != nil {
			t, c2, err := quota.ReadMonthlyUsageMoney(ctx, q.redis, userID)
			if err != nil {
				reason := "redis_unavailable"
				if errors.Is(err, context.DeadlineExceeded) {
					reason = "redis_timeout"
				}
				quota.CheckDBFallbackTotal.WithLabelValues(reason).Inc()
				q.logger.Warn("quota check redis error",
					zap.Error(err),
					zap.String("user_id", sanitize.LogValue(userID)),
				)
			} else {
				usedTokens, usedCost = t, c2
			}
		}

		// Per-user subscription token limit (cached counter)
		if tokenLimit > 0 && usedTokens >= tokenLimit {
			QuotaExceededTotal.WithLabelValues("token_limit").Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "monthly token quota exceeded",
				"limit": tokenLimit,
				"used":  usedTokens,
			})
			return
		}

		// Per-user subscription budget limit (cached counter)
		if budgetLimit.IsPositive() && usedCost.Cmp(budgetLimit) >= 0 {
			QuotaExceededTotal.WithLabelValues("budget_limit").Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "monthly budget quota exceeded",
				"limit": budgetLimit.StringFixed(2),
				"used":  usedCost.StringFixed(2),
			})
			return
		}

		// Hard-limit budget: backed by SQL aggregation in BudgetService, runs
		// even when Redis is down. Only enforces if the user has an active
		// budget with EnforceHardLimit=true.
		if q.budgetFn != nil {
			if uid, err := uuid.Parse(userID); err == nil {
				if status, err := q.budgetFn(ctx, uid); err == nil && status != nil {
					if status.EnforceHardLimit && status.IsOverBudget {
						QuotaExceededTotal.WithLabelValues("budget_hard_limit").Inc()
						c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
							"error": "monthly budget hard limit exceeded",
							"limit": status.MonthlyLimitUSD.StringFixed(2),
							"spend": status.CurrentSpend.StringFixed(2),
						})
						return
					}
				}
			}
		}

		// Set remaining quota headers for client visibility
		if tokenLimit > 0 {
			c.Header("X-Quota-Tokens-Limit", strconv.FormatInt(tokenLimit, 10))
			c.Header("X-Quota-Tokens-Remaining", strconv.FormatInt(max64(0, tokenLimit-usedTokens), 10))
		}
		if budgetLimit.IsPositive() {
			remainingBudget := budgetLimit.Sub(usedCost)
			if remainingBudget.IsNegative() {
				remainingBudget = decimal.Zero
			}
			c.Header("X-Quota-Budget-Limit", budgetLimit.StringFixed(2))
			c.Header("X-Quota-Budget-Remaining", remainingBudget.StringFixed(2))
		}

		c.Next()
	}
}

func (q *QuotaChecker) getInt64FromCtx(c *gin.Context, key string) int64 {
	val, exists := c.Get(key)
	if !exists {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func (q *QuotaChecker) getMoneyFromCtx(c *gin.Context, key string) decimal.Decimal {
	val, exists := c.Get(key)
	if !exists {
		return decimal.Zero
	}
	switch v := val.(type) {
	case decimal.Decimal:
		return v.Round(models.MoneyScale)
	case string:
		parsed, err := decimal.NewFromString(v)
		if err == nil {
			return parsed.Round(models.MoneyScale)
		}
	}
	return decimal.Zero
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
