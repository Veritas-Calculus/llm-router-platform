package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"llm-router-platform/internal/models"
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
		query := sanitizeQuery(c.Request.URL.RawQuery)
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

		fields := []zap.Field{
			zap.String("request_id", reqIDStr),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.String("user_agent", sanitize.LogValue(c.Request.UserAgent())),
		}

		if traceID, ok := c.Get(TraceIDKey); ok {
			if traceIDStr, ok := traceID.(string); ok && traceIDStr != "" {
				fields = append(fields, zap.String("trace_id", sanitize.LogValue(traceIDStr)))
			}
		}
		if rawAPIKey, ok := c.Get("api_key"); ok {
			if apiKey, ok := rawAPIKey.(*models.APIKey); ok && apiKey != nil {
				fields = append(fields,
					zap.String("api_key_id", apiKey.ID.String()),
					zap.String("api_key_prefix", sanitize.LogValue(apiKey.KeyPrefix)),
				)
			}
		}
		if rawProject, ok := c.Get("project"); ok {
			if project, ok := rawProject.(*models.Project); ok && project != nil {
				fields = append(fields,
					zap.String("project_id", project.ID.String()),
					zap.String("org_id", project.OrgID.String()),
				)
			}
		}
		if modelName, ok := stringContextValue(c, "llm_model"); ok {
			fields = append(fields, zap.String("model", sanitize.LogValue(modelName)))
		}
		if providerName, ok := stringContextValue(c, "provider_name"); ok {
			fields = append(fields, zap.String("provider", sanitize.LogValue(providerName)))
		}
		if providerID, ok := stringContextValue(c, "provider_id"); ok {
			fields = append(fields, zap.String("provider_id", sanitize.LogValue(providerID)))
		}

		m.logger.Info("request", fields...)
	}
}

func stringContextValue(c *gin.Context, key string) (string, bool) {
	value, ok := c.Get(key)
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok && str != ""
}

func sanitizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return sanitize.LogValue(sanitize.RedactSecrets(rawQuery))
	}
	for key := range values {
		if isSensitiveQueryKey(key) {
			values[key] = []string{"[REDACTED]"}
		}
	}
	return sanitize.LogValue(values.Encode())
}

func isSensitiveQueryKey(key string) bool {
	switch strings.ToLower(key) {
	case "access_token", "api_key", "apikey", "auth", "authorization", "code", "id_token", "key", "password", "refresh_token", "secret", "token":
		return true
	default:
		return false
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
//
// Audit L-01: only emit the methods/headers preflight response when the
// caller's Origin is in the whitelist (or "*" is configured). Previously
// we returned the full Methods/Headers/Max-Age set even when the request
// Origin was unknown — harmless in practice (browsers still reject without
// ACAO) but it pre-stages a bypass if CORS_ORIGINS is ever flipped to "*".
// Now we keep the response minimal for non-whitelisted Origins: a 204 (for
// OPTIONS) with no CORS metadata at all. Same-origin requests have no
// Origin header and get passed through to the next handler unchanged.
func (m *CORSMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		isOptions := c.Request.Method == http.MethodOptions

		// Empty Origin means same-origin (or a non-browser caller) — let
		// the request through without echoing CORS metadata.
		if origin == "" {
			if isOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		allowed, isWildcard := m.matchOrigin(origin)
		if !allowed {
			// Unknown Origin. Don't emit any ACAO / Allow-Methods /
			// Allow-Headers; this denies the cross-origin call cleanly.
			// Vary: Origin is still useful so caches don't conflate
			// whitelisted vs non-whitelisted callers.
			c.Header("Vary", "Origin")
			if isOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		if isWildcard {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			// Vary: Origin prevents CDN/proxy cache poisoning when reflecting
			// the request Origin header into Access-Control-Allow-Origin.
			c.Header("Vary", "Origin")
		}

		// Only allow methods actually used: GraphQL (POST) and LLM API (GET, POST)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key, X-Request-Id, X-Trace-Id")
		c.Header("Access-Control-Expose-Headers", "X-Request-Id, X-Trace-Id, X-Langfuse-Trace-Id")
		c.Header("Access-Control-Max-Age", "86400")

		if isOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// matchOrigin returns (allowed, isWildcard). When isWildcard is true the
// caller should respond with `Access-Control-Allow-Origin: *` (no creds);
// otherwise reflect the exact origin and set Allow-Credentials.
func (m *CORSMiddleware) matchOrigin(origin string) (bool, bool) {
	for _, o := range m.allowOrigins {
		if o == "*" {
			return true, true
		}
		if o == origin {
			return true, false
		}
	}
	return false, false
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
