// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"time"
	"unicode"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userService      *user.Service
	auditService     *audit.Service
	jwtConfig        *config.JWTConfig
	registrationMode string
	logger           *zap.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(userService *user.Service, auditSvc *audit.Service, jwtConfig *config.JWTConfig, registrationMode string, logger *zap.Logger) *AuthHandler {
	if registrationMode == "" {
		registrationMode = "open"
	}
	return &AuthHandler{
		userService:      userService,
		auditService:     auditSvc,
		jwtConfig:        jwtConfig,
		registrationMode: registrationMode,
		logger:           logger,
	}
}

// RegisterRequest represents registration request (input only, never serialized).
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=128"` // #nosec G101 -- request input only
	Name     string `json:"name" binding:"required,min=1,max=100"`
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
	if !hasUpper || !hasLower || !hasDigit {
		return "password must contain at least one uppercase letter, one lowercase letter, and one digit"
	}
	return ""
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	// Enforce registration mode
	switch h.registrationMode {
	case "closed":
		c.JSON(http.StatusForbidden, gin.H{"error": "registration is currently closed"})
		return
	case "invite":
		// TODO: validate invite code from request
		c.JSON(http.StatusForbidden, gin.H{"error": "registration requires an invitation code"})
		return
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	userObj, err := h.userService.Register(c.Request.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-login: generate token for newly registered user
	token, err := h.generateToken(userObj.ID, userObj.Email, userObj.Role)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{
			"id":    userObj.ID,
			"email": userObj.Email,
			"name":  userObj.Name,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":         userObj.ID,
			"email":      userObj.Email,
			"name":       userObj.Name,
			"role":       userObj.Role,
			"created_at": userObj.CreatedAt,
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

	// Audit: successful login
	if h.auditService != nil {
		h.auditService.Log(c.Request.Context(), audit.ActionLogin, userObj.ID, userObj.ID,
			c.ClientIP(), c.Request.UserAgent(), nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":         userObj.ID,
			"email":      userObj.Email,
			"name":       userObj.Name,
			"role":       userObj.Role,
			"created_at": userObj.CreatedAt,
		},
	})
}

// RefreshToken refreshes JWT token using current DB state.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
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

	// Query database for current user state (not from old JWT claims)
	userObj, err := h.userService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	if !userObj.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is disabled"})
		return
	}

	token, err := h.generateToken(userObj.ID, userObj.Email, userObj.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

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
		"id":    userObj.ID,
		"email": userObj.Email,
		"name":  userObj.Name,
		"role":  userObj.Role,
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

// generateToken creates a JWT token.
func (h *AuthHandler) generateToken(userID uuid.UUID, email, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"role":  role,
		"exp":   time.Now().Add(h.jwtConfig.ExpiresIn).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtConfig.Secret))
}

// APIKeyHandler handles API key endpoints.
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

// Create creates a new API key.
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

// List returns all API keys for the authenticated user.
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

// Revoke deactivates an API key.
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

// Delete permanently removes an API key.
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
