package middleware

import (
	"net/http"
	"time"

	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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
		path := sanitize.LogValue(c.Request.URL.Path)
		query := sanitize.LogValue(c.Request.URL.RawQuery)
		method := sanitize.LogValue(c.Request.Method)
		clientIP := sanitize.LogValue(c.ClientIP())

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
// In release mode, wildcard "*" origins are rejected to prevent misconfiguration.
func NewCORSMiddleware(origins []string, mode string) *CORSMiddleware {
	if mode == "release" {
		filtered := make([]string, 0, len(origins))
		for _, o := range origins {
			if o != "*" {
				filtered = append(filtered, o)
			}
			// Silently drop "*" in release mode — forces explicit origin config
		}
		return &CORSMiddleware{allowOrigins: filtered}
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
				c.Header("Access-Control-Allow-Credentials", "true")
				// Vary: Origin prevents CDN/proxy cache poisoning when reflecting
				// the request Origin header into Access-Control-Allow-Origin.
				c.Header("Vary", "Origin")
			}
		}

		// Only allow methods actually used: GraphQL (POST) and LLM API (GET, POST)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
					zap.String("path", sanitize.LogValue(c.Request.URL.Path)),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
