package handlers

import (
	"net/http"

	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// APIKeyHandler handles user API key endpoints.
type APIKeyHandler struct {
	userService *user.Service
	logger      *zap.Logger
}

// NewAPIKeyHandler creates a new API key handler.
func NewAPIKeyHandler(userService *user.Service, logger *zap.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		userService: userService,
		logger:      logger,
	}
}

// CreateAPIKeyRequest represents API key creation request.
type CreateAPIKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// Create godoc
// @Summary      Create API key
// @Description  Generate a new API key for the authenticated user
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Param        body body CreateAPIKeyReq true "API key details"
// @Success      201 {object} map[string]interface{}
// @Failure      400 {object} map[string]string
// @Router       /api/v1/api-keys [post]
func (h *APIKeyHandler) Create(c *gin.Context) {
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	apiKey, rawKey, err := h.userService.CreateAPIKey(c.Request.Context(), id, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           apiKey.ID,
		"name":         apiKey.Name,
		"key":          rawKey,
		"key_prefix":   apiKey.KeyPrefix,
		"is_active":    apiKey.IsActive,
		"rate_limit":   apiKey.RateLimit,
		"daily_limit":  apiKey.DailyLimit,
		"expires_at":   apiKey.ExpiresAt,
		"last_used_at": apiKey.LastUsedAt,
		"created_at":   apiKey.CreatedAt,
	})
}

// List godoc
// @Summary      List API keys
// @Description  Return all API keys for the authenticated user
// @Tags         api-keys
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Router       /api/v1/api-keys [get]
func (h *APIKeyHandler) List(c *gin.Context) {
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

	keys, err := h.userService.GetAPIKeys(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": keys})
}

// Revoke godoc
// @Summary      Revoke API key
// @Description  Deactivate an API key (does not delete it)
// @Tags         api-keys
// @Produce      json
// @Param        id path string true "API Key ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Router       /api/v1/api-keys/{id}/revoke [put]
func (h *APIKeyHandler) Revoke(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}

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

	if err := h.userService.RevokeAPIKey(c.Request.Context(), id, keyID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked"})
}

// Delete godoc
// @Summary      Delete API key
// @Description  Permanently remove an API key
// @Tags         api-keys
// @Produce      json
// @Param        id path string true "API Key ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Router       /api/v1/api-keys/{id} [delete]
func (h *APIKeyHandler) Delete(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}

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

	if err := h.userService.DeleteAPIKey(c.Request.Context(), id, keyID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}
