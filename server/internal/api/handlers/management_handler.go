// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/router"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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

// CreateProxyRequest represents proxy creation request (input only, never serialized).
type CreateProxyRequest struct {
	URL             string `json:"url" binding:"required"`
	Type            string `json:"type" binding:"required"`
	Region          string `json:"region"`
	Username        string `json:"username"`
	Password        string `json:"password"` // #nosec G101 -- request input only
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
		Password string `json:"password"` // #nosec G101 -- request input only
	} `json:"proxies" binding:"required,min=1"`
}

// BatchCreateResult represents the result of batch proxy creation.
// Uses ProxyResponse instead of models.Proxy to ensure sensitive fields are never serialized.
type BatchCreateResult struct {
	Success int             `json:"success"`
	Failed  int             `json:"failed"`
	Proxies []ProxyResponse `json:"proxies"`
	Errors  []string        `json:"errors,omitempty"`
}

// BatchCreate creates multiple proxies at once.
func (h *ProxyHandler) BatchCreate(c *gin.Context) {
	var req BatchCreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := BatchCreateResult{
		Proxies: make([]ProxyResponse, 0),
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
			result.Proxies = append(result.Proxies, toProxyResponse(proxy))
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

// UpdateProxyRequest represents proxy update request (input only, never serialized).
type UpdateProxyRequest struct {
	URL             string `json:"url" binding:"required"`
	Type            string `json:"type" binding:"required"`
	Region          string `json:"region"`
	IsActive        bool   `json:"is_active"`
	Username        string `json:"username"`
	Password        string `json:"password"` // #nosec G101 -- request input only
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
	router        *router.Router
	healthService *health.Service
	logger        *zap.Logger
}

// NewProviderHandler creates a new provider handler.
func NewProviderHandler(r *router.Router, hs *health.Service, logger *zap.Logger) *ProviderHandler {
	return &ProviderHandler{
		router:        r,
		healthService: hs,
		logger:        logger,
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
	IsActive       *bool    `json:"is_active"`
	Priority       *int     `json:"priority"`
	Weight         *float64 `json:"weight"`
	MaxRetries     *int     `json:"max_retries"`
	Timeout        *int     `json:"timeout"`
	BaseURL        *string  `json:"base_url"`
	UseProxy       *bool    `json:"use_proxy"`
	DefaultProxyID *string  `json:"default_proxy_id"` // Use string to allow "null" for clearing
	RequiresAPIKey *bool    `json:"requires_api_key"`
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
	if req.UseProxy != nil {
		provider.UseProxy = *req.UseProxy
	}
	if req.RequiresAPIKey != nil {
		provider.RequiresAPIKey = *req.RequiresAPIKey
	}
	if req.DefaultProxyID != nil {
		if *req.DefaultProxyID == "" || *req.DefaultProxyID == "null" {
			provider.DefaultProxyID = nil
		} else {
			proxyID, err := uuid.Parse(*req.DefaultProxyID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid proxy id"})
				return
			}
			provider.DefaultProxyID = &proxyID
		}
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
	providerID := c.Param("id")

	// Try to parse as UUID first
	id, err := uuid.Parse(providerID)
	if err != nil {
		// If not a UUID, try to get provider by name
		p, err := h.router.GetProviderByName(c.Request.Context(), providerID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
			return
		}
		id = p.ID
	}

	// Use health service which handles proxy correctly
	status, err := h.healthService.CheckSingleProvider(c.Request.Context(), id)
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

// CreateProviderAPIKeyRequest represents the request to create a provider API key (input only, never serialized).
type CreateProviderAPIKeyRequest struct {
	APIKey    string   `json:"api_key" binding:"required"` // #nosec G101 -- request input only, encrypted before storage
	Alias     string   `json:"alias" binding:"required"`
	Priority  int      `json:"priority,omitempty"`
	Weight    *float64 `json:"weight,omitempty"`
	RateLimit int      `json:"rate_limit,omitempty"`
}

// UpdateProviderAPIKeyRequest represents the request to update a provider API key.
type UpdateProviderAPIKeyRequest struct {
	Priority  *int     `json:"priority,omitempty"`
	Weight    *float64 `json:"weight,omitempty"`
	RateLimit *int     `json:"rate_limit,omitempty"`
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

	// Encrypt the API key before storing
	encryptedKey, err := crypto.Encrypt(req.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt API key"})
		return
	}

	prio := req.Priority
	if prio <= 0 {
		prio = 1
	}
	weight := 1.0
	if req.Weight != nil {
		weight = *req.Weight
	}

	key := &models.ProviderAPIKey{
		ProviderID:      providerID,
		Alias:           req.Alias,
		EncryptedAPIKey: encryptedKey,
		KeyPrefix:       keyPrefix,
		IsActive:        true,
		Priority:        prio,
		Weight:          weight,
		RateLimit:       req.RateLimit,
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

// UpdateAPIKey updates a provider API key.
func (h *ProviderHandler) UpdateAPIKey(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("key_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}

	var req UpdateProviderAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, err := h.router.GetProviderAPIKeyByID(c.Request.Context(), keyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	if req.Priority != nil {
		prio := *req.Priority
		if prio <= 0 {
			prio = 1
		}
		key.Priority = prio
	}
	if req.Weight != nil {
		key.Weight = *req.Weight
	}
	if req.RateLimit != nil {
		key.RateLimit = *req.RateLimit
	}

	if err := h.router.UpdateProviderAPIKey(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, key)
}
