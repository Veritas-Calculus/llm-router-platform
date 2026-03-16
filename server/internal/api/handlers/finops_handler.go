// Package handlers provides HTTP request handlers.
// This file implements FinOps endpoints: budgets, anomaly detection, and CSV export.
package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"llm-router-platform/internal/service/billing"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FinOpsHandler handles budget, anomaly, and export operations.
type FinOpsHandler struct {
	billing       *billing.Service
	budgetService *billing.BudgetService
	logger        *zap.Logger
}

// NewFinOpsHandler creates a new FinOps handler.
func NewFinOpsHandler(billingService *billing.Service, budgetService *billing.BudgetService, logger *zap.Logger) *FinOpsHandler {
	return &FinOpsHandler{
		billing:       billingService,
		budgetService: budgetService,
		logger:        logger,
	}
}

// ─── Budget Endpoints ───────────────────────────────────────

// SetBudgetRequest represents a budget creation request.
type SetBudgetRequest struct {
	MonthlyLimitUSD float64 `json:"monthly_limit_usd" binding:"required,gt=0"`
	AlertThreshold  float64 `json:"alert_threshold" binding:"omitempty,gt=0,lte=1"`
	WebhookURL      string  `json:"webhook_url,omitempty"`
	Email           string  `json:"email,omitempty"`
}

// SetBudget creates or updates the budget for the current user.
func (h *FinOpsHandler) SetBudget(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	var req SetBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// SSRF prevention: validate webhook URL does not point to internal networks
	if err := sanitize.ValidateWebhookURL(req.WebhookURL, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_url: " + err.Error()})
		return
	}

	threshold := req.AlertThreshold
	if threshold == 0 {
		threshold = 0.8 // Default 80%
	}

	budget, err := h.budgetService.SetBudget(c.Request.Context(), userID, req.MonthlyLimitUSD, threshold, req.WebhookURL, req.Email)
	if err != nil {
		h.logger.Error("failed to set budget", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set budget"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"budget": budget})
}

// GetBudget returns the budget for the current user.
func (h *FinOpsHandler) GetBudget(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	budget := h.budgetService.GetBudget(c.Request.Context(), userID)
	if budget == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no budget set"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"budget": budget})
}

// GetBudgetStatus returns budget status including current spend.
func (h *FinOpsHandler) GetBudgetStatus(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	status, err := h.budgetService.CheckBudget(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("budget check failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check budget"})
		return
	}

	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no budget set"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// DeleteBudget removes the budget for the current user.
func (h *FinOpsHandler) DeleteBudget(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	if err := h.budgetService.DeleteBudget(c.Request.Context(), userID); err != nil {
		h.logger.Error("failed to delete budget", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete budget"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "budget deleted"})
}

// ─── Anomaly Detection ──────────────────────────────────────

// DetectAnomaly checks for cost anomalies for the current user.
func (h *FinOpsHandler) DetectAnomaly(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	windowDays, _ := strconv.Atoi(c.DefaultQuery("window_days", "14"))
	threshold, _ := strconv.ParseFloat(c.DefaultQuery("threshold", "3.0"), 64)

	result, err := h.billing.DetectCostAnomaly(c.Request.Context(), userID, windowDays, threshold)
	if err != nil {
		h.logger.Error("anomaly detection failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "anomaly detection failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DetectSystemAnomaly checks for system-wide cost anomalies (admin only).
func (h *FinOpsHandler) DetectSystemAnomaly(c *gin.Context) {
	windowDays, _ := strconv.Atoi(c.DefaultQuery("window_days", "14"))
	threshold, _ := strconv.ParseFloat(c.DefaultQuery("threshold", "3.0"), 64)

	result, err := h.billing.DetectSystemCostAnomaly(c.Request.Context(), windowDays, threshold)
	if err != nil {
		h.logger.Error("system anomaly detection failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "anomaly detection failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ─── CSV Export ──────────────────────────────────────────────

// ExportUsage exports the current user's usage as CSV.
func (h *FinOpsHandler) ExportUsage(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	startTime, endTime := parseTimeRange(c)

	filename := fmt.Sprintf("usage_report_%s_%s.csv",
		startTime.Format("20060102"),
		endTime.Format("20060102"),
	)

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	if err := h.billing.ExportUsageCSV(c.Request.Context(), userID, startTime, endTime, w); err != nil {
		h.logger.Error("CSV export failed", zap.Error(err))
		// Can't change status code after headers sent, just log
	}
}

// ExportSystemUsage exports system-wide usage as CSV (admin only).
func (h *FinOpsHandler) ExportSystemUsage(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	filename := fmt.Sprintf("system_usage_%s_%s.csv",
		startTime.Format("20060102"),
		endTime.Format("20060102"),
	)

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	if err := h.billing.ExportSystemUsageCSV(c.Request.Context(), startTime, endTime, w); err != nil {
		h.logger.Error("system CSV export failed", zap.Error(err))
	}
}

// ─── Helpers ────────────────────────────────────────────────

func parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	startStr := c.Query("start")
	endStr := c.Query("end")

	var startTime, endTime time.Time

	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			startTime = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			endTime = t.Add(24*time.Hour - time.Second) // End of day
		}
	}

	if startTime.IsZero() {
		startTime = now.AddDate(0, 0, -days)
	}
	if endTime.IsZero() {
		endTime = now
	}

	return startTime, endTime
}
