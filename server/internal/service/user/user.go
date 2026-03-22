// Package user provides user and API key management services.
package user

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"

	"fmt"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	userRepo    *repository.UserRepository
	apiKeyRepo  *repository.APIKeyRepository
	projectRepo *repository.ProjectRepository
	orgRepo     *repository.OrganizationRepository
	logger      *zap.Logger
}
// NewService creates a new user service.
func NewService(
	userRepo *repository.UserRepository,
	apiKeyRepo *repository.APIKeyRepository,
	projectRepo *repository.ProjectRepository,
	orgRepo *repository.OrganizationRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		userRepo:    userRepo,
		apiKeyRepo:  apiKeyRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
		logger:      logger,
	}
}

// bcryptCost is the unified bcrypt cost factor used for all password hashing.
// Cost 12 provides strong brute-force resistance (~250ms per hash on modern hardware).
const bcryptCost = 12

// commonPasswords is a blocklist of frequently breached passwords (lowercase).
// Only includes passwords ≥8 chars that could pass character-class checks.
var commonPasswords = map[string]bool{
	"password1":  true, "password12": true, "password123": true,
	"qwerty123":  true, "qwertyui":  true, "qwerty1234": true,
	"abc12345":   true, "abcd1234":  true, "abcdef12": true,
	"welcome1":   true, "letmein1":  true, "trustno1": true,
	"iloveyou1":  true, "sunshine1": true, "princess1": true,
	"football1":  true, "baseball1": true, "dragon123": true,
	"master123":  true, "monkey123": true, "shadow123": true,
	"michael1":   true, "jennifer1": true, "charlie1": true,
	"admin123":   true, "login123":  true, "welcome123": true,
	"passw0rd1":  true, "p@ssword1": true, "p@ssw0rd1": true,
	"changeme1":  true, "test1234":  true, "guest1234": true,
	"12345678a":  true, "1234567890a": true, "123456789a": true,
	"Superman1":  true, "Computer1": true, "starwars1": true,
}

// ValidatePassword enforces minimum password strength requirements.
// Returns nil if valid, or a descriptive error.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	// Block top common passwords (case-insensitive)
	lower := strings.ToLower(password)
	if commonPasswords[lower] {
		return fmt.Errorf("password is too common, please choose a stronger password")
	}
	return nil
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, email, password, name string) (*models.User, error) {
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}

	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("registration failed") // generic to prevent user enumeration
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
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
		return nil, errors.New("invalid credentials") // Generic to prevent user enumeration
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

// GetByEmail retrieves a user by email.
func (s *Service) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepo.GetByEmail(ctx, email)
}

// GetByID retrieves a user by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

// ResetPassword resets a user's password using an ID (typically from a reset token).
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, newPass string) error {
	if err := ValidatePassword(newPass); err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPass), bcryptCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.RequirePasswordChange = false
	user.TokensInvalidatedAt = time.Now() // revoke all existing sessions

	return s.userRepo.Update(ctx, user)
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
	if err := ValidatePassword(newPass); err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPass)); err != nil {
		return errors.New("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPass), bcryptCost)
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

// CreateAPIKey generates a new API key for a project.
func (s *Service) CreateAPIKey(ctx context.Context, projectID uuid.UUID, name string, scopes string, rateLimit *int, tokenLimit *int) (*models.APIKey, string, error) {
	// Enforce max API key limit
	existing, err := s.apiKeyRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	if len(existing) >= MaxAPIKeysPerUser {
		return nil, "", errors.New("maximum number of API keys reached")
	}

	rawKey := generateAPIKey()
	hashedKey := hashAPIKey(rawKey)

	rl := 1000
	if rateLimit != nil {
		rl = *rateLimit
	}
	tl := 0
	if tokenLimit != nil {
		tl = *tokenLimit
	}

	apiKey := &models.APIKey{
		ProjectID:  projectID,
		KeyHash:    hashedKey,
		KeyPrefix:  rawKey[:8],
		Name:       name,
		IsActive:   true,
		Scopes:     scopes,
		RateLimit:  rl,
		TokenLimit: int64(tl),
		DailyLimit: 10000,
		ExpiresAt:  time.Now().AddDate(1, 0, 0), // M5: default 1-year expiry
	}

	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, "", err
	}

	return apiKey, rawKey, nil
}

// UpdateAPIKey updates an existing API key's settings.
func (s *Service) UpdateAPIKey(ctx context.Context, keyID uuid.UUID, name *string, scopes *string, rateLimit *int, tokenLimit *int, isActive *bool) (*models.APIKey, error) {
	key, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}

	if name != nil {
		key.Name = *name
	}
	if scopes != nil {
		key.Scopes = *scopes
	}
	if rateLimit != nil {
		key.RateLimit = *rateLimit
	}
	if tokenLimit != nil {
		key.TokenLimit = int64(*tokenLimit)
	}
	if isActive != nil {
		key.IsActive = *isActive
	}

	if err := s.apiKeyRepo.Update(ctx, key); err != nil {
		return nil, err
	}

	return key, nil
}

// GetAPIKeys returns all API keys for a project.
func (s *Service) GetAPIKeys(ctx context.Context, projectID uuid.UUID) ([]models.APIKey, error) {
	return s.apiKeyRepo.GetByProjectID(ctx, projectID)
}

// GetOrganizations returns all organizations a user has access to.
func (s *Service) GetOrganizations(ctx context.Context, userID uuid.UUID) ([]models.Organization, error) {
	return s.orgRepo.GetByUserID(ctx, userID)
}

// GetProjects returns all projects for an organization.
func (s *Service) GetProjects(ctx context.Context, orgID uuid.UUID) ([]models.Project, error) {
	return s.projectRepo.GetByOrgID(ctx, orgID)
}

// GetAllAPIKeys returns all API keys in the system (for admin view).
func (s *Service) GetAllAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	return s.apiKeyRepo.GetAll(ctx)
}

// ValidateAPIKey validates an API key and returns the associated project.
func (s *Service) ValidateAPIKey(ctx context.Context, rawKey string) (*models.Project, *models.APIKey, error) {
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

	project, err := s.projectRepo.GetByID(ctx, apiKey.ProjectID)
	if err != nil {
		return nil, nil, err
	}

	// Wait, does the project have an IsActive field? If not, we skip.
	// We might need to check if organization is active or not later.

	now := time.Now()
	apiKey.LastUsedAt = now
	_ = s.apiKeyRepo.Update(ctx, apiKey)

	return project, apiKey, nil
}

// RevokeAPIKey deactivates an API key.
func (s *Service) RevokeAPIKey(ctx context.Context, projectID, keyID uuid.UUID) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return err
	}

	if apiKey.ProjectID != projectID {
		return errors.New("unauthorized")
	}

	apiKey.IsActive = false
	return s.apiKeyRepo.Update(ctx, apiKey)
}

// DeleteAPIKey permanently removes an API key from the database.
func (s *Service) DeleteAPIKey(ctx context.Context, projectID, keyID uuid.UUID) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return err
	}

	if apiKey.ProjectID != projectID {
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
	if !crypto.IsInitialized() {
		// This should never be reached after MustInitialize at startup.
		// Hard-fail to prevent keys being hashed with a predictable key.
		panic("hashAPIKey called before crypto initialization")
	}

	// Use HMAC-SHA256 via crypto package (key stays internal)
	return hex.EncodeToString(crypto.HMACHash([]byte(key)))
}

// GenerateMfaSecret creates a new TOTP secret for the user.
func (s *Service) GenerateMfaSecret(ctx context.Context, userID uuid.UUID, email string) (*models.MfaSecretInfo, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Veritas Calculus",
		AccountName: email,
		Period:      30,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate MFA secret: %w", err)
	}

	backupCodes := []string{}
	for i := 0; i < 8; i++ {
		// Generate 8 character random string for backup code
		b := make([]byte, 4)
		cryptorand.Read(b)
		code := fmt.Sprintf("%04x-%04x", b[0:2], b[2:4])
		backupCodes = append(backupCodes, code)
	}
	backupCodesJSON, _ := json.Marshal(backupCodes)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.MfaSecret = key.Secret()
	user.MfaBackupCodes = string(backupCodesJSON)
	user.MfaEnabled = false // Still false until verified

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return &models.MfaSecretInfo{
		Secret:      key.Secret(),
		QrCodeUrl:   key.URL(),
		BackupCodes: backupCodes,
	}, nil
}

// VerifyAndEnableMfa verifies a TOTP code and turns on MFA.
func (s *Service) VerifyAndEnableMfa(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user.MfaSecret == "" {
		return false, fmt.Errorf("MFA secret not generated")
	}

	valid := totp.Validate(code, user.MfaSecret)
	if !valid {
		return false, fmt.Errorf("invalid verification code")
	}

	user.MfaEnabled = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return false, err
	}

	s.logger.Info("user enabled MFA", zap.String("user_id", userID.String()))
	return true, nil
}

// DisableMfa verifies a TOTP code or backup code and turns off MFA.
func (s *Service) DisableMfa(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if !user.MfaEnabled {
		return false, fmt.Errorf("MFA is not enabled")
	}

	valid := totp.Validate(code, user.MfaSecret)

	// Check backup codes if TOTP fails
	if !valid && user.MfaBackupCodes != "" {
		var backupCodes []string
		if err := json.Unmarshal([]byte(user.MfaBackupCodes), &backupCodes); err == nil {
			for i, bc := range backupCodes {
				if bc == code || strings.ReplaceAll(bc, "-", "") == code {
					valid = true
					// Remove the used backup code
					backupCodes = append(backupCodes[:i], backupCodes[i+1:]...)
					bJSON, _ := json.Marshal(backupCodes)
					user.MfaBackupCodes = string(bJSON)
					break
				}
			}
		}
	}

	if !valid {
		return false, fmt.Errorf("invalid verification code")
	}

	user.MfaEnabled = false
	user.MfaSecret = ""
	user.MfaBackupCodes = ""
	if err := s.userRepo.Update(ctx, user); err != nil {
		return false, err
	}

	s.logger.Info("user disabled MFA", zap.String("user_id", userID.String()))
	return true, nil
}

// UpdateProject updates an existing Project's properties including quota limits and IP whitelists.
// A caller must evaluate permission scopes independently before yielding.
func (s *Service) UpdateProject(ctx context.Context, id uuid.UUID, updateData *models.Project) (*models.Project, error) {
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("project %s not found: %w", id, err)
	}

	if updateData.Name != "" {
		project.Name = updateData.Name
	}
	// Note: Description, QuotaLimit, WhiteListedIps are allowed to be empty or zero
	// If explicit struct field assignments are required, the caller must assign them
	project.Description = updateData.Description
	project.QuotaLimit = updateData.QuotaLimit
	project.WhiteListedIps = updateData.WhiteListedIps

	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}
	return project, nil
}
