// Package middleware provides HTTP middleware functions.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RateLimiter provides request rate limiting backed by Redis.
// Falls back to an in-memory counter when Redis is unavailable (fail-closed).
type RateLimiter struct {
	requestsPerMinute int
	redisClient       *redis.Client
	logger            *zap.Logger

	// In-memory fallback when Redis is unavailable
	fallbackMu      sync.Mutex
	fallbackCounter map[string]*fallbackEntry
	fallbackEnabled atomic.Bool
}

// fallbackEntry tracks request count for in-memory rate limiting.
type fallbackEntry struct {
	count    int
	windowAt time.Time
}

// NewRateLimiter creates a new Redis-backed rate limiter.
// If redisClient is nil, the in-memory fallback is used instead of disabling rate limiting.
func NewRateLimiter(requestsPerMinute int, redisClient *redis.Client, logger *zap.Logger) *RateLimiter {
	r := &RateLimiter{
		requestsPerMinute: requestsPerMinute,
		redisClient:       redisClient,
		logger:            logger,
		fallbackCounter:   make(map[string]*fallbackEntry),
	}
	// M2: Start background cleanup goroutine to prevent memory leak
	go r.cleanupLoop()
	return r
}

// cleanupLoop periodically evicts expired entries from the in-memory fallback map.
func (r *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.fallbackMu.Lock()
		now := time.Now()
		for key, entry := range r.fallbackCounter {
			if now.Sub(entry.windowAt) > 2*time.Minute {
				delete(r.fallbackCounter, key)
			}
		}
		r.fallbackMu.Unlock()
	}
}

// Limit applies sliding-window rate limiting per API key (or client IP as fallback).
func (r *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer API key UUID from context (set by APIKey auth middleware)
		// so the Redis key pattern is consistent with PerKeyRateLimiter
		// and queryable by the rate-limit-status resolver.
		var identifier string
		rlSource := "per_ip"
		if keyVal, exists := c.Get("api_key"); exists {
			if apiKey, ok := keyVal.(*models.APIKey); ok {
				identifier = apiKey.ID.String()
				rlSource = "per_key"
			}
		}
		if identifier == "" {
			identifier = c.ClientIP()
		}

		// Determine effective rate limit
		effectiveLimit := r.requestsPerMinute

		// Check for per-user rate limit set by auth middleware
		if userLimit, exists := c.Get("user_rate_limit"); exists {
			if ul, ok := userLimit.(int); ok && ul > 0 {
				effectiveLimit = ul
			}
		}

		if r.redisClient == nil {
			r.limitInMemory(c, identifier)
			return
		}

		key := fmt.Sprintf("ratelimit:%s", identifier)
		ctx := context.Background()
		now := time.Now().UnixMilli()
		windowStart := now - 60000

		pipe := r.redisClient.Pipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
		pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: now})
		countCmd := pipe.ZCard(ctx, key)
		pipe.Expire(ctx, key, 2*time.Minute)

		_, err := pipe.Exec(ctx)
		if err != nil {
			r.logger.Warn("rate limiter redis error, using in-memory fallback",
				zap.Error(err),
				zap.String("identifier", sanitize.LogValue(identifier)),
			)
			RateLimitFallbackTotal.Inc()
			r.fallbackEnabled.Store(true)
			r.limitInMemory(c, identifier)
			return
		}

		// Clear fallback flag on successful Redis operation
		r.fallbackEnabled.Store(false)

		count := countCmd.Val()
		remaining := int64(effectiveLimit) - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(effectiveLimit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		if count > int64(effectiveLimit) {
			RateLimitExceededTotal.WithLabelValues(rlSource).Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60,
			})
			return
		}

		c.Next()
	}
}

// limitInMemory applies a simple per-minute counter as fallback when Redis is down.
func (r *RateLimiter) limitInMemory(c *gin.Context, identifier string) {
	r.fallbackMu.Lock()
	defer r.fallbackMu.Unlock()

	now := time.Now()
	entry, exists := r.fallbackCounter[identifier]
	if !exists || now.Sub(entry.windowAt) > time.Minute {
		r.fallbackCounter[identifier] = &fallbackEntry{count: 1, windowAt: now}
		c.Next()
		return
	}

	entry.count++
	if entry.count > r.requestsPerMinute {
		RateLimitExceededTotal.WithLabelValues("fallback").Inc()
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":       "rate limit exceeded (fallback)",
			"retry_after": 60,
		})
		return
	}

	c.Next()
}
