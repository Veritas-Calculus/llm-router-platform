package middleware

import (
	"context"
	"fmt"
	"llm-router-platform/pkg/sanitize"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// QuotaChecker validates monthly token and budget quotas for users.
// It reads cached usage from Redis to avoid DB queries on every request.
// Usage is updated asynchronously after each LLM request completes.
type QuotaChecker struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewQuotaChecker creates a new monthly quota checker.
func NewQuotaChecker(redisClient *redis.Client, logger *zap.Logger) *QuotaChecker {
	return &QuotaChecker{
		redis:  redisClient,
		logger: logger,
	}
}

// MonthlyQuotaKey returns the Redis hash key for a user's monthly usage.
func MonthlyQuotaKey(userID string) string {
	return fmt.Sprintf("quota:%s:%s", userID, time.Now().Format("2006-01"))
}

// Check validates that the user has not exceeded their monthly token or budget quota.
// This should be placed after auth middleware so user_id and user limits are available.
func (q *QuotaChecker) Check() gin.HandlerFunc {
	return func(c *gin.Context) {
		if q.redis == nil {
			c.Next()
			return
		}

		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		// Read quota limits from context (set by auth middleware)
		tokenLimit := q.getInt64FromCtx(c, "user_monthly_token_limit")
		budgetLimit := q.getFloat64FromCtx(c, "user_monthly_budget_usd")

		// If no limits configured, skip quota check
		if tokenLimit <= 0 && budgetLimit <= 0 {
			c.Next()
			return
		}

		// Get current usage from Redis
		quotaKey := MonthlyQuotaKey(userID)
		ctx := context.Background()

		result, err := q.redis.HGetAll(ctx, quotaKey).Result()
		if err != nil {
			// fail-open: if Redis is down, allow the request
			q.logger.Warn("quota check redis error, allowing request",
				zap.Error(err),
				zap.String("user_id", sanitize.LogValue(userID)),
			)
			c.Next()
			return
		}

		// Parse current usage
		usedTokens, _ := strconv.ParseInt(result["tokens"], 10, 64)
		usedCost, _ := strconv.ParseFloat(result["cost_usd"], 64)

		// Check token quota
		if tokenLimit > 0 && usedTokens >= tokenLimit {
			QuotaExceededTotal.WithLabelValues("token_limit").Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "monthly token quota exceeded",
				"limit": tokenLimit,
				"used":  usedTokens,
			})
			return
		}

		// Check budget quota
		if budgetLimit > 0 && usedCost >= budgetLimit {
			QuotaExceededTotal.WithLabelValues("budget_limit").Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "monthly budget quota exceeded",
				"limit": budgetLimit,
				"used":  usedCost,
			})
			return
		}

		// Set remaining quota headers for client visibility
		if tokenLimit > 0 {
			c.Header("X-Quota-Tokens-Limit", strconv.FormatInt(tokenLimit, 10))
			c.Header("X-Quota-Tokens-Remaining", strconv.FormatInt(max64(0, tokenLimit-usedTokens), 10))
		}
		if budgetLimit > 0 {
			c.Header("X-Quota-Budget-Limit", fmt.Sprintf("%.2f", budgetLimit))
			c.Header("X-Quota-Budget-Remaining", fmt.Sprintf("%.2f", max64f(0, budgetLimit-usedCost)))
		}

		c.Next()
	}
}

// IncrementUsage updates the Redis usage cache after a request completes.
// This should be called asynchronously from the handler after LLM response.
func IncrementUsage(redisClient *redis.Client, userID string, tokens int64, costUSD float64) {
	if redisClient == nil {
		return
	}

	ctx := context.Background()
	key := MonthlyQuotaKey(userID)

	pipe := redisClient.Pipeline()
	pipe.HIncrBy(ctx, key, "tokens", tokens)
	pipe.HIncrByFloat(ctx, key, "cost_usd", costUSD)
	pipe.Expire(ctx, key, 35*24*time.Hour) // 35 days TTL
	_, _ = pipe.Exec(ctx)
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

func (q *QuotaChecker) getFloat64FromCtx(c *gin.Context, key string) float64 {
	val, exists := c.Get(key)
	if !exists {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func max64f(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
