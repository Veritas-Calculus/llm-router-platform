package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// tokenBucketScript is a Redis Lua script implementing an atomic token bucket.
// KEYS[1] = bucket key
// ARGV[1] = bucket capacity (max tokens)
// ARGV[2] = refill rate (tokens per second)
// ARGV[3] = current timestamp (seconds, float)
// ARGV[4] = tokens to consume (usually 1)
//
// Returns: {allowed (0/1), remaining_tokens, retry_after_seconds}
// #nosec G101 -- Redis Lua script, not a credential
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

-- Initialize bucket if first access
if tokens == nil then
    tokens = capacity
    last_refill = now
end

-- Refill tokens based on elapsed time
local elapsed = math.max(0, now - last_refill)
local refill = elapsed * rate
tokens = math.min(capacity, tokens + refill)

-- Try to consume
local allowed = 0
local retry_after = 0
if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
else
    -- Calculate wait time until enough tokens are available
    retry_after = math.ceil((requested - tokens) / rate)
end

-- Persist state
redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
redis.call('EXPIRE', key, math.ceil(capacity / rate) + 10)

return {allowed, math.floor(tokens), retry_after}
`

// TokenBucketLimiter implements per-key token bucket rate limiting using a Redis Lua script.
// Each request consumes one token; tokens refill at a steady rate.
type TokenBucketLimiter struct {
	redis    *redis.Client
	script   *redis.Script
	capacity int     // Maximum burst size
	rate     float64 // Tokens refilled per second
	logger   *zap.Logger
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
// capacity is the maximum burst size, ratePerMinute is the sustained request rate.
func NewTokenBucketLimiter(redisClient *redis.Client, capacity int, ratePerMinute int, logger *zap.Logger) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		redis:    redisClient,
		script:   redis.NewScript(tokenBucketScript),
		capacity: capacity,
		rate:     float64(ratePerMinute) / 60.0, // convert to per-second
		logger:   logger,
	}
}

// Limit applies token bucket rate limiting keyed by user_id or API key.
func (l *TokenBucketLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.redis == nil {
			c.Next()
			return
		}

		// Use user_id as the rate limit key
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		key := fmt.Sprintf("tb:user:%s", userID)
		ctx := context.Background()
		now := float64(time.Now().UnixNano()) / 1e9 // seconds with nanosecond precision

		result, err := l.script.Run(ctx, l.redis, []string{key},
			l.capacity, l.rate, now, 1,
		).Int64Slice()

		if err != nil {
			l.logger.Warn("token bucket lua script error, allowing request", zap.Error(err))
			c.Next()
			return
		}

		allowed := result[0] == 1
		remaining := result[1]
		retryAfter := result[2]

		c.Header("X-RateLimit-Limit", strconv.Itoa(l.capacity))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if !allowed {
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"limit":       l.capacity,
				"remaining":   remaining,
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}
