// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/router"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DashboardHandler handles dashboard endpoints.
type DashboardHandler struct {
	billing *billing.Service
	health  *health.Service
	router  *router.Router
	logger  *zap.Logger
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(billing *billing.Service, health *health.Service, routerSvc *router.Router, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		billing: billing,
		health:  health,
		router:  routerSvc,
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
	// Verify user is authenticated (but we show system-wide stats)
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get system-wide usage stats for this month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	monthlySummary, err := h.billing.GetSystemUsageSummary(c.Request.Context(), monthStart, now)
	if err != nil {
		h.logger.Error("failed to get monthly usage", zap.Error(err))
		monthlySummary = &billing.UsageSummary{}
	}

	todaySummary, err := h.billing.GetSystemUsageSummary(c.Request.Context(), todayStart, now)
	if err != nil {
		h.logger.Error("failed to get today usage", zap.Error(err))
		todaySummary = &billing.UsageSummary{}
	}

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

	// Count active providers from database
	activeProviders := 0
	if providers, err := h.router.GetAllProviders(c.Request.Context()); err == nil {
		for _, p := range providers {
			if p.IsActive {
				activeProviders++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_requests":     monthlySummary.TotalRequests,
		"success_rate":       monthlySummary.SuccessRate,
		"total_tokens":       monthlySummary.TotalTokens,
		"total_cost":         monthlySummary.TotalCost,
		"average_latency_ms": monthlySummary.AvgLatency,
		"active_users":       1,
		"active_providers":   activeProviders,
		"active_proxies":     healthyProxies,
		"requests_today":     todaySummary.TotalRequests,
		"cost_today":         todaySummary.TotalCost,
		"tokens_today":       todaySummary.TotalTokens,
		"error_count":        monthlySummary.ErrorCount,
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

// GetUsageChart returns usage chart data for the last 7 days.
func (h *DashboardHandler) GetUsageChart(c *gin.Context) {
	// Verify user is authenticated (but we show system-wide stats)
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get system-wide daily usage for last 7 days
	dailyUsage, err := h.billing.GetSystemDailyUsage(c.Request.Context(), 7)
	if err != nil {
		h.logger.Error("failed to get daily usage", zap.Error(err))
	}

	// Create map for quick lookup
	usageMap := make(map[string]*billing.DailyUsage)
	for i := range dailyUsage {
		usageMap[dailyUsage[i].Date] = &dailyUsage[i]
	}

	// Build response with all 7 days
	var data []gin.H
	now := time.Now()
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")

		if usage, ok := usageMap[dateStr]; ok {
			data = append(data, gin.H{
				"date":     dateStr,
				"requests": usage.Requests,
				"tokens":   usage.Tokens,
				"cost":     usage.Cost,
			})
		} else {
			data = append(data, gin.H{
				"date":     dateStr,
				"requests": 0,
				"tokens":   0,
				"cost":     0.0,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GetProviderStats returns statistics for each provider.
func (h *DashboardHandler) GetProviderStats(c *gin.Context) {
	// Verify user is authenticated (but we show system-wide stats)
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get system-wide usage by provider for the last 30 days
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	providerUsage, err := h.billing.GetSystemUsageByProvider(c.Request.Context(), startTime, now)
	if err != nil {
		h.logger.Error("failed to get provider usage", zap.Error(err))
	}

	// Build stats with provider names from router
	var stats []gin.H
	for _, usage := range providerUsage {
		providerName := "Unknown"
		if provider, err := h.router.GetProviderByID(c.Request.Context(), usage.ProviderID); err == nil {
			providerName = provider.Name
		}

		stats = append(stats, gin.H{
			"provider_id":    usage.ProviderID.String(),
			"provider_name":  providerName,
			"requests":       usage.Requests,
			"tokens":         usage.Tokens,
			"success_rate":   0.0,
			"avg_latency_ms": 0,
			"total_cost":     usage.Cost,
		})
	}

	// If no usage data, get all providers
	if len(stats) == 0 {
		providers, err := h.router.GetAllProviders(c.Request.Context())
		if err == nil {
			for _, provider := range providers {
				if provider.IsActive {
					stats = append(stats, gin.H{
						"provider_id":    provider.ID.String(),
						"provider_name":  provider.Name,
						"requests":       0,
						"tokens":         0,
						"success_rate":   0.0,
						"avg_latency_ms": 0,
						"total_cost":     0.0,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetModelStats returns statistics for each model.
func (h *DashboardHandler) GetModelStats(c *gin.Context) {
	// Verify user is authenticated (but we show system-wide stats)
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get system-wide usage by model for the last 30 days
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	modelUsage, err := h.billing.GetSystemUsageByModel(c.Request.Context(), startTime, now)
	if err != nil {
		h.logger.Error("failed to get model usage", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
		return
	}

	// Build stats using model name directly from usage data
	var stats []gin.H
	for _, usage := range modelUsage {
		modelName := usage.ModelName
		if modelName == "" {
			modelName = "Unknown"
		}

		stats = append(stats, gin.H{
			"model_id":      usage.ModelID.String(),
			"model_name":    modelName,
			"requests":      usage.Requests,
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"total_cost":    usage.Cost,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
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

// ProxyResponse represents a proxy in API responses.
type ProxyResponse struct {
	ID              string  `json:"id"`
	URL             string  `json:"url"`
	Type            string  `json:"type"`
	Username        string  `json:"username,omitempty"`
	HasAuth         bool    `json:"has_auth"`
	Region          string  `json:"region"`
	UpstreamProxyID string  `json:"upstream_proxy_id,omitempty"`
	IsActive        bool    `json:"is_active"`
	Weight          float64 `json:"weight"`
	SuccessCount    int64   `json:"success_count"`
	FailureCount    int64   `json:"failure_count"`
	AvgLatency      float64 `json:"avg_latency"`
	LastChecked     string  `json:"last_checked"`
	CreatedAt       string  `json:"created_at"`
}

// toProxyResponse converts a Proxy model to ProxyResponse.
func toProxyResponse(p *models.Proxy) ProxyResponse {
	upstreamID := ""
	if p.UpstreamProxyID != nil {
		upstreamID = p.UpstreamProxyID.String()
	}
	return ProxyResponse{
		ID:              p.ID.String(),
		URL:             p.URL,
		Type:            p.Type,
		Username:        p.Username,
		HasAuth:         p.HasAuth(),
		Region:          p.Region,
		UpstreamProxyID: upstreamID,
		IsActive:        p.IsActive,
		Weight:          p.Weight,
		SuccessCount:    p.SuccessCount,
		FailureCount:    p.FailureCount,
		AvgLatency:      p.AvgLatency,
		LastChecked:     p.LastChecked.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:       p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// List returns all proxies.
func (h *ProxyHandler) List(c *gin.Context) {
	proxies, err := h.proxyService.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]ProxyResponse, len(proxies))
	for i, p := range proxies {
		responses[i] = toProxyResponse(&p)
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// CreateProxyRequest represents proxy creation request.
type CreateProxyRequest struct {
	URL             string `json:"url" binding:"required"`
	Type            string `json:"type" binding:"required"`
	Region          string `json:"region"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	UpstreamProxyID string `json:"upstream_proxy_id"`
}

// Create creates a new proxy.
func (h *ProxyHandler) Create(c *gin.Context) {
	var req CreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var upstreamID *uuid.UUID
	if req.UpstreamProxyID != "" {
		id, err := uuid.Parse(req.UpstreamProxyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upstream_proxy_id"})
			return
		}
		upstreamID = &id
	}

	proxy, err := h.proxyService.Create(c.Request.Context(), req.URL, req.Type, req.Region, req.Username, req.Password, upstreamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toProxyResponse(proxy))
}

// BatchCreateProxyRequest represents batch proxy creation request.
type BatchCreateProxyRequest struct {
	Proxies []struct {
		URL      string `json:"url" binding:"required"`
		Type     string `json:"type"`
		Region   string `json:"region"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"proxies" binding:"required,min=1"`
}

// BatchCreateResult represents the result of batch proxy creation.
type BatchCreateResult struct {
	Success int            `json:"success"`
	Failed  int            `json:"failed"`
	Proxies []models.Proxy `json:"proxies"`
	Errors  []string       `json:"errors,omitempty"`
}

// BatchCreate creates multiple proxies at once.
func (h *ProxyHandler) BatchCreate(c *gin.Context) {
	var req BatchCreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := BatchCreateResult{
		Proxies: make([]models.Proxy, 0),
		Errors:  make([]string, 0),
	}

	for _, p := range req.Proxies {
		proxyType := p.Type
		if proxyType == "" {
			proxyType = "http" // default type
		}

		proxy, err := h.proxyService.Create(c.Request.Context(), p.URL, proxyType, p.Region, p.Username, p.Password, nil)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, p.URL+": "+err.Error())
		} else {
			result.Success++
			result.Proxies = append(result.Proxies, *proxy)
		}
	}

	c.JSON(http.StatusCreated, result)
}

// TestProxy tests a proxy's connectivity and returns the result.
func (h *ProxyHandler) TestProxy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	healthy, latency, testErr := h.proxyService.CheckHealth(c.Request.Context(), id)

	// Get the proxy for full info
	proxy, err := h.proxyService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"id":         proxy.ID,
		"url":        proxy.URL,
		"is_healthy": healthy,
		"latency_ms": latency.Milliseconds(),
		"tested_at":  proxy.LastChecked,
	}

	if testErr != nil {
		response["error"] = testErr.Error()
	}

	c.JSON(http.StatusOK, response)
}

// TestAllProxies tests all proxies and returns results.
func (h *ProxyHandler) TestAllProxies(c *gin.Context) {
	proxies, err := h.proxyService.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]gin.H, 0, len(proxies))
	for _, proxy := range proxies {
		healthy, latency, testErr := h.proxyService.CheckHealth(c.Request.Context(), proxy.ID)

		result := gin.H{
			"id":         proxy.ID,
			"url":        proxy.URL,
			"is_healthy": healthy,
			"latency_ms": latency.Milliseconds(),
		}

		if testErr != nil {
			result["error"] = testErr.Error()
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// UpdateProxyRequest represents proxy update request.
type UpdateProxyRequest struct {
	URL             string `json:"url" binding:"required"`
	Type            string `json:"type" binding:"required"`
	Region          string `json:"region"`
	IsActive        bool   `json:"is_active"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	UpstreamProxyID string `json:"upstream_proxy_id"`
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

	var upstreamID *uuid.UUID
	if req.UpstreamProxyID != "" {
		uid, err := uuid.Parse(req.UpstreamProxyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upstream_proxy_id"})
			return
		}
		// Prevent circular reference
		if uid == id {
			c.JSON(http.StatusBadRequest, gin.H{"error": "proxy cannot be its own upstream"})
			return
		}
		upstreamID = &uid
	}

	proxy, err := h.proxyService.Update(c.Request.Context(), id, req.URL, req.Type, req.Region, req.IsActive, req.Username, req.Password, upstreamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toProxyResponse(proxy))
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

	proxy, err := h.proxyService.Toggle(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toProxyResponse(proxy))
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
	providers, err := h.router.GetAllProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": providers})
}

// UpdateProviderRequest represents the request to update a provider.
type UpdateProviderRequest struct {
	IsActive   *bool    `json:"is_active"`
	Priority   *int     `json:"priority"`
	Weight     *float64 `json:"weight"`
	MaxRetries *int     `json:"max_retries"`
	Timeout    *int     `json:"timeout"`
	BaseURL    *string  `json:"base_url"`
}

// Update updates a provider.
func (h *ProviderHandler) Update(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider, err := h.router.GetProviderByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	if req.IsActive != nil {
		provider.IsActive = *req.IsActive
	}
	if req.Priority != nil {
		provider.Priority = *req.Priority
	}
	if req.Weight != nil {
		provider.Weight = *req.Weight
	}
	if req.MaxRetries != nil {
		provider.MaxRetries = *req.MaxRetries
	}
	if req.Timeout != nil {
		provider.Timeout = *req.Timeout
	}
	if req.BaseURL != nil {
		provider.BaseURL = *req.BaseURL
	}

	if err := h.router.UpdateProvider(c.Request.Context(), provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// Toggle enables or disables a provider.
func (h *ProviderHandler) Toggle(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	provider, err := h.router.GetProviderByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	provider.IsActive = !provider.IsActive
	if err := h.router.UpdateProvider(c.Request.Context(), provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// ToggleProxy enables or disables proxy usage for a provider.
func (h *ProviderHandler) ToggleProxy(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	provider, err := h.router.GetProviderByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	provider.UseProxy = !provider.UseProxy
	if err := h.router.UpdateProvider(c.Request.Context(), provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
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

// GetAPIKeys returns all API keys for a provider.
func (h *ProviderHandler) GetAPIKeys(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	keys, err := h.router.GetAllProviderAPIKeys(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": keys})
}

// ToggleAPIKey toggles a provider API key's active status.
func (h *ProviderHandler) ToggleAPIKey(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("key_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}

	key, err := h.router.ToggleProviderAPIKey(c.Request.Context(), keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, key)
}

// CreateProviderAPIKeyRequest represents the request to create a provider API key.
type CreateProviderAPIKeyRequest struct {
	APIKey string `json:"api_key" binding:"required"`
	Alias  string `json:"alias" binding:"required"`
}

// CreateAPIKey creates a new API key for a provider.
func (h *ProviderHandler) CreateAPIKey(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	var req CreateProviderAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store only the first few characters as prefix for display
	keyPrefix := req.APIKey
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8] + "..."
	}

	key := &models.ProviderAPIKey{
		ProviderID:      providerID,
		Alias:           req.Alias,
		EncryptedAPIKey: req.APIKey, // In production, this should be encrypted
		KeyPrefix:       keyPrefix,
		IsActive:        true,
		Weight:          1.0,
	}

	if err := h.router.CreateProviderAPIKey(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, key)
}

// DeleteAPIKey deletes a provider API key.
func (h *ProviderHandler) DeleteAPIKey(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("key_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}

	if err := h.router.DeleteProviderAPIKey(c.Request.Context(), keyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}
