// Package user provides user and API key management services.
package user

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Service handles user and API key operations.
type Service struct {
	userRepo   *repository.UserRepository
	apiKeyRepo *repository.APIKeyRepository
	logger     *zap.Logger
}

// NewService creates a new user service.
func NewService(
	userRepo *repository.UserRepository,
	apiKeyRepo *repository.APIKeyRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		userRepo:   userRepo,
		apiKeyRepo: apiKeyRepo,
		logger:     logger,
	}
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, email, password, name string) (*models.User, error) {
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         name,
		Role:         "user",
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Authenticate validates user credentials and returns the user.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !user.IsActive {
		return nil, errors.New("account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

// GetByID retrieves a user by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

// UpdateProfile updates user profile information.
func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, name string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	user.Name = name
	return s.userRepo.Update(ctx, user)
}

// ChangePassword updates user password.
func (s *Service) ChangePassword(ctx context.Context, id uuid.UUID, oldPass, newPass string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPass)); err != nil {
		return errors.New("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	return s.userRepo.Update(ctx, user)
}

// CreateAPIKey generates a new API key for a user.
func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string) (*models.APIKey, string, error) {
	rawKey := generateAPIKey()
	hashedKey := hashAPIKey(rawKey)

	apiKey := &models.APIKey{
		UserID:     userID,
		KeyHash:    hashedKey,
		KeyPrefix:  rawKey[:8],
		Name:       name,
		IsActive:   true,
		RateLimit:  1000,
		DailyLimit: 10000,
	}

	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, "", err
	}

	return apiKey, rawKey, nil
}

// GetAPIKeys returns all API keys for a user.
func (s *Service) GetAPIKeys(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	return s.apiKeyRepo.GetByUserID(ctx, userID)
}

// GetAllAPIKeys returns all API keys in the system (for admin view).
func (s *Service) GetAllAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	return s.apiKeyRepo.GetAll(ctx)
}

// ValidateAPIKey validates an API key and returns the associated user.
func (s *Service) ValidateAPIKey(ctx context.Context, rawKey string) (*models.User, *models.APIKey, error) {
	hashedKey := hashAPIKey(rawKey)
	apiKey, err := s.apiKeyRepo.GetByKeyHash(ctx, hashedKey)
	if err != nil {
		return nil, nil, errors.New("invalid API key")
	}

	if !apiKey.IsActive {
		return nil, nil, errors.New("API key is disabled")
	}

	if apiKey.ExpiresAt.Before(time.Now()) && !apiKey.ExpiresAt.IsZero() {
		return nil, nil, errors.New("API key has expired")
	}

	user, err := s.userRepo.GetByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, nil, err
	}

	if !user.IsActive {
		return nil, nil, errors.New("user account is disabled")
	}

	now := time.Now()
	apiKey.LastUsedAt = now
	_ = s.apiKeyRepo.Update(ctx, apiKey)

	return user, apiKey, nil
}

// RevokeAPIKey deactivates an API key.
func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return err
	}

	if apiKey.UserID != userID {
		return errors.New("unauthorized")
	}

	apiKey.IsActive = false
	return s.apiKeyRepo.Update(ctx, apiKey)
}

// DeleteAPIKey permanently removes an API key from the database.
func (s *Service) DeleteAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return err
	}

	if apiKey.UserID != userID {
		return errors.New("unauthorized")
	}

	return s.apiKeyRepo.Delete(ctx, keyID)
}

// GetAPIKeyByID retrieves an API key by ID.
func (s *Service) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	return s.apiKeyRepo.GetByID(ctx, id)
}

// generateAPIKey creates a new random API key.
func generateAPIKey() string {
	id := uuid.New().String()
	return "llm_" + strings.ReplaceAll(id, "-", "")
}

// hashAPIKey creates a deterministic hash of the API key for storage and lookup.
// Note: We intentionally use SHA-256 instead of bcrypt here because:
// 1. API keys are high-entropy random strings (128+ bits), not user-chosen passwords
// 2. We need O(1) lookup by hash, which bcrypt cannot provide
// 3. API keys are generated by us, not user input, so no dictionary attacks apply
// #nosec G401 - SHA256 is appropriate for API key hashing, not password hashing
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
