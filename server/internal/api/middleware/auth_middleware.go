package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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

func (m *AuthMiddleware) JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		claims, err := m.parseTokenClaims(authHeader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		userIDStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			AuthFailuresTotal.WithLabelValues("invalid_token").Inc()
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		userObj, errCode, status, err := m.validateUserState(c.Request.Context(), userID, claims, c.Request.URL.Path)
		if err != nil {
			if errCode != "" {
				AuthFailuresTotal.WithLabelValues(errCode).Inc()
			}
			c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}

		c.Set("user_id", userIDStr)
		c.Set("email", userObj.Email)
		c.Set("role", userObj.Role)
		c.Set("user_monthly_token_limit", userObj.MonthlyTokenLimit)
		c.Set("user_monthly_budget_usd", userObj.MonthlyBudgetUSD)
		c.Set("user_rate_limit", userObj.RateLimitPerMinute)

		c.Next()
	}
}

func (m *AuthMiddleware) parseTokenClaims(authHeader string) (jwt.MapClaims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("invalid authorization format")
	}

	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

func (m *AuthMiddleware) validateUserState(ctx context.Context, userID uuid.UUID, claims jwt.MapClaims, path string) (*models.User, string, int, error) {
	userObj, err := m.userService.GetByID(ctx, userID)
	if err != nil {
		return nil, "", http.StatusUnauthorized, fmt.Errorf("user not found")
	}

	if !userObj.IsActive {
		return nil, "account_disabled", http.StatusForbidden, fmt.Errorf("account is disabled")
	}

	if userObj.RequirePasswordChange {
		if path != "/api/v1/user/password" && path != "/api/v1/user/profile" && path != "/api/v1/auth/logout" {
			return nil, "", http.StatusForbidden, fmt.Errorf("password_change_required")
		}
	}

	if !userObj.TokensInvalidatedAt.IsZero() {
		iat, _ := claims.GetIssuedAt()
		if iat != nil && iat.Before(userObj.TokensInvalidatedAt) {
			return nil, "token_revoked", http.StatusUnauthorized, fmt.Errorf("token has been revoked")
		}
	}

	return userObj, "", 0, nil
}

func (m *AuthMiddleware) OptionalJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		claims, err := m.parseTokenClaims(authHeader)
		if err != nil {
			c.Next()
			return
		}

		userIDStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.Next()
			return
		}

		userObj, err := m.userService.GetByID(c.Request.Context(), userID)
		if err != nil || !userObj.IsActive {
			c.Next()
			return
		}

		if !userObj.TokensInvalidatedAt.IsZero() {
			iat, _ := claims.GetIssuedAt()
			if iat != nil && iat.Before(userObj.TokensInvalidatedAt) {
				c.Next()
				return
			}
		}

		c.Set("user_id", userIDStr)
		c.Set("email", userObj.Email)
		c.Set("role", userObj.Role)
		c.Set("user_monthly_token_limit", userObj.MonthlyTokenLimit)
		c.Set("user_monthly_budget_usd", userObj.MonthlyBudgetUSD)
		c.Set("user_rate_limit", userObj.RateLimitPerMinute)
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

		projectObj, key, err := m.userService.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("project", projectObj)
		c.Set("api_key", key)
		c.Set("project_id", projectObj.ID.String())
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
	redisClient     *redis.Client
	maxAttempts     int
	logger          *zap.Logger
	fallbackMu      sync.Mutex
	fallbackCounter map[string]*authRateEntry
}

// authRateEntry tracks per-IP auth attempt counts for in-memory fallback.
type authRateEntry struct {
	count    int
	windowAt time.Time
}

// NewAuthRateLimiter creates a rate limiter for authentication endpoints.
func NewAuthRateLimiter(redisClient *redis.Client, maxAttempts int, logger *zap.Logger) *AuthRateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	l := &AuthRateLimiter{
		redisClient:     redisClient,
		maxAttempts:     maxAttempts,
		logger:          logger,
		fallbackCounter: make(map[string]*authRateEntry),
	}
	// M2: Start background cleanup goroutine to prevent memory leak
	go l.cleanupLoop()
	return l
}

// cleanupLoop periodically evicts expired entries from the in-memory auth rate limiter.
func (l *AuthRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.fallbackMu.Lock()
		now := time.Now()
		for key, entry := range l.fallbackCounter {
			if now.Sub(entry.windowAt) > 2*time.Minute {
				delete(l.fallbackCounter, key)
			}
		}
		l.fallbackMu.Unlock()
	}
}

// Limit applies per-IP rate limiting for auth endpoints (maxAttempts/minute).
func (l *AuthRateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if l.redisClient == nil {
			// No Redis — use in-memory fallback
			if l.checkInMemory(ip) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "too many authentication attempts, try again later",
					"retry_after": 60,
				})
				return
			}
			c.Next()
			return
		}

		key := fmt.Sprintf("auth_ratelimit:%s", ip)
		ctx := context.Background()

		count, err := l.redisClient.Incr(ctx, key).Result()
		if err != nil {
			l.logger.Warn("auth rate limiter redis error, using in-memory fallback", zap.Error(err))
			if l.checkInMemory(ip) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "too many authentication attempts, try again later",
					"retry_after": 60,
				})
				return
			}
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

// checkInMemory returns true if the IP has exceeded maxAttempts in the current 1-minute window.
func (l *AuthRateLimiter) checkInMemory(ip string) bool {
	l.fallbackMu.Lock()
	defer l.fallbackMu.Unlock()

	now := time.Now()
	entry, exists := l.fallbackCounter[ip]
	if !exists || now.Sub(entry.windowAt) > time.Minute {
		l.fallbackCounter[ip] = &authRateEntry{count: 1, windowAt: now}
		return false
	}

	entry.count++
	return entry.count > l.maxAttempts
}
