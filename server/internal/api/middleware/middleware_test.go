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
	cors := NewCORSMiddleware([]string{"*"})
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
	cors := NewCORSMiddleware([]string{"*"})
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
	assert.Contains(t, allowMethods, "PUT")
	assert.Contains(t, allowMethods, "DELETE")
}

func TestRateLimiterCreation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(100, logger)

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
	cors := NewCORSMiddleware([]string{"http://localhost:3000"})
	router.Use(cors.Handle())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRateLimiterMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(100, logger)

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
