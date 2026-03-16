package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthHandlerRegisterValidation(t *testing.T) {
	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=8"`
			Name     string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"email":"test@example.com","password":"password123","name":"Test User"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid email",
			body:       `{"email":"invalid","password":"password123","name":"Test User"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "short password",
			body:       `{"email":"test@example.com","password":"short","name":"Test User"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name",
			body:       `{"email":"test@example.com","password":"password123"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestAuthHandlerLoginValidation(t *testing.T) {
	router := gin.New()
	router.POST("/login", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"email":"test@example.com","password":"password123"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing email",
			body:       `{"password":"password123"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing password",
			body:       `{"email":"test@example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestChatHandlerValidation(t *testing.T) {
	router := gin.New()
	router.POST("/chat", func(c *gin.Context) {
		var req ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request with string content",
			body:       `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid request with array content",
			body:       `{"model":"gpt-4","messages":[{"role":"user","content":[{"type":"text","text":"Hello"}]}]}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid request with mixed multimodal content",
			body:       `{"model":"gpt-4","messages":[{"role":"user","content":[{"type":"text","text":"Describe this image"},{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}]}]}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing model",
			body:       `{"messages":[{"role":"user","content":"Hello"}]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty messages",
			body:       `{"model":"gpt-4","messages":[]}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/chat", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}


func TestAPIKeyHandlerValidation(t *testing.T) {
	router := gin.New()
	router.POST("/api-keys", func(c *gin.Context) {
		var req struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"name":"My API Key"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing name",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api-keys", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestProxyHandlerValidation(t *testing.T) {
	router := gin.New()
	router.POST("/proxies", func(c *gin.Context) {
		var req struct {
			URL    string `json:"url" binding:"required,url"`
			Type   string `json:"type" binding:"required,oneof=http https socks5"`
			Region string `json:"region"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid http proxy",
			body:       `{"url":"http://proxy.example.com:8080","type":"http"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid socks5 proxy",
			body:       `{"url":"socks5://proxy.example.com:1080","type":"socks5"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid type",
			body:       `{"url":"http://proxy.example.com:8080","type":"invalid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing url",
			body:       `{"type":"http"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/proxies", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestProviderHandlerValidation(t *testing.T) {
	router := gin.New()
	router.POST("/providers", func(c *gin.Context) {
		var req struct {
			Name    string `json:"name" binding:"required"`
			BaseURL string `json:"base_url" binding:"required,url"`
			APIKey  string `json:"api_key" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"name":"openai","base_url":"https://api.openai.com/v1","api_key":"sk-xxx"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing name",
			body:       `{"base_url":"https://api.openai.com/v1","api_key":"sk-xxx"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid url",
			body:       `{"name":"openai","base_url":"invalid","api_key":"sk-xxx"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/providers", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestTTSHandlerValidation(t *testing.T) {
	router := gin.New()
	router.POST("/speech", func(c *gin.Context) {
		var req SpeechSynthesisRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"model":"tts-1","input":"Hello world","voice":"alloy"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid request with optional fields",
			body:       `{"model":"tts-1-hd","input":"Hello world","voice":"nova","response_format":"opus","speed":1.5}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing model",
			body:       `{"input":"Hello world","voice":"alloy"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing input",
			body:       `{"model":"tts-1","voice":"alloy"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing voice",
			body:       `{"model":"tts-1","input":"Hello world"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/speech", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestTaskHandlerValidation(t *testing.T) {
	router := gin.New()
	router.POST("/tasks", func(c *gin.Context) {
		var req CreateTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid task request",
			body:       `{"type":"tts","input":"{\"text\":\"hello\"}"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid batch_tts request",
			body:       `{"type":"batch_tts","input":"[{\"text\":\"hello\"},{\"text\":\"world\"}]","webhook_url":"https://example.com/callback"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing type",
			body:       `{"input":"{\"text\":\"hello\"}"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing input",
			body:       `{"type":"tts"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/tasks", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
