package handlers

import (
	"net/http"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/router"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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

// ─── CRUD Operations ────────────────────────────────────────────────────

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

// ─── Toggle Operations ──────────────────────────────────────────────────

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

// ─── Health Check ───────────────────────────────────────────────────────

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

// ─── Provider API Keys ──────────────────────────────────────────────────

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
