package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestParseWhitelist(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name      string
		whitelist string
		expected  int
	}{
		{"Empty string", "", 0},
		{"Single IP", "192.168.1.1", 1},
		{"Multiple IPs and CIDRs", "192.168.1.1, 10.0.0.0/8,  invalid_ip, 2001:db8::/32", 3},
		{"Whitespace handling", " 192.168.1.1 , 10.0.0.0/8 ", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.ParseWhitelist(tt.whitelist, logger)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestCheckIPAllowed(t *testing.T) {
	logger := zap.NewNop()
	subnets := middleware.ParseWhitelist("192.168.1.1, 10.0.0.0/8", logger)

	assert.True(t, middleware.CheckIPAllowed("192.168.1.1", subnets, logger))
	assert.True(t, middleware.CheckIPAllowed("10.5.0.1", subnets, logger))
	assert.False(t, middleware.CheckIPAllowed("192.168.1.2", subnets, logger))
	assert.False(t, middleware.CheckIPAllowed("invalid_ip", subnets, logger))
}

func TestAdminIPWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	// Test 1: Empty whitelist allows everything
	router := gin.New()
	router.Use(middleware.AdminIPWhitelist("", logger))
	router.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test 2: Populated whitelist allows matching IP
	router2 := gin.New()
	router2.Use(middleware.AdminIPWhitelist("192.168.1.1", logger))
	router2.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	router2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Test 3: Populated whitelist denies non-matching IP
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "10.0.0.1:1234"
	w3 := httptest.NewRecorder()
	router2.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusForbidden, w3.Code)
}

func TestTenantAPIKeyWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	router := gin.New()
	
	// Mock the AuthMiddleware.APIKey logic inject
	router.Use(func(c *gin.Context) {
		whitelist := c.GetHeader("X-Test-Whitelist")
		
		project := &models.Project{
			BaseModel:      models.BaseModel{ID: uuid.New()},
			Name:           "Test Project",
			WhiteListedIps: whitelist,
		}
		
		if c.GetHeader("X-Test-No-Project") == "" {
			c.Set("project", project)
		}
		c.Next()
	})
	
	router.Use(middleware.TenantAPIKeyWhitelist(logger))
	router.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Test 1: No project injected (falls through)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Test-No-Project", "true")
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test 2: Project injected, empty whitelist (allowed)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Test-Whitelist", "")
	req2.RemoteAddr = "1.2.3.4:1234"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Test 3: Project with whitelist, matching IP
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-Test-Whitelist", "192.168.1.0/24, 10.0.0.1")
	req3.RemoteAddr = "192.168.1.100:1234"
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	// Test 4: Project with whitelist, non-matching IP
	req4 := httptest.NewRequest("GET", "/test", nil)
	req4.Header.Set("X-Test-Whitelist", "192.168.1.0/24")
	req4.RemoteAddr = "10.0.0.10:1234"
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusForbidden, w4.Code)
}
