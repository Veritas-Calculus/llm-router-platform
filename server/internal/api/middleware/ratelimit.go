package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"llm-router-platform/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PerKeyRateLimiter enforces per-API-key rate limits using Redis sliding window.
// It checks both per-minute and per-day limits from the APIKey model.
// When Redis is unavailable, it falls back to an in-memory counter.
type PerKeyRateLimiter struct {
	redis  *redis.Client
	logger *zap.Logger
	// In-memory fallback when Redis is down
	fallbackMu      sync.Mutex
	fallbackCounter map[string]*rateFallbackEntry
}

// rateFallbackEntry tracks request count for in-memory rate limiting.
type rateFallbackEntry struct {
	count    int
	windowAt time.Time
}

// NewPerKeyRateLimiter creates a new per-key rate limiter.
func NewPerKeyRateLimiter(redisClient *redis.Client, logger *zap.Logger) *PerKeyRateLimiter {
	return &PerKeyRateLimiter{
		redis:           redisClient,
		logger:          logger,
		fallbackCounter: make(map[string]*rateFallbackEntry),
	}
}

// Limit applies per-API-key rate limits (minute + daily).
// When Redis is unavailable, per-minute limits use an in-memory sliding counter.
func (l *PerKeyRateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
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

		if l.redis == nil {
			l.limitInMemoryFallback(c, apiKey)
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

		// 3. Tokens Per Minute (TPM)
		if apiKey.TokenLimit > 0 {
			tpmKey := fmt.Sprintf("rl:tpm:%s:%d", apiKey.ID.String(), time.Now().Unix()/60)

			currentStr := l.redis.Get(ctx, tpmKey).Val()
			currentTokens, _ := strconv.ParseInt(currentStr, 10, 64)

			c.Header("X-RateLimit-Tokens-Limit", strconv.FormatInt(apiKey.TokenLimit, 10))
			c.Header("X-RateLimit-Tokens-Remaining", strconv.FormatInt(max(0, apiKey.TokenLimit-currentTokens), 10))

			if currentTokens >= apiKey.TokenLimit {
				c.Header("X-RateLimit-Tokens-Remaining", "0")
				c.Header("Retry-After", "60")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "API key token limit exceeded",
					"limit":       apiKey.TokenLimit,
					"window":      "1m",
					"retry_after": 60,
				})
				return
			}
		}

		c.Next()
	}
}

// limitInMemoryFallback applies in-memory rate limiting for per-key limits
// when Redis is unavailable (per-minute only; daily/TPM require Redis).
func (l *PerKeyRateLimiter) limitInMemoryFallback(c *gin.Context, apiKey *models.APIKey) {
	if apiKey.RateLimit > 0 {
		key := fmt.Sprintf("rl:key:%s:m", apiKey.ID.String())
		exceeded, _ := l.fallbackCheck(key, apiKey.RateLimit, time.Minute)
		if exceeded {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "API key rate limit exceeded (fallback)",
				"limit":       apiKey.RateLimit,
				"window":      "1m",
				"retry_after": 60,
			})
			return
		}
	}
	c.Next()
}

// checkSlidingWindow implements a Redis sorted-set sliding window counter.
// Returns (exceeded, currentCount).
func (l *PerKeyRateLimiter) checkSlidingWindow(ctx context.Context, key string, limit int, window time.Duration) (bool, int64) {
	now := time.Now()
	windowStart := now.Add(-window)

	pipe := l.redis.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})
	pipe.Expire(ctx, key, window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		l.logger.Warn("per-key rate limiter redis error, using in-memory fallback", zap.Error(err))
		return l.fallbackCheck(key, limit, window)
	}

	count := countCmd.Val()
	return count >= int64(limit), count
}

// checkDailyCounter uses a simple Redis INCR with TTL for daily limits.
// Returns (exceeded, currentCount).
func (l *PerKeyRateLimiter) checkDailyCounter(ctx context.Context, key string, limit int) (bool, int64) {
	count, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		l.logger.Warn("per-key daily limiter redis error, using in-memory fallback", zap.Error(err))
		return l.fallbackCheck(key, limit, 24*time.Hour)
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

// fallbackCheck provides in-memory rate limiting when Redis is unavailable.
func (l *PerKeyRateLimiter) fallbackCheck(key string, limit int, window time.Duration) (bool, int64) {
	l.fallbackMu.Lock()
	defer l.fallbackMu.Unlock()

	now := time.Now()
	entry, exists := l.fallbackCounter[key]
	if !exists || now.Sub(entry.windowAt) > window {
		l.fallbackCounter[key] = &rateFallbackEntry{count: 1, windowAt: now}
		return false, 1
	}

	entry.count++
	return entry.count > limit, int64(entry.count)
}

// ─── Per-User Rate Limiter ──────────────────────────────────────────────

// PerUserRateLimiter enforces per-user rate limits.
// Uses User.RateLimitPerMinute if set, otherwise falls through to global limiter.
type PerUserRateLimiter struct {
	redis           *redis.Client
	globalDefault   int // fallback if user has no custom limit
	logger          *zap.Logger
	fallbackMu      sync.Mutex
	fallbackCounter map[string]*rateFallbackEntry
}

// NewPerUserRateLimiter creates a per-user rate limiter.
func NewPerUserRateLimiter(redisClient *redis.Client, globalDefault int, logger *zap.Logger) *PerUserRateLimiter {
	return &PerUserRateLimiter{
		redis:           redisClient,
		globalDefault:   globalDefault,
		logger:          logger,
		fallbackCounter: make(map[string]*rateFallbackEntry),
	}
}

// Limit applies per-user rate limiting using the user's configured limit or global default.
// When Redis is unavailable, uses an in-memory fallback counter.
func (l *PerUserRateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

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

		if l.redis == nil {
			l.limitInMemoryFallback(c, key, limit)
			return
		}

		now := time.Now()
		windowStart := now.Add(-time.Minute)
		ctx := context.Background()

		pipe := l.redis.Pipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
		countCmd := pipe.ZCard(ctx, key)
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(now.UnixNano()),
			Member: fmt.Sprintf("%d", now.UnixNano()),
		})
		pipe.Expire(ctx, key, 2*time.Minute)

		if _, err := pipe.Exec(ctx); err != nil {
			l.logger.Warn("per-user rate limiter redis error, using in-memory fallback", zap.Error(err))
			l.limitInMemoryFallback(c, key, limit)
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

// limitInMemoryFallback applies in-memory rate limiting when Redis is unavailable.
func (l *PerUserRateLimiter) limitInMemoryFallback(c *gin.Context, key string, limit int) {
	l.fallbackMu.Lock()
	defer l.fallbackMu.Unlock()

	now := time.Now()
	entry, exists := l.fallbackCounter[key]
	if !exists || now.Sub(entry.windowAt) > time.Minute {
		l.fallbackCounter[key] = &rateFallbackEntry{count: 1, windowAt: now}
		c.Next()
		return
	}

	entry.count++
	if entry.count > limit {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":       "user rate limit exceeded (fallback)",
			"retry_after": 60,
		})
		return
	}

	c.Next()
}
