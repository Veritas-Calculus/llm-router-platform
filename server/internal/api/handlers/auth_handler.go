// Package handlers provides HTTP request handlers.
package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
	"unicode"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Account lockout constants — per-email protection against distributed brute force.
const (
	accountLockoutThreshold = 10              // Failed attempts before lockout
	accountLockoutDuration  = 30 * time.Minute // Lockout window
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userService      *user.Service
	auditService     *audit.Service
	jwtConfig        *config.JWTConfig
	registrationMode string
	inviteCode       string // static invite code (legacy fallback)
	redisClient      *redis.Client
	db               *gorm.DB // for DB-backed invite codes
	logger           *zap.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(userService *user.Service, auditSvc *audit.Service, jwtConfig *config.JWTConfig, registrationMode, inviteCode string, redisClient *redis.Client, db *gorm.DB, logger *zap.Logger) *AuthHandler {
	if registrationMode == "" {
		registrationMode = "closed"
	}
	return &AuthHandler{
		userService:      userService,
		auditService:     auditSvc,
		jwtConfig:        jwtConfig,
		registrationMode: registrationMode,
		inviteCode:       inviteCode,
		redisClient:      redisClient,
		db:               db,
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

	// Invite code validation: when mode=invite, check DB codes first, fallback to static code
	if h.registrationMode == "invite" {
		if req.InviteCode == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "invite code is required"})
			return
		}
		if !h.validateAndConsumeInviteCode(c.Request.Context(), req.InviteCode) {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired invite code"})
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

	// Audit: successful registration
	if h.auditService != nil {
		h.auditService.Log(c.Request.Context(), audit.ActionRegister, userObj.ID, userObj.ID,
			c.ClientIP(), c.Request.UserAgent(), map[string]interface{}{"email": userObj.Email})
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

	// M7: Per-account lockout — check if this email is locked out
	if h.isAccountLockedOut(c.Request.Context(), req.Email) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "account temporarily locked due to too many failed attempts",
			"retry_after": int(accountLockoutDuration.Seconds()),
		})
		return
	}

	userObj, err := h.userService.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		// Audit: failed login
		if h.auditService != nil {
			h.auditService.Log(c.Request.Context(), audit.ActionLoginFailed, uuid.Nil, uuid.Nil,
				c.ClientIP(), c.Request.UserAgent(), map[string]interface{}{"email": req.Email})
		}
		// Increment lockout counter
		h.recordFailedLogin(c.Request.Context(), req.Email)
		// H3: Hardcoded error — never propagate internal error details to prevent user enumeration
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Clear lockout counter on successful login
	h.clearFailedLogins(c.Request.Context(), req.Email)

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

// Logout invalidates all existing tokens for the current user.
func (h *AuthHandler) Logout(c *gin.Context) {
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

	if err := h.userService.InvalidateTokens(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
		return
	}

	// Audit: logout
	if h.auditService != nil {
		h.auditService.Log(c.Request.Context(), audit.ActionLogout, id, id,
			c.ClientIP(), c.Request.UserAgent(), nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// ─── Account Lockout Helpers ────────────────────────────────────────────

// isAccountLockedOut checks whether the given email has exceeded the lockout threshold.
func (h *AuthHandler) isAccountLockedOut(ctx context.Context, email string) bool {
	if h.redisClient == nil {
		return false // No Redis = no lockout (per-IP rate limit still applies)
	}
	key := fmt.Sprintf("auth:lockout:%s", email)
	count, err := h.redisClient.Get(ctx, key).Int()
	if err != nil {
		return false
	}
	return count >= accountLockoutThreshold
}

// recordFailedLogin increments the per-account failure counter in Redis.
func (h *AuthHandler) recordFailedLogin(ctx context.Context, email string) {
	if h.redisClient == nil {
		return
	}
	key := fmt.Sprintf("auth:lockout:%s", email)
	count, err := h.redisClient.Incr(ctx, key).Result()
	if err != nil {
		h.logger.Warn("failed to record login failure", zap.Error(err))
		return
	}
	if count == 1 {
		h.redisClient.Expire(ctx, key, accountLockoutDuration)
	}
}

// clearFailedLogins removes the per-account failure counter after successful login.
func (h *AuthHandler) clearFailedLogins(ctx context.Context, email string) {
	if h.redisClient == nil {
		return
	}
	key := fmt.Sprintf("auth:lockout:%s", email)
	h.redisClient.Del(ctx, key)
}

// ─── Invite Code Methods ────────────────────────────────────────────────

// validateAndConsumeInviteCode atomically checks and consumes an invite code.
// Uses a single SQL UPDATE with conditions to avoid race conditions (M1 fix).
func (h *AuthHandler) validateAndConsumeInviteCode(ctx context.Context, code string) bool {
	// 1. Try DB-backed invite codes — atomic check-and-consume
	if h.db != nil {
		now := time.Now()
		result := h.db.WithContext(ctx).
			Model(&models.InviteCode{}).
			Where("code = ? AND is_active = true AND (max_uses = 0 OR use_count < max_uses) AND (expires_at IS NULL OR expires_at > ?)", code, now).
			UpdateColumn("use_count", gorm.Expr("use_count + 1"))
		if result.Error == nil && result.RowsAffected > 0 {
			return true
		}
		// If the code exists in DB but wasn't consumed, it's invalid
		var exists int64
		if h.db.WithContext(ctx).Model(&models.InviteCode{}).Where("code = ?", code).Count(&exists).Error == nil && exists > 0 {
			return false // Code exists but is expired/exhausted/disabled
		}
	}

	// 2. Fallback: legacy static invite code
	if h.inviteCode != "" && code == h.inviteCode {
		return true
	}

	return false
}

// CreateInviteCodeRequest represents a request to create an invite code (admin only).
type CreateInviteCodeRequest struct {
	MaxUses   int        `json:"max_uses"` // 0 = unlimited, 1 = one-time
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateInviteCode generates a new invite code (admin only).
func (h *AuthHandler) CreateInviteCode(c *gin.Context) {
	var req CreateInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxUses == 0 {
		req.MaxUses = 1 // Default: one-time use
	}

	// Generate cryptographically secure invite code
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate code"})
		return
	}
	code := "inv_" + hex.EncodeToString(buf)

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	ic := models.InviteCode{
		Code:      code,
		CreatedBy: adminID,
		MaxUses:   req.MaxUses,
		ExpiresAt: req.ExpiresAt,
		IsActive:  true,
	}

	if err := h.db.Create(&ic).Error; err != nil {
		h.logger.Error("failed to create invite code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invite code"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":       ic.Code,
		"max_uses":   ic.MaxUses,
		"expires_at": ic.ExpiresAt,
	})
}

// ListInviteCodes returns all invite codes (admin only).
func (h *AuthHandler) ListInviteCodes(c *gin.Context) {
	var codes []models.InviteCode
	if err := h.db.Order("created_at DESC").Find(&codes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list invite codes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": codes})
}
