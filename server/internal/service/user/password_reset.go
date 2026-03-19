package user

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// resetTokenTTL is the time-to-live for password reset tokens.
	resetTokenTTL = 15 * time.Minute
	// resetTokenBytes is the number of random bytes for the reset token.
	resetTokenBytes = 32
)

var (
	// ErrInvalidResetToken is returned when a reset token is invalid, expired, or already used.
	ErrInvalidResetToken = errors.New("invalid or expired token")
)

// PasswordResetService handles password reset token operations.
type PasswordResetService struct {
	db *gorm.DB
}

// NewPasswordResetService creates a new password reset service.
func NewPasswordResetService(db *gorm.DB) *PasswordResetService {
	return &PasswordResetService{db: db}
}

// CreateResetToken generates a reset token for a user and stores its hash.
// Returns the raw token (to be emailed) — the raw token is NEVER stored.
func (s *PasswordResetService) CreateResetToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// Invalidate any existing unused tokens for this user
	s.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", time.Now())

	// Generate cryptographically random token
	buf := make([]byte, resetTokenBytes)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", err
	}
	rawToken := hex.EncodeToString(buf)

	// Store only the HMAC hash — raw token leaves this function and is never persisted
	tokenHash := hex.EncodeToString(crypto.HMACHash([]byte(rawToken)))

	record := &models.PasswordResetToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(resetTokenTTL),
	}

	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return "", err
	}

	return rawToken, nil
}

// ValidateAndConsumeToken verifies a raw reset token and marks it as used.
// Returns the user ID associated with the token. The token becomes single-use after this call.
func (s *PasswordResetService) ValidateAndConsumeToken(ctx context.Context, rawToken string) (uuid.UUID, error) {
	tokenHash := hex.EncodeToString(crypto.HMACHash([]byte(rawToken)))

	var record models.PasswordResetToken
	err := s.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&record).Error

	if err != nil {
		return uuid.Nil, ErrInvalidResetToken
	}

	// Mark as used (single-use enforcement)
	now := time.Now()
	record.UsedAt = &now
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return uuid.Nil, err
	}

	return record.UserID, nil
}

// CleanupExpiredTokens removes tokens that have expired more than 24 hours ago.
func (s *PasswordResetService) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-24 * time.Hour)
	result := s.db.WithContext(ctx).
		Where("expires_at < ?", cutoff).
		Delete(&models.PasswordResetToken{})
	return result.RowsAffected, result.Error
}
