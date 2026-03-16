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

func TestSecurityHeadersApplied(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	_ = logger

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Contains(t, w.Header().Get("Permissions-Policy"), "camera=()")
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
