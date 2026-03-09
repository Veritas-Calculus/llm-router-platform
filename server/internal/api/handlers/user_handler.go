// Package handlers provides HTTP request handlers.
// This file implements admin user management endpoints.
package handlers

import (
	"net/http"
	"time"

	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UserHandler handles admin user management endpoints.
type UserHandler struct {
	userService *user.Service
	billing     *billing.Service
	logger      *zap.Logger
}

// NewUserHandler creates a new user handler.
func NewUserHandler(userService *user.Service, billing *billing.Service, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		billing:     billing,
		logger:      logger,
	}
}

// UserResponse represents a user in admin API responses.
type UserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	IsActive    bool   `json:"is_active"`
	LastLoginAt string `json:"last_login_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	APIKeyCount int    `json:"api_key_count"`
}

// List returns all users (admin only).
func (h *UserHandler) List(c *gin.Context) {
	query := c.Query("q")

	var err error
	var users []struct {
		ID          uuid.UUID `json:"id"`
		Email       string    `json:"email"`
		Name        string    `json:"name"`
		Role        string    `json:"role"`
		IsActive    bool      `json:"is_active"`
		LastLoginAt time.Time `json:"last_login_at"`
		CreatedAt   time.Time `json:"created_at"`
	}

	if query != "" {
		rawUsers, searchErr := h.userService.SearchUsers(c.Request.Context(), query)
		if searchErr != nil {
			h.logger.Error("user search failed", zap.Error(searchErr))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search users"})
			return
		}
		for _, u := range rawUsers {
			users = append(users, struct {
				ID          uuid.UUID `json:"id"`
				Email       string    `json:"email"`
				Name        string    `json:"name"`
				Role        string    `json:"role"`
				IsActive    bool      `json:"is_active"`
				LastLoginAt time.Time `json:"last_login_at"`
				CreatedAt   time.Time `json:"created_at"`
			}{u.ID, u.Email, u.Name, u.Role, u.IsActive, u.LastLoginAt, u.CreatedAt})
		}
		err = nil
	} else {
		rawUsers, listErr := h.userService.ListUsers(c.Request.Context())
		if listErr != nil {
			h.logger.Error("list users failed", zap.Error(listErr))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
			return
		}
		for _, u := range rawUsers {
			users = append(users, struct {
				ID          uuid.UUID `json:"id"`
				Email       string    `json:"email"`
				Name        string    `json:"name"`
				Role        string    `json:"role"`
				IsActive    bool      `json:"is_active"`
				LastLoginAt time.Time `json:"last_login_at"`
				CreatedAt   time.Time `json:"created_at"`
			}{u.ID, u.Email, u.Name, u.Role, u.IsActive, u.LastLoginAt, u.CreatedAt})
		}
		err = nil
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	// Build response with key counts
	var responses []gin.H
	for _, u := range users {
		keys, _ := h.userService.GetAPIKeys(c.Request.Context(), u.ID)

		responses = append(responses, gin.H{
			"id":            u.ID.String(),
			"email":         u.Email,
			"name":          u.Name,
			"role":          u.Role,
			"is_active":     u.IsActive,
			"last_login_at": u.LastLoginAt.Format(time.RFC3339),
			"created_at":    u.CreatedAt.Format(time.RFC3339),
			"api_key_count": len(keys),
		})
	}

	if responses == nil {
		responses = make([]gin.H, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  responses,
		"total": len(responses),
	})
}

// GetUser returns a single user with usage stats (admin only).
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	u, err := h.userService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	keys, _ := h.userService.GetAPIKeys(c.Request.Context(), id)

	// Get usage summary for this month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	summary, _ := h.billing.GetUsageSummary(c.Request.Context(), id, monthStart, now)
	if summary == nil {
		summary = &billing.UsageSummary{}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                  u.ID.String(),
		"email":               u.Email,
		"name":                u.Name,
		"role":                u.Role,
		"is_active":           u.IsActive,
		"created_at":          u.CreatedAt.Format(time.RFC3339),
		"api_keys":            len(keys),
		"usage_month":         summary,
		"monthly_token_limit": u.MonthlyTokenLimit,
		"monthly_budget_usd":  u.MonthlyBudgetUSD,
	})
}

// GetUserUsage returns usage stats for a specific user (admin only).
func (h *UserHandler) GetUserUsage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Verify user exists
	_, err = h.userService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	dailyUsage, err := h.billing.GetDailyUsage(c.Request.Context(), id, 30)
	if err != nil {
		h.logger.Error("failed to get user usage", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get usage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dailyUsage})
}

// GetUserAPIKeys returns API keys for a specific user (admin only).
func (h *UserHandler) GetUserAPIKeys(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	keys, err := h.userService.GetAPIKeys(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": keys})
}

// ToggleUser enables or disables a user (admin only).
func (h *UserHandler) ToggleUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Prevent admin from disabling themselves
	currentUserID := c.GetString("user_id")
	if currentUserID == id.String() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot disable your own account"})
		return
	}

	u, err := h.userService.ToggleUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        u.ID.String(),
		"email":     u.Email,
		"name":      u.Name,
		"role":      u.Role,
		"is_active": u.IsActive,
	})
}

// UpdateRoleRequest represents a role change request.
type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin"`
}

// UpdateRole changes a user's role (admin only).
func (h *UserHandler) UpdateRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Prevent admin from changing their own role
	currentUserID := c.GetString("user_id")
	if currentUserID == id.String() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change your own role"})
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.userService.UpdateRole(c.Request.Context(), id, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    u.ID.String(),
		"email": u.Email,
		"name":  u.Name,
		"role":  u.Role,
	})
}

// UpdateQuotaRequest represents a quota update request.
type UpdateQuotaRequest struct {
	MonthlyTokenLimit *int64   `json:"monthly_token_limit"` // nil = don't change, 0 = unlimited
	MonthlyBudgetUSD  *float64 `json:"monthly_budget_usd"`  // nil = don't change, 0 = unlimited
}

// UpdateQuota updates a user's quota limits (admin only).
func (h *UserHandler) UpdateQuota(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.userService.UpdateQuota(c.Request.Context(), id, req.MonthlyTokenLimit, req.MonthlyBudgetUSD)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                  u.ID.String(),
		"email":               u.Email,
		"name":                u.Name,
		"monthly_token_limit": u.MonthlyTokenLimit,
		"monthly_budget_usd":  u.MonthlyBudgetUSD,
	})
}
