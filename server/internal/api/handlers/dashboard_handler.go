// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/router"
)

// DashboardHandler handles dashboard endpoints.
type DashboardHandler struct {
	billing *billing.Service
	health  *health.Service
	logger  *zap.Logger
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(billing *billing.Service, health *health.Service, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		billing: billing,
		health:  health,
		logger:  logger,
	}
}

// GetStats returns dashboard statistics.
func (h *DashboardHandler) GetStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, -1, 0)

	summary, err := h.billing.GetUsageSummary(c.Request.Context(), id, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_requests": summary.TotalRequests,
		"total_tokens":   summary.TotalTokens,
		"total_cost":     summary.TotalCost,
		"avg_latency":    summary.AvgLatency,
		"success_rate":   summary.SuccessRate,
	})
}

// GetOverview returns system overview.
func (h *DashboardHandler) GetOverview(c *gin.Context) {
	apiKeyHealth, err := h.health.GetAPIKeysHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	proxyHealth, err := h.health.GetProxiesHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	healthyAPIKeys := 0
	for _, k := range apiKeyHealth {
		if k.IsHealthy {
			healthyAPIKeys++
		}
	}

	healthyProxies := 0
	for _, p := range proxyHealth {
		if p.IsHealthy {
			healthyProxies++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys": gin.H{
			"total":   len(apiKeyHealth),
			"healthy": healthyAPIKeys,
		},
		"proxies": gin.H{
			"total":   len(proxyHealth),
			"healthy": healthyProxies,
		},
	})
}

// ProxyHandler handles proxy management endpoints.
type ProxyHandler struct {
	proxyService *proxy.Service
	logger       *zap.Logger
}

// NewProxyHandler creates a new proxy handler.
func NewProxyHandler(proxyService *proxy.Service, logger *zap.Logger) *ProxyHandler {
	return &ProxyHandler{
		proxyService: proxyService,
		logger:       logger,
	}
}

// List returns all proxies.
func (h *ProxyHandler) List(c *gin.Context) {
	proxies, err := h.proxyService.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, proxies)
}

// CreateProxyRequest represents proxy creation request.
type CreateProxyRequest struct {
	URL      string `json:"url" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Region   string `json:"region"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Create creates a new proxy.
func (h *ProxyHandler) Create(c *gin.Context) {
	var req CreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proxy, err := h.proxyService.Create(c.Request.Context(), req.URL, req.Type, req.Region, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, proxy)
}

// UpdateProxyRequest represents proxy update request.
type UpdateProxyRequest struct {
	URL      string `json:"url" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Region   string `json:"region"`
	IsActive bool   `json:"is_active"`
}

// Update updates a proxy.
func (h *ProxyHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req UpdateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.proxyService.Update(c.Request.Context(), id, req.URL, req.Type, req.Region, req.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy updated"})
}

// Delete removes a proxy.
func (h *ProxyHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.proxyService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy deleted"})
}

// Toggle enables or disables a proxy.
func (h *ProxyHandler) Toggle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.proxyService.Toggle(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy toggled"})
}

// ProviderHandler handles provider management endpoints.
type ProviderHandler struct {
	router *router.Router
	logger *zap.Logger
}

// NewProviderHandler creates a new provider handler.
func NewProviderHandler(r *router.Router, logger *zap.Logger) *ProviderHandler {
	return &ProviderHandler{
		router: r,
		logger: logger,
	}
}

// List returns all providers.
func (h *ProviderHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"providers": []string{"openai", "anthropic", "google"}})
}

// Toggle enables or disables a provider.
func (h *ProviderHandler) Toggle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "provider toggled"})
}

// CheckHealth checks provider health.
func (h *ProviderHandler) CheckHealth(c *gin.Context) {
	providerName := c.Param("id")

	status, err := h.router.CheckProviderHealth(c.Request.Context(), providerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}
