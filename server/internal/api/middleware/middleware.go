// Package middleware provides HTTP middleware functions.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// sanitizeLogString sanitizes user input for safe logging by removing
// potentially dangerous characters like newlines and carriage returns.
// This prevents log injection attacks.
func sanitizeLogString(s string) string {
	// Standard log sanitization: replace control characters and limit charset
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		// Allow standard printable ASCII characters
		if r >= 32 && r <= 126 {
			return r
		}
		// Replace everything else with a safe placeholder
		return '?'
	}, s)
}

// AuthMiddleware handles JWT authentication.
type AuthMiddleware struct {
	jwtSecret   []byte
	userService *user.Service
	logger      *zap.Logger
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(cfg *config.JWTConfig, userService *user.Service, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:   []byte(cfg.Secret),
		userService: userService,
		logger:      logger,
	}
}

// JWT validates JWT token in Authorization header.
// After signature verification, it queries the database to confirm
// the user's current role and active status (defense against stale claims).
func (m *AuthMiddleware) JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		userIDStr, _ := claims["sub"].(string)

		// Query database for real-time user state instead of trusting JWT claims.
		// This ensures role changes and account disabling take effect immediately.
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			AuthFailuresTotal.WithLabelValues("invalid_token").Inc()
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		userObj, err := m.userService.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		if !userObj.IsActive {
			AuthFailuresTotal.WithLabelValues("account_disabled").Inc()
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account is disabled"})
			return
		}

		// Check if token was issued before a forced invalidation
		// (password change, admin force-logout, etc.)
		if !userObj.TokensInvalidatedAt.IsZero() {
			iat, _ := claims.GetIssuedAt()
			if iat != nil && iat.Before(userObj.TokensInvalidatedAt) {
				AuthFailuresTotal.WithLabelValues("token_revoked").Inc()
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				return
			}
		}

		c.Set("user_id", userIDStr)
		c.Set("email", userObj.Email)
		c.Set("role", userObj.Role) // Real-time role from DB, not from JWT
		c.Next()
	}
}

// APIKey validates API key in header.
func (m *AuthMiddleware) APIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.GetHeader("Authorization")
			apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}

		userObj, key, err := m.userService.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("user", userObj)
		c.Set("api_key", key)
		c.Set("user_id", userObj.ID.String())
		c.Next()
	}
}

// RateLimiter provides request rate limiting backed by Redis.
type RateLimiter struct {
	redisClient       *redis.Client
	requestsPerMinute int
	logger            *zap.Logger
}

// NewRateLimiter creates a new Redis-backed rate limiter.
// If redisClient is nil, rate limiting is disabled (fail-open).
func NewRateLimiter(requestsPerMinute int, redisClient *redis.Client, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		redisClient:       redisClient,
		requestsPerMinute: requestsPerMinute,
		logger:            logger,
	}
}

// Limit applies sliding-window rate limiting per API key (or client IP as fallback).
func (r *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fail-open if Redis is not configured
		if r.redisClient == nil {
			c.Next()
			return
		}

		// Identify the client: prefer API key, fallback to IP
		identifier := c.GetString("user_id")
		if identifier == "" {
			identifier = c.ClientIP()
		}
		key := fmt.Sprintf("ratelimit:%s", identifier)

		now := time.Now()
		windowStart := now.Add(-time.Minute)
		ctx := context.Background()

		pipe := r.redisClient.Pipeline()

		// Remove entries outside the sliding window
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))

		// Count current entries in the window
		countCmd := pipe.ZCard(ctx, key)

		// Add the current request
		pipe.ZAdd(ctx, key, &redis.Z{
			Score:  float64(now.UnixNano()),
			Member: fmt.Sprintf("%d:%d", now.UnixNano(), now.Nanosecond()),
		})

		// Set expiry to auto-cleanup
		pipe.Expire(ctx, key, 2*time.Minute)

		if _, err := pipe.Exec(ctx); err != nil {
			// Fail-open: if Redis is down, allow the request through
			RateLimitFailOpenTotal.Inc() // Track for alerting
			r.logger.Warn("rate limiter redis error, allowing request",
				zap.Error(err),
				zap.String("identifier", identifier),
			)
			c.Next()
			return
		}

		currentCount := countCmd.Val()

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(r.requestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, r.requestsPerMinute-int(currentCount)-1)))

		if int(currentCount) >= r.requestsPerMinute {
			retryAfter := 60 // seconds until window resets
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.Header("X-RateLimit-Remaining", "0")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

// LoggingMiddleware provides request logging.
type LoggingMiddleware struct {
	logger *zap.Logger
}

// NewLoggingMiddleware creates a new logging middleware.
func NewLoggingMiddleware(logger *zap.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{logger: logger}
}

// Log logs request details including the request ID for correlation.
func (m *LoggingMiddleware) Log() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := sanitizeLogString(c.Request.URL.Path)
		query := sanitizeLogString(c.Request.URL.RawQuery)
		method := sanitizeLogString(c.Request.Method)
		clientIP := sanitizeLogString(c.ClientIP())

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if query != "" {
			path = path + "?" + query
		}

		// Include request_id if available (set by RequestIDMiddleware)
		requestID, _ := c.Get(RequestIDKey)
		reqIDStr, _ := requestID.(string)

		m.logger.Info("request",
			zap.String("request_id", reqIDStr),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
		)
	}
}

// CORSMiddleware handles CORS headers.
type CORSMiddleware struct {
	allowOrigins []string
}

// NewCORSMiddleware creates a new CORS middleware.
// If no origins are configured, CORS is denied by default (secure default).
func NewCORSMiddleware(origins []string) *CORSMiddleware {
	return &CORSMiddleware{allowOrigins: origins}
}

// Handle adds CORS headers.
func (m *CORSMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false

		for _, o := range m.allowOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			if m.allowOrigins[0] == "*" {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RecoveryMiddleware handles panic recovery.
type RecoveryMiddleware struct {
	logger *zap.Logger
}

// NewRecoveryMiddleware creates a new recovery middleware.
func NewRecoveryMiddleware(logger *zap.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{logger: logger}
}

// Recover handles panics gracefully.
func (m *RecoveryMiddleware) Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				m.logger.Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", sanitizeLogString(c.Request.URL.Path)),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}

// AdminOnly restricts access to admin users.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

// AuthRateLimiter limits login/register attempts per IP to prevent brute force.
type AuthRateLimiter struct {
	redisClient *redis.Client
	maxAttempts int
	logger      *zap.Logger
}

// NewAuthRateLimiter creates a rate limiter for authentication endpoints.
func NewAuthRateLimiter(redisClient *redis.Client, maxAttempts int, logger *zap.Logger) *AuthRateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &AuthRateLimiter{
		redisClient: redisClient,
		maxAttempts: maxAttempts,
		logger:      logger,
	}
}

// Limit applies per-IP rate limiting for auth endpoints (5 attempts/minute).
func (l *AuthRateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.redisClient == nil {
			c.Next()
			return
		}

		key := fmt.Sprintf("auth_ratelimit:%s", c.ClientIP())
		ctx := context.Background()

		count, err := l.redisClient.Incr(ctx, key).Result()
		if err != nil {
			l.logger.Warn("auth rate limiter redis error", zap.Error(err))
			c.Next()
			return
		}

		if count == 1 {
			l.redisClient.Expire(ctx, key, time.Minute)
		}

		if int(count) > l.maxAttempts {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "too many authentication attempts, try again later",
				"retry_after": 60,
			})
			return
		}

		c.Next()
	}
}
