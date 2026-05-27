package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCORSMiddlewareHandle(t *testing.T) {
	router := gin.New()
	cors := NewCORSMiddleware([]string{"*"}, "")
	router.Use(cors.Handle())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddlewareAllowsMethods(t *testing.T) {
	router := gin.New()
	cors := NewCORSMiddleware([]string{"*"}, "")
	router.Use(cors.Handle())
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)

	allowMethods := w.Header().Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowMethods, "GET")
	assert.Contains(t, allowMethods, "POST")
	assert.Contains(t, allowMethods, "OPTIONS")
	assert.NotContains(t, allowMethods, "PUT")
	assert.NotContains(t, allowMethods, "DELETE")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "X-Request-Id")
	assert.Contains(t, w.Header().Get("Access-Control-Expose-Headers"), "X-Trace-Id")
}

func TestRateLimiterCreation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(100, nil, logger)

	assert.NotNil(t, limiter)
}

func TestLoggingMiddlewareCreation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	logging := NewLoggingMiddleware(logger)

	assert.NotNil(t, logging)
}

func TestLoggingMiddlewareLog(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	logging := NewLoggingMiddleware(logger)

	router := gin.New()
	router.Use(logging.Log())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSanitizeQueryRedactsSensitiveValues(t *testing.T) {
	query := sanitizeQuery("code=oauth-code&state=ok&access_token=secret")

	assert.Contains(t, query, "code=%5BREDACTED%5D")
	assert.Contains(t, query, "access_token=%5BREDACTED%5D")
	assert.Contains(t, query, "state=ok")
	assert.NotContains(t, query, "oauth-code")
	assert.NotContains(t, query, "secret")
}

func TestRecoveryMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	recovery := NewRecoveryMiddleware(logger)

	router := gin.New()
	router.Use(recovery.Recover())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminOnly(t *testing.T) {
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

func TestAdminOnlySuccess(t *testing.T) {
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

func TestExtractToken(t *testing.T) {
	token := extractToken("Bearer valid-token-here")
	assert.Equal(t, "valid-token-here", token)

	token = extractToken("invalid-format")
	assert.Equal(t, "", token)

	token = extractToken("")
	assert.Equal(t, "", token)
}

func extractToken(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

func TestCORSWithSpecificOrigin(t *testing.T) {
	router := gin.New()
	cors := NewCORSMiddleware([]string{"http://localhost:3000"}, "")
	router.Use(cors.Handle())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// Audit L-01: preflight from a non-whitelisted Origin must NOT receive
// Allow-Methods / Allow-Headers / ACAO. Returning 403 makes the rejection
// observable to defenders without leaking the allow-list shape.
func TestCORSRejectsUnknownOriginPreflight(t *testing.T) {
	router := gin.New()
	cors := NewCORSMiddleware([]string{"http://localhost:3000"}, "")
	router.Use(cors.Handle())
	router.POST("/graphql", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/graphql", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
}

// Audit L-01: a same-origin request (no Origin header) must pass through.
// We never echo ACAO and never reject it as a preflight.
func TestCORSPassesThroughSameOrigin(t *testing.T) {
	router := gin.New()
	cors := NewCORSMiddleware([]string{"http://localhost:3000"}, "")
	router.Use(cors.Handle())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	// no Origin header set
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// Audit L-01: a non-preflight (POST) request from an unknown Origin must
// still be served (browsers will discard the response when ACAO is absent)
// but without any CORS metadata in the response.
func TestCORSAllowsBodyButOmitsHeadersForUnknownOrigin(t *testing.T) {
	router := gin.New()
	cors := NewCORSMiddleware([]string{"http://localhost:3000"}, "")
	router.Use(cors.Handle())
	router.POST("/graphql", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/graphql", nil)
	req.Header.Set("Origin", "https://evil.example")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"))
}

func TestRateLimiterMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(100, nil, logger)

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
