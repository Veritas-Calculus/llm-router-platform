// Package middleware provides HTTP middleware functions.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

// JWT validates JWT token in Authorization header.
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

		c.Set("user_id", claims["sub"])
		c.Set("email", claims["email"])
		c.Set("role", claims["role"])
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

// RateLimiter provides request rate limiting.
type RateLimiter struct {
	requestsPerMinute int
	windowDuration    time.Duration
	logger            *zap.Logger
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(requestsPerMinute int, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		requestsPerMinute: requestsPerMinute,
		windowDuration:    time.Minute,
		logger:            logger,
	}
}

// Limit applies rate limiting per API key.
func (r *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
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

// Log logs request details.
func (m *LoggingMiddleware) Log() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if query != "" {
			path = path + "?" + query
		}

		m.logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// CORSMiddleware handles CORS headers.
type CORSMiddleware struct {
	allowOrigins []string
}

// NewCORSMiddleware creates a new CORS middleware.
func NewCORSMiddleware(origins []string) *CORSMiddleware {
	if len(origins) == 0 {
		origins = []string{"*"}
	}
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
					zap.String("path", c.Request.URL.Path),
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
