// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"strconv"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/health"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	healthService *health.Service
	logger        *zap.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(healthService *health.Service, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
		logger:        logger,
	}
}

// GetAPIKeysHealth returns health status of all API keys.
func (h *HealthHandler) GetAPIKeysHealth(c *gin.Context) {
	statuses, err := h.healthService.GetAPIKeysHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statuses)
}

// CheckAPIKey checks health of a specific API key.
func (h *HealthHandler) CheckAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	status, err := h.healthService.CheckSingleAPIKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetProxiesHealth returns health status of all proxies.
func (h *HealthHandler) GetProxiesHealth(c *gin.Context) {
	statuses, err := h.healthService.GetProxiesHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statuses)
}

// CheckProxy checks health of a specific proxy.
func (h *HealthHandler) CheckProxy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	status, err := h.healthService.CheckSingleProxy(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetHealthHistory returns health check history.
func (h *HealthHandler) GetHealthHistory(c *gin.Context) {
	targetType := c.Query("target_type")
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	history, err := h.healthService.GetHealthHistory(c.Request.Context(), targetType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetProvidersHealth returns health status of all active providers.
func (h *HealthHandler) GetProvidersHealth(c *gin.Context) {
	statuses, err := h.healthService.GetProvidersHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statuses)
}

// CheckProvider checks health of a specific provider.
func (h *HealthHandler) CheckProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	status, err := h.healthService.CheckSingleProvider(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// CheckAllProviders runs health checks on all active providers.
func (h *HealthHandler) CheckAllProviders(c *gin.Context) {
	if err := h.healthService.CheckAllProviders(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "health check initiated for all providers"})
}

// AlertHandler handles alert endpoints.
type AlertHandler struct {
	healthService *health.Service
	logger        *zap.Logger
}

// NewAlertHandler creates a new alert handler.
func NewAlertHandler(healthService *health.Service, logger *zap.Logger) *AlertHandler {
	return &AlertHandler{
		healthService: healthService,
		logger:        logger,
	}
}

// List returns alerts with pagination.
func (h *AlertHandler) List(c *gin.Context) {
	status := c.Query("status")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	alerts, total, err := h.healthService.GetAlerts(c.Request.Context(), status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts":    alerts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Acknowledge marks an alert as acknowledged.
func (h *AlertHandler) Acknowledge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.healthService.AcknowledgeAlert(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "alert acknowledged"})
}

// Resolve marks an alert as resolved.
func (h *AlertHandler) Resolve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.healthService.ResolveAlert(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "alert resolved"})
}

// AlertConfigRequest represents alert config update request.
type AlertConfigRequest struct {
	TargetType       string `json:"target_type" binding:"required"`
	TargetID         string `json:"target_id" binding:"required"`
	IsEnabled        bool   `json:"is_enabled"`
	FailureThreshold int    `json:"failure_threshold"`
	WebhookURL       string `json:"webhook_url"`
	Email            string `json:"email"`
}

// UpdateConfig updates alert configuration.
func (h *AlertHandler) UpdateConfig(c *gin.Context) {
	var req AlertConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
		return
	}

	config := &models.AlertConfig{
		TargetType:       req.TargetType,
		TargetID:         targetID,
		IsEnabled:        req.IsEnabled,
		FailureThreshold: req.FailureThreshold,
		WebhookURL:       req.WebhookURL,
		Email:            req.Email,
	}

	if err := h.healthService.UpdateAlertConfig(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// GetConfig returns alert configuration for a target.
func (h *AlertHandler) GetConfig(c *gin.Context) {
	targetType := c.Param("target_type")
	targetID, err := uuid.Parse(c.Param("target_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
		return
	}

	config, err := h.healthService.GetAlertConfig(c.Request.Context(), targetType, targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}
