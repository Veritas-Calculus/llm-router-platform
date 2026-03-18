package handlers

import (
	"net/http"

	"llm-router-platform/internal/service/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ConfigHandler struct {
	configService *config.Service
	logger        *zap.Logger
}

func NewConfigHandler(s *config.Service, logger *zap.Logger) *ConfigHandler {
	return &ConfigHandler{
		configService: s,
		logger:        logger,
	}
}

func (h *ConfigHandler) GetSettings(c *gin.Context) {
	category := c.Query("category")
	settings, err := h.configService.GetByCategory(c.Request.Context(), category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": settings})
}

func (h *ConfigHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		Key         string `json:"key" binding:"required"`
		Value       string `json:"value"`
		Description string `json:"description"`
		Category    string `json:"category"`
		IsSecret    bool   `json:"is_secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.configService.Set(c.Request.Context(), req.Key, req.Value, req.Description, req.Category, req.IsSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "setting updated"})
}
