package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"llm-router-platform/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// PerKeyRateLimiter enforces per-API-key rate limits using Redis sliding window.
// It checks both per-minute and per-day limits from the APIKey model.
type PerKeyRateLimiter struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewPerKeyRateLimiter creates a new per-key rate limiter.
func NewPerKeyRateLimiter(redisClient *redis.Client, logger *zap.Logger) *PerKeyRateLimiter {
	return &PerKeyRateLimiter{
		redis:  redisClient,
		logger: logger,
	}
}

// Limit applies per-API-key rate limits (minute + daily).
func (l *PerKeyRateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.redis == nil {
			c.Next()
			return
		}

		keyVal, exists := c.Get("api_key")
		if !exists {
			c.Next()
			return
		}

		apiKey, ok := keyVal.(*models.APIKey)
		if !ok {
			c.Next()
			return
		}

		ctx := context.Background()

		// 1. Per-minute limit (sliding window)
		if apiKey.RateLimit > 0 {
			minuteKey := fmt.Sprintf("rl:key:%s:m", apiKey.ID.String())
			exceeded, current := l.checkSlidingWindow(ctx, minuteKey, apiKey.RateLimit, time.Minute)

			c.Header("X-RateLimit-Limit", strconv.Itoa(apiKey.RateLimit))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, apiKey.RateLimit-int(current)-1)))
			c.Header("X-RateLimit-Window", "60")

			if exceeded {
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", "60")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "API key rate limit exceeded",
					"limit":       apiKey.RateLimit,
					"window":      "1m",
					"retry_after": 60,
				})
				return
			}
		}

		// 2. Daily limit (simple counter)
		if apiKey.DailyLimit > 0 {
			today := time.Now().Format("2006-01-02")
			dailyKey := fmt.Sprintf("rl:key:%s:d:%s", apiKey.ID.String(), today)
			exceeded, current := l.checkDailyCounter(ctx, dailyKey, apiKey.DailyLimit)

			c.Header("X-DailyLimit-Limit", strconv.Itoa(apiKey.DailyLimit))
			c.Header("X-DailyLimit-Remaining", strconv.Itoa(max(0, apiKey.DailyLimit-int(current)-1)))

			if exceeded {
				c.Header("X-DailyLimit-Remaining", "0")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "API key daily limit exceeded",
					"limit":       apiKey.DailyLimit,
					"window":      "24h",
					"retry_after": l.secondsUntilMidnight(),
				})
				return
			}
		}

		c.Next()
	}
}

// checkSlidingWindow implements a Redis sorted-set sliding window counter.
// Returns (exceeded, currentCount).
func (l *PerKeyRateLimiter) checkSlidingWindow(ctx context.Context, key string, limit int, window time.Duration) (bool, int64) {
	now := time.Now()
	windowStart := now.Add(-window)

	pipe := l.redis.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, &redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})
	pipe.Expire(ctx, key, window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		l.logger.Warn("per-key rate limiter redis error", zap.Error(err))
		return false, 0 // fail-open
	}

	count := countCmd.Val()
	return count >= int64(limit), count
}

// checkDailyCounter uses a simple Redis INCR with TTL for daily limits.
// Returns (exceeded, currentCount).
func (l *PerKeyRateLimiter) checkDailyCounter(ctx context.Context, key string, limit int) (bool, int64) {
	count, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		l.logger.Warn("per-key daily limiter redis error", zap.Error(err))
		return false, 0 // fail-open
	}

	if count == 1 {
		// Set TTL on first use — expires at end of day + buffer
		l.redis.Expire(ctx, key, 25*time.Hour)
	}

	return count > int64(limit), count
}

// secondsUntilMidnight returns seconds until next UTC midnight.
func (l *PerKeyRateLimiter) secondsUntilMidnight() int {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return int(midnight.Sub(now).Seconds())
}

// ─── Per-User Rate Limiter ──────────────────────────────────────────────

// PerUserRateLimiter enforces per-user rate limits.
// Uses User.RateLimitPerMinute if set, otherwise falls through to global limiter.
type PerUserRateLimiter struct {
	redis         *redis.Client
	globalDefault int // fallback if user has no custom limit
	logger        *zap.Logger
}

// NewPerUserRateLimiter creates a per-user rate limiter.
func NewPerUserRateLimiter(redisClient *redis.Client, globalDefault int, logger *zap.Logger) *PerUserRateLimiter {
	return &PerUserRateLimiter{
		redis:         redisClient,
		globalDefault: globalDefault,
		logger:        logger,
	}
}

// Limit applies per-user rate limiting using the user's configured limit or global default.
func (l *PerUserRateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.redis == nil {
			c.Next()
			return
		}

		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		// Determine the rate limit: per-user override or global default
		limit := l.globalDefault
		if userLimit, exists := c.Get("user_rate_limit"); exists {
			if ul, ok := userLimit.(int); ok && ul > 0 {
				limit = ul
			}
		}

		if limit <= 0 {
			c.Next()
			return
		}

		key := fmt.Sprintf("rl:user:%s:m", userID)
		now := time.Now()
		windowStart := now.Add(-time.Minute)
		ctx := context.Background()

		pipe := l.redis.Pipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
		countCmd := pipe.ZCard(ctx, key)
		pipe.ZAdd(ctx, key, &redis.Z{
			Score:  float64(now.UnixNano()),
			Member: fmt.Sprintf("%d", now.UnixNano()),
		})
		pipe.Expire(ctx, key, 2*time.Minute)

		if _, err := pipe.Exec(ctx); err != nil {
			l.logger.Warn("per-user rate limiter redis error", zap.Error(err))
			c.Next()
			return
		}

		count := countCmd.Val()

		c.Header("X-UserRateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-UserRateLimit-Remaining", strconv.Itoa(max(0, limit-int(count)-1)))

		if count >= int64(limit) {
			c.Header("X-UserRateLimit-Remaining", "0")
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "user rate limit exceeded",
				"limit":       limit,
				"window":      "1m",
				"retry_after": 60,
			})
			return
		}

		c.Next()
	}
}
