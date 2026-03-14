// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"unicode"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userService      *user.Service
	auditService     *audit.Service
	jwtConfig        *config.JWTConfig
	registrationMode string
	inviteCode       string // static invite code for mode="invite"
	redisClient      *redis.Client // nil-safe: JTI tracking disabled without Redis
	logger           *zap.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(userService *user.Service, auditSvc *audit.Service, jwtConfig *config.JWTConfig, registrationMode, inviteCode string, redisClient *redis.Client, logger *zap.Logger) *AuthHandler {
	if registrationMode == "" {
		registrationMode = "open"
	}
	return &AuthHandler{
		userService:      userService,
		auditService:     auditSvc,
		jwtConfig:        jwtConfig,
		registrationMode: registrationMode,
		inviteCode:       inviteCode,
		redisClient:      redisClient,
		logger:           logger,
	}
}

// ─── Registration & Login ───────────────────────────────────────────────

// RegisterRequest represents registration request (input only, never serialized).
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password   string `json:"password" binding:"required,min=8,max=128"` // #nosec G101 -- request input only
	Name       string `json:"name" binding:"required,min=1,max=100"`
	InviteCode string `json:"invite_code"`
}

// validatePassword enforces password complexity: min 8 chars, must contain
// at least one uppercase letter, one lowercase letter, and one digit.
func validatePassword(password string) string {
	if len(password) < 8 {
		return "password must be at least 8 characters"
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	if !hasUpper {
		return "password must contain at least one uppercase letter"
	}
	if !hasLower {
		return "password must contain at least one lowercase letter"
	}
	if !hasDigit {
		return "password must contain at least one digit"
	}
	return ""
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Invite code validation: when mode=invite, a valid code is required
	if h.registrationMode == "invite" {
		if h.inviteCode == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "invite-only registration is not configured"})
			return
		}
		if req.InviteCode != h.inviteCode {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid invite code"})
			return
		}
	}
	if h.registrationMode == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "registration is closed"})
		return
	}

	userObj, err := h.userService.Register(c.Request.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.generateToken(userObj.ID, userObj.Email, userObj.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.generateRefreshToken(userObj.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(h.jwtConfig.ExpiresIn.Seconds()),
		"user": gin.H{
			"id":    userObj.ID,
			"email": userObj.Email,
			"name":  userObj.Name,
			"role":  userObj.Role,
		},
	})
}

// LoginRequest represents login request (input only, never serialized).
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"` // #nosec G101 -- request input only
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userObj, err := h.userService.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		// Audit: failed login
		if h.auditService != nil {
			h.auditService.Log(c.Request.Context(), audit.ActionLoginFailed, uuid.Nil, uuid.Nil,
				c.ClientIP(), c.Request.UserAgent(), map[string]interface{}{"email": req.Email})
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, err := h.generateToken(userObj.ID, userObj.Email, userObj.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.generateRefreshToken(userObj.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	// Audit: successful login
	if h.auditService != nil {
		h.auditService.Log(c.Request.Context(), audit.ActionLogin, userObj.ID, userObj.ID,
			c.ClientIP(), c.Request.UserAgent(), nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(h.jwtConfig.ExpiresIn.Seconds()),
		"user": gin.H{
			"id":                      userObj.ID,
			"email":                   userObj.Email,
			"name":                    userObj.Name,
			"role":                    userObj.Role,
			"require_password_change": userObj.RequirePasswordChange,
			"monthly_token_limit":     userObj.MonthlyTokenLimit,
			"monthly_budget_usd":      userObj.MonthlyBudgetUSD,
			"created_at":              userObj.CreatedAt,
		},
	})
}

// ─── Profile & Password ────────────────────────────────────────────────

// GetProfile returns user profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
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

	userObj, err := h.userService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      userObj.ID,
		"email":                   userObj.Email,
		"name":                    userObj.Name,
		"role":                    userObj.Role,
		"require_password_change": userObj.RequirePasswordChange,
		"monthly_token_limit":     userObj.MonthlyTokenLimit,
		"monthly_budget_usd":      userObj.MonthlyBudgetUSD,
	})
}

// UpdateProfileRequest represents profile update request.
type UpdateProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

// UpdateProfile updates user profile.
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
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

	if err := h.userService.UpdateProfile(c.Request.Context(), id, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
}

// ChangePasswordRequest represents password change request.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// ChangePassword changes user password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if msg := validatePassword(req.NewPassword); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
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

	if err := h.userService.ChangePassword(c.Request.Context(), id, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}
