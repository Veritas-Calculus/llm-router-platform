package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"llm-router-platform/pkg/sanitize"
)

// ─── Token Generation ───────────────────────────────────────────────────

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

// ─── Refresh Token Rotation ─────────────────────────────────────────────

// RefreshToken godoc
// @Summary      Refresh access token
// @Description  Issue a new JWT access token using the authenticated user's current state
// @Tags         auth
// @Produce      json
// @Success      200 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Router       /api/v1/auth/refresh [post]
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

// RefreshTokenRequest represents a token refresh request.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RotateRefreshToken godoc
// @Summary      Rotate refresh token
// @Description  Exchange a valid refresh token for a new access+refresh pair (single-use)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RefreshTokenRequest true "Refresh token"
// @Success      200 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Router       /api/v1/auth/token/rotate [post]
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
				zap.String("jti", sanitize.LogValue(jti)),
				zap.String("ip", sanitize.LogValue(c.ClientIP())))
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

// ─── JTI Tracking ───────────────────────────────────────────────────────

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
		h.logger.Error("failed to mark JTI as consumed", zap.String("jti", sanitize.LogValue(jti)), zap.Error(err))
	}
}
