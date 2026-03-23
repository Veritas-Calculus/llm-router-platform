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
	// emailVerifyTokenTTL is the time-to-live for verification tokens.
	emailVerifyTokenTTL = 24 * time.Hour
	// emailVerifyTokenBytes is the number of random bytes for the token.
	emailVerifyTokenBytes = 32
)

var (
	// ErrInvalidVerifyToken is returned when a verification token is invalid, expired, or already used.
	ErrInvalidVerifyToken = errors.New("invalid or expired verification link")
	// ErrAlreadyVerified is returned when a user's email is already verified.
	ErrAlreadyVerified = errors.New("email is already verified")
)

// EmailVerificationService handles email verification token operations.
type EmailVerificationService struct {
	db *gorm.DB
}

// NewEmailVerificationService creates a new email verification service.
func NewEmailVerificationService(db *gorm.DB) *EmailVerificationService {
	return &EmailVerificationService{db: db}
}

// CreateVerificationToken generates a verification token for a user and stores its hash.
// Returns the raw token (to be emailed) — the raw token is NEVER stored.
func (s *EmailVerificationService) CreateVerificationToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// Invalidate any existing unused tokens for this user
	s.db.WithContext(ctx).
		Model(&models.EmailVerificationToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", time.Now())

	// Generate cryptographically random token
	buf := make([]byte, emailVerifyTokenBytes)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", err
	}
	rawToken := hex.EncodeToString(buf)

	// Store only the HMAC hash
	tokenHash := hex.EncodeToString(crypto.HMACHash([]byte(rawToken)))

	record := &models.EmailVerificationToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(emailVerifyTokenTTL),
	}

	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return "", err
	}

	return rawToken, nil
}

// VerifyEmail validates a raw verification token and marks the user's email as verified.
// Returns the user ID associated with the token.
func (s *EmailVerificationService) VerifyEmail(ctx context.Context, rawToken string) (uuid.UUID, error) {
	tokenHash := hex.EncodeToString(crypto.HMACHash([]byte(rawToken)))

	var record models.EmailVerificationToken
	err := s.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&record).Error

	if err != nil {
		return uuid.Nil, ErrInvalidVerifyToken
	}

	// Mark token as used
	now := time.Now()
	record.UsedAt = &now
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return uuid.Nil, err
	}

	// Mark user email as verified
	result := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", record.UserID).
		Updates(map[string]interface{}{
			"email_verified":    true,
			"email_verified_at": now,
		})

	if result.Error != nil {
		return uuid.Nil, result.Error
	}

	return record.UserID, nil
}
