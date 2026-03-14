// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"time"

	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DashboardHandler handles dashboard endpoints.
type DashboardHandler struct {
	billing     *billing.Service
	health      *health.Service
	router      *router.Router
	userService *user.Service
	logger      *zap.Logger
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(billing *billing.Service, health *health.Service, routerSvc *router.Router, userService *user.Service, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		billing:     billing,
		health:      health,
		router:      routerSvc,
		userService: userService,
		logger:      logger,
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

// GetOverview returns overview stats, role-aware.
// Admin: system-wide stats. User: personal stats.
func (h *DashboardHandler) GetOverview(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	role, _ := c.Get("role")
	isAdmin := role == "admin"

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var monthlySummary, todaySummary *billing.UsageSummary
	var err error

	if isAdmin {
		// Admin sees system-wide stats
		monthlySummary, err = h.billing.GetSystemUsageSummary(c.Request.Context(), monthStart, now)
		if err != nil {
			h.logger.Error("failed to get monthly usage", zap.Error(err))
			monthlySummary = &billing.UsageSummary{}
		}

		todaySummary, err = h.billing.GetSystemUsageSummary(c.Request.Context(), todayStart, now)
		if err != nil {
			h.logger.Error("failed to get today usage", zap.Error(err))
			todaySummary = &billing.UsageSummary{}
		}
	} else {
		// Regular user sees their own stats
		id, _ := uuid.Parse(userIDStr.(string))
		monthlySummary, err = h.billing.GetUsageSummary(c.Request.Context(), id, monthStart, now)
		if err != nil {
			h.logger.Error("failed to get user monthly usage", zap.Error(err))
			monthlySummary = &billing.UsageSummary{}
		}

		todaySummary, err = h.billing.GetUsageSummary(c.Request.Context(), id, todayStart, now)
		if err != nil {
			h.logger.Error("failed to get user today usage", zap.Error(err))
			todaySummary = &billing.UsageSummary{}
		}
	}

	result := gin.H{
		"total_requests":     monthlySummary.TotalRequests,
		"success_rate":       monthlySummary.SuccessRate,
		"total_tokens":       monthlySummary.TotalTokens,
		"total_cost":         monthlySummary.TotalCost,
		"average_latency_ms": monthlySummary.AvgLatency,
		"requests_today":     todaySummary.TotalRequests,
		"cost_today":         todaySummary.TotalCost,
		"tokens_today":       todaySummary.TotalTokens,
		"error_count":        monthlySummary.ErrorCount,
	}

	// Admin gets extra system info
	if isAdmin {
		apiKeyHealth, err := h.health.GetAPIKeysHealth(c.Request.Context())
		if err != nil {
			h.logger.Error("failed to get api key health", zap.Error(err))
		}

		proxyHealth, err := h.health.GetProxiesHealth(c.Request.Context())
		if err != nil {
			h.logger.Error("failed to get proxy health", zap.Error(err))
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

		activeProviders := 0
		if providers, err := h.router.GetAllProviders(c.Request.Context()); err == nil {
			for _, p := range providers {
				if p.IsActive {
					activeProviders++
				}
			}
		}

		// Real active users count
		activeUsers, _ := h.userService.CountActiveUsers(c.Request.Context(), todayStart)
		totalUsers, _ := h.userService.CountUsers(c.Request.Context())

		result["active_users"] = activeUsers
		result["total_users"] = totalUsers
		result["active_providers"] = activeProviders
		result["active_proxies"] = healthyProxies
		result["api_keys"] = gin.H{"total": len(apiKeyHealth), "healthy": healthyAPIKeys}
		result["proxies"] = gin.H{"total": len(proxyHealth), "healthy": healthyProxies}
	}

	c.JSON(http.StatusOK, result)
}

// GetUsageChart returns usage chart data for the last 7 days.
// Admin: system-wide. User: personal.
func (h *DashboardHandler) GetUsageChart(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	role, _ := c.Get("role")
	isAdmin := role == "admin"

	var dailyUsage []billing.DailyUsage
	var err error

	if isAdmin {
		dailyUsage, err = h.billing.GetSystemDailyUsage(c.Request.Context(), 7)
	} else {
		id, _ := uuid.Parse(userIDStr.(string))
		dailyUsage, err = h.billing.GetDailyUsage(c.Request.Context(), id, 7)
	}
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
// Admin: system-wide. User: personal.
func (h *DashboardHandler) GetProviderStats(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	role, _ := c.Get("role")
	isAdmin := role == "admin"

	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	var providerUsage []billing.ProviderUsage
	var err error

	if isAdmin {
		providerUsage, err = h.billing.GetSystemUsageByProvider(c.Request.Context(), startTime, now)
	} else {
		id, _ := uuid.Parse(userIDStr.(string))
		providerUsage, err = h.billing.GetUsageByProvider(c.Request.Context(), id, startTime, now)
	}
	if err != nil {
		h.logger.Error("failed to get provider usage", zap.Error(err))
	}

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

	if len(stats) == 0 && isAdmin {
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
// Admin: system-wide. User: personal.
func (h *DashboardHandler) GetModelStats(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	role, _ := c.Get("role")
	isAdmin := role == "admin"

	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	var modelUsage []billing.ModelUsage
	var err error

	if isAdmin {
		modelUsage, err = h.billing.GetSystemUsageByModel(c.Request.Context(), startTime, now)
	} else {
		id, _ := uuid.Parse(userIDStr.(string))
		modelUsage, err = h.billing.GetUsageByModel(c.Request.Context(), id, startTime, now)
	}
	if err != nil {
		h.logger.Error("failed to get model usage", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
		return
	}

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
