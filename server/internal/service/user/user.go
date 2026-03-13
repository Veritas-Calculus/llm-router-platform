// Package user provides user and API key management services.
package user

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"

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
		return nil, errors.New("registration failed") // generic to prevent user enumeration
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

// ListUsers returns all users (admin only).
func (s *Service) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.userRepo.GetAll(ctx)
}

// SearchUsers searches users by email or name (admin only).
func (s *Service) SearchUsers(ctx context.Context, query string) ([]models.User, error) {
	return s.userRepo.Search(ctx, query)
}

// ToggleUser enables or disables a user account and invalidates tokens (admin only).
func (s *Service) ToggleUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user.IsActive = !user.IsActive
	if !user.IsActive {
		// When disabling, invalidate all tokens immediately
		user.TokensInvalidatedAt = time.Now()
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	s.logger.Info("user toggled",
		zap.String("user_id", id.String()),
		zap.Bool("is_active", user.IsActive),
	)
	return user, nil
}

// InvalidateTokens forces all existing tokens for a user to be rejected.
func (s *Service) InvalidateTokens(ctx context.Context, id uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	user.TokensInvalidatedAt = time.Now()
	s.logger.Info("tokens invalidated for user", zap.String("user_id", id.String()))
	return s.userRepo.Update(ctx, user)
}

// UpdateRole changes a user's role (admin only).
func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, role string) (*models.User, error) {
	if role != "user" && role != "admin" {
		return nil, errors.New("invalid role: must be 'user' or 'admin'")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user.Role = role
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	s.logger.Info("user role updated",
		zap.String("user_id", id.String()),
		zap.String("role", sanitize.LogValue(role)),
	)
	return user, nil
}

// CountUsers returns the total number of registered users.
func (s *Service) CountUsers(ctx context.Context) (int64, error) {
	return s.userRepo.Count(ctx)
}

// CountActiveUsers returns users who made API calls since a given time.
func (s *Service) CountActiveUsers(ctx context.Context, since time.Time) (int64, error) {
	return s.userRepo.CountActiveUsers(ctx, since)
}

// UpdateQuota updates a user's quota limits (admin only).
func (s *Service) UpdateQuota(ctx context.Context, id uuid.UUID, tokenLimit *int64, budgetLimit *float64) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tokenLimit != nil {
		user.MonthlyTokenLimit = *tokenLimit
	}
	if budgetLimit != nil {
		user.MonthlyBudgetUSD = *budgetLimit
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	s.logger.Info("user quota updated",
		zap.String("user_id", id.String()),
		zap.Int64("monthly_token_limit", user.MonthlyTokenLimit),
		zap.Float64("monthly_budget_usd", user.MonthlyBudgetUSD),
	)
	return user, nil
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

// ChangePassword updates user password and invalidates all existing tokens.
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
	user.RequirePasswordChange = false
	user.TokensInvalidatedAt = time.Now() // revoke all existing tokens
	s.logger.Info("password changed, tokens invalidated", zap.String("user_id", id.String()))
	return s.userRepo.Update(ctx, user)
}

// MaxAPIKeysPerUser is the maximum number of API keys a user can create.
const MaxAPIKeysPerUser = 20

// CreateAPIKey generates a new API key for a user.
func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string) (*models.APIKey, string, error) {
	// Enforce max API key limit
	existing, err := s.apiKeyRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if len(existing) >= MaxAPIKeysPerUser {
		return nil, "", errors.New("maximum number of API keys reached")
	}

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

// generateAPIKey creates a new cryptographically random API key.
// Uses crypto/rand for 256-bit entropy (32 bytes hex-encoded).
func generateAPIKey() string {
	b := make([]byte, 32) // 256-bit
	if _, err := cryptorand.Read(b); err != nil {
		// Fallback to UUID if crypto/rand fails (should never happen)
		id := uuid.New().String()
		return "llm_" + strings.ReplaceAll(id, "-", "")
	}
	return "llm_" + hex.EncodeToString(b)
}

// hashAPIKey creates a deterministic keyed hash of the API key for storage and lookup.
// We use HMAC-SHA256 with the system's encryption key as a salt.
// Note: We use a keyed hash instead of bcrypt because:
// 1. API keys are high-entropy random strings (128+ bits), not user-chosen passwords
// 2. We need O(1) lookup by hash, which bcrypt/Argon2 cannot provide
// 3. HMAC-SHA256 with a system-level salt prevents rainbow table attacks
func hashAPIKey(key string) string {
	// If encryption is not initialized, fall back to simple SHA-256 for stability
	// but in production we should always have a salt.
	if !crypto.IsInitialized() {
		// Fallback HMAC for startup/testing when ENCRYPTION_KEY is not yet configured.
		// Uses a static key — production must always have crypto initialized.
		h := hmac.New(sha256.New, []byte("llm-router-fallback-hmac-key-v1!"))
		h.Write([]byte(key))
		return hex.EncodeToString(h.Sum(nil))
	}

	// Use HMAC-SHA256 via crypto package (key stays internal)
	return hex.EncodeToString(crypto.HMACHash([]byte(key)))
}
