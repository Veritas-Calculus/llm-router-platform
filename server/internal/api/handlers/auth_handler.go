// Package handlers provides HTTP request handlers.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"unicode"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userService      *user.Service
	auditService     *audit.Service
	jwtConfig        *config.JWTConfig
	registrationMode string
	redisClient      *redis.Client // nil-safe: JTI tracking disabled without Redis
	logger           *zap.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(userService *user.Service, auditSvc *audit.Service, jwtConfig *config.JWTConfig, registrationMode string, redisClient *redis.Client, logger *zap.Logger) *AuthHandler {
	if registrationMode == "" {
		registrationMode = "open"
	}
	return &AuthHandler{
		userService:      userService,
		auditService:     auditSvc,
		jwtConfig:        jwtConfig,
		registrationMode: registrationMode,
		redisClient:      redisClient,
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

// generateToken creates a JWT access token.
func (h *AuthHandler) generateToken(userID uuid.UUID, email, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"role":  role,
		"type":  "access",
		"exp":   time.Now().Add(h.jwtConfig.ExpiresIn).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtConfig.Secret))
}

// generateRefreshToken creates a long-lived refresh token (7 days).
func (h *AuthHandler) generateRefreshToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "refresh",
		"jti":  uuid.New().String(), // unique token ID for rotation
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtConfig.Secret))
}

// RefreshTokenRequest represents a token refresh request.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RotateRefreshToken accepts a valid refresh token and returns a new access+refresh pair.
// This implements the refresh-token rotation pattern: each refresh token is single-use.
// If Redis is available, the token's JTI is tracked to prevent reuse.
func (h *AuthHandler) RotateRefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse and validate the refresh token
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(h.jwtConfig.Secret), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
		return
	}

	// Verify this is a refresh token, not an access token
	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token is not a refresh token"})
		return
	}

	// JTI reuse detection: each refresh token can only be used once
	jti, _ := claims["jti"].(string)
	if jti != "" {
		if consumed := h.isJTIConsumed(c.Request.Context(), jti); consumed {
			h.logger.Warn("refresh token reuse detected — possible token theft",
				zap.String("jti", jti),
				zap.String("ip", c.ClientIP()))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token has already been used"})
			return
		}
	}

	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
		return
	}

	// Fetch current user state from database
	userObj, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	if !userObj.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is disabled"})
		return
	}

	// Check token wasn't issued before tokens were invalidated
	if !userObj.TokensInvalidatedAt.IsZero() {
		iat, _ := claims["iat"].(float64)
		if time.Unix(int64(iat), 0).Before(userObj.TokensInvalidatedAt) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}
	}

	// Mark the JTI as consumed BEFORE issuing new tokens
	if jti != "" {
		exp, _ := claims["exp"].(float64)
		ttl := time.Until(time.Unix(int64(exp), 0))
		h.consumeJTI(c.Request.Context(), jti, ttl)
	}

	// Issue new access token + refresh token (rotation)
	newAccessToken, err := h.generateToken(userObj.ID, userObj.Email, userObj.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	newRefreshToken, err := h.generateRefreshToken(userObj.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         newAccessToken,
		"refresh_token": newRefreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(h.jwtConfig.ExpiresIn.Seconds()),
	})
}

// isJTIConsumed checks if a refresh token JTI has already been used.
// Returns false if Redis is unavailable (fail-open for availability).
func (h *AuthHandler) isJTIConsumed(ctx context.Context, jti string) bool {
	if h.redisClient == nil {
		return false
	}
	key := "rt:jti:" + jti
	exists, err := h.redisClient.Exists(ctx, key).Result()
	if err != nil {
		h.logger.Warn("redis JTI check failed, allowing token", zap.Error(err))
		return false
	}
	return exists > 0
}

// consumeJTI marks a refresh token JTI as consumed in Redis.
func (h *AuthHandler) consumeJTI(ctx context.Context, jti string, ttl time.Duration) {
	if h.redisClient == nil {
		return
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour // fallback: refresh token lifetime
	}
	key := "rt:jti:" + jti
	if err := h.redisClient.Set(ctx, key, "1", ttl).Err(); err != nil {
		h.logger.Error("failed to mark JTI as consumed", zap.String("jti", jti), zap.Error(err))
	}
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
