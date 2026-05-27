package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// ─── Security Header Tests ──────────────────────────────────────────────

// SecurityHeaders is now intentionally a *minimal* defense-in-depth set —
// nginx owns CSP / HSTS / X-Frame-Options / Referrer-Policy / X-XSS-Protection
// / Permissions-Policy at the edge (see web/snippets/security-headers.conf).
// These tests pin the Go middleware's surface to exactly that minimum so
// future drift surfaces in CI rather than as duplicated response headers in
// production.

func TestSecurityHeadersOnAPIPath(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	_ = logger

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/v1/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Always-on defense-in-depth.
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	// Cache-Control on non-health API responses.
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
	assert.Equal(t, "0", w.Header().Get("Expires"))

	// Headers that are nginx's sole responsibility — these MUST NOT be set
	// by the Go middleware anymore; setting them here was the cause of
	// audit findings H-01 / L-07 (duplicate / conflicting headers).
	assert.Empty(t, w.Header().Get("Content-Security-Policy"))
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	assert.Empty(t, w.Header().Get("X-Frame-Options"))
	assert.Empty(t, w.Header().Get("Referrer-Policy"))
	assert.Empty(t, w.Header().Get("X-XSS-Protection"))
	assert.Empty(t, w.Header().Get("Permissions-Policy"))
}

func TestSecurityHeadersSkipsCacheControlOnHealth(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.GET("/readyz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for _, path := range []string{"/healthz", "/readyz", "/health"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", path, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "path %s", path)
		// nosniff always.
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"), "path %s", path)
		// Cache-Control / Pragma / Expires must be empty on health probes —
		// nginx owns those headers for liveness/readiness paths so LB
		// caching policy is decided in exactly one place.
		assert.Empty(t, w.Header().Get("Cache-Control"), "path %s", path)
		assert.Empty(t, w.Header().Get("Pragma"), "path %s", path)
		assert.Empty(t, w.Header().Get("Expires"), "path %s", path)
	}
}

// ─── Body Size Limit Tests ──────────────────────────────────────────────

func TestBodySizeLimitRejectsLargePayload(t *testing.T) {
	router := gin.New()
	router.Use(BodySizeLimit(100)) // 100 bytes
	router.POST("/test", func(c *gin.Context) {
		// Must read body for MaxBytesReader to trigger
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "too large")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	// Create a request with body larger than 100 bytes
	largeBody := make([]byte, 200)
	for i := range largeBody {
		largeBody[i] = 'A'
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(largeBody))
	req.ContentLength = 200
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code, "large payload should be rejected")
}

func TestBodySizeLimitAllowsSmallPayload(t *testing.T) {
	router := gin.New()
	router.Use(BodySizeLimit(1000)) // 1KB
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.ContentLength = 50
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── AdminOnly Auth Tests ───────────────────────────────────────────────

func TestAdminOnlyBlocksUnauthenticated(t *testing.T) {
	router := gin.New()
	// No role set = unauthenticated
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminOnlyBlocksRegularUser(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminOnlyAllowsAdmin(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── Per-Key Rate Limiter Fallback Tests ────────────────────────────────

func TestPerKeyRateLimiterNoRedis(t *testing.T) {
	// Without Redis, should still create successfully and allow requests
	logger, _ := zap.NewDevelopment()
	limiter := NewPerKeyRateLimiter(nil, logger)
	assert.NotNil(t, limiter)

	router := gin.New()
	router.Use(limiter.Limit())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPerUserRateLimiterNoRedis(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewPerUserRateLimiter(nil, 60, logger)
	assert.NotNil(t, limiter)

	router := gin.New()
	router.Use(limiter.Limit())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── Rate Limiter In-Memory Fallback Tests ──────────────────────────────

func TestPerKeyFallbackCheckLimits(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewPerKeyRateLimiter(nil, logger)

	// First request should not be exceeded
	exceeded, count := limiter.fallbackCheck("test-key", 3, 60000000000)
	assert.False(t, exceeded)
	assert.Equal(t, int64(1), count)

	// Next two should also pass
	exceeded, _ = limiter.fallbackCheck("test-key", 3, 60000000000)
	assert.False(t, exceeded)
	exceeded, _ = limiter.fallbackCheck("test-key", 3, 60000000000)
	assert.False(t, exceeded)

	// Fourth should exceed
	exceeded, _ = limiter.fallbackCheck("test-key", 3, 60000000000)
	assert.True(t, exceeded)
}
