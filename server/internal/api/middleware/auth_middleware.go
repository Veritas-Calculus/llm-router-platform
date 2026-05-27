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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"llm-router-platform/pkg/jwtsign"
)

// AccessTokenCookieName is the name of the HttpOnly cookie carrying the
// access JWT. C-02: keeping the access token out of localStorage means a
// single XSS no longer translates into a session takeover.
//
// Kept in sync with server/internal/graphql/resolvers/helpers_auth.go
// (accessTokenCookieName) — both names point at the same cookie.
const AccessTokenCookieName = "llm_router_access"

// AuthMiddleware handles JWT authentication.
type AuthMiddleware struct {
	jwtSecret   []byte
	signer      *jwtsign.Signer
	userService *user.Service
	logger      *zap.Logger
}

// NewAuthMiddleware creates a new auth middleware.
//
// Uses jwtsign.Signer when JWT.Algorithm selects RS256/EdDSA; otherwise
// keeps the legacy HS256+secret behavior so existing deployments don't
// need any config change to upgrade.
func NewAuthMiddleware(cfg *config.JWTConfig, userService *user.Service, logger *zap.Logger) *AuthMiddleware {
	m := &AuthMiddleware{
		jwtSecret:   []byte(cfg.Secret),
		userService: userService,
		logger:      logger,
	}
	if signer, err := BuildJWTSigner(*cfg); err == nil {
		m.signer = signer
	} else {
		// Don't fail boot — the legacy path still works as a fallback.
		// BuildJWTSigner only errors when an explicit non-HS256 algorithm
		// is mis-configured (missing keys), which we want surfaced loudly.
		logger.Error("JWT signer build failed; falling back to HS256 secret only",
			zap.Error(err))
	}
	return m
}

func (m *AuthMiddleware) JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, ok := extractAccessToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		claims, err := m.parseRawToken(rawToken)
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
		if iat, _ := claims.GetIssuedAt(); iat != nil {
			c.Set("token_iat", iat.Time)
		}

		c.Next()
	}
}

// extractAccessToken returns the raw JWT from the request. It prefers the
// Authorization: Bearer header (so non-browser API clients keep working
// unchanged) and falls back to the llm_router_access HttpOnly cookie
// (browser SPA path after C-02).
func extractAccessToken(c *gin.Context) (string, bool) {
	if h := c.GetHeader("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[1]), true
		}
		// Authorization header present but malformed — refuse rather than
		// silently fall through to cookie. This matches the previous
		// behaviour and avoids confusing the caller.
		return "", false
	}
	if cookie, err := c.Cookie(AccessTokenCookieName); err == nil && strings.TrimSpace(cookie) != "" {
		return strings.TrimSpace(cookie), true
	}
	return "", false
}

// parseRawToken validates a raw JWT (no "Bearer " prefix) and returns its
// claims. Replaces the legacy parseTokenClaims helper which required the
// Authorization header format.
func (m *AuthMiddleware) parseRawToken(raw string) (jwt.MapClaims, error) {
	// Prefer the configured signer (handles HS256 / RS256 / EdDSA + key
	// rotation via kid). Fall back to direct HMAC parsing if no signer was
	// configured — keeps the legacy path working when the build helper
	// errored.
	if m.signer != nil {
		claims := jwt.MapClaims{}
		token, err := m.signer.Parse(raw, &claims)
		if err != nil || !token.Valid {
			return nil, fmt.Errorf("invalid token")
		}
		return claims, nil
	}

	token, err := jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) {
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
		rawToken, ok := extractAccessToken(c)
		if !ok {
			c.Next()
			return
		}

		claims, err := m.parseRawToken(rawToken)
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
		if iat, _ := claims.GetIssuedAt(); iat != nil {
			c.Set("token_iat", iat.Time)
		}
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

		if strings.Count(apiKey, ".") == 2 {
			projectObj, key, err := m.validatePlaygroundToken(c.Request.Context(), apiKey)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			c.Set("project", projectObj)
			c.Set("api_key", key)
			c.Set("project_id", projectObj.ID.String())
			c.Next()
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

func (m *AuthMiddleware) validatePlaygroundToken(ctx context.Context, tokenStr string) (*models.Project, *models.APIKey, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "playground_api_key" {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	sub, _ := claims["sub"].(string)
	if _, err := uuid.Parse(sub); err != nil {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	keyIDStr, _ := claims["api_key_id"].(string)
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	projectObj, key, err := m.userService.ValidateAPIKeyByID(ctx, keyID)
	if err != nil {
		return nil, nil, err
	}

	if projectID, _ := claims["project_id"].(string); projectID != "" && projectID != key.ProjectID.String() {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	if err := m.userService.RequireProjectRole(ctx, sub, key.ProjectID.String(), "OWNER", "ADMIN", "MEMBER"); err != nil {
		return nil, nil, err
	}

	return projectObj, key, nil
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
		ctx, cancel := context.WithTimeout(c.Request.Context(), rateLimitRedisTimeout)
		defer cancel()

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
