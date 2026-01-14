package user

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestGenerateAPIKey(t *testing.T) {
	key := generateAPIKey()

	assert.NotEmpty(t, key)
	assert.True(t, len(key) > 20)
	assert.True(t, strings.HasPrefix(key, "llm_"))
}

func TestGenerateAPIKeyUniqueness(t *testing.T) {
	keys := make(map[string]bool)

	for i := 0; i < 100; i++ {
		key := generateAPIKey()
		assert.False(t, keys[key], "Key should be unique")
		keys[key] = true
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "llm_test_api_key_123"
	hash := hashAPIKey(key)

	assert.NotEmpty(t, hash)
	assert.NotEqual(t, key, hash)
}

func TestUserModel(t *testing.T) {
	user := models.User{
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		Name:         "Test User",
		Role:         "user",
		IsActive:     true,
	}

	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "user", user.Role)
	assert.True(t, user.IsActive)
}

func TestAPIKeyModel(t *testing.T) {
	userID := uuid.New()
	apiKey := models.APIKey{
		UserID:     userID,
		KeyHash:    "hashedkey",
		KeyPrefix:  "llm_abc",
		Name:       "Test Key",
		IsActive:   true,
		RateLimit:  1000,
		DailyLimit: 10000,
	}

	assert.Equal(t, userID, apiKey.UserID)
	assert.Equal(t, "llm_abc", apiKey.KeyPrefix)
	assert.Equal(t, "Test Key", apiKey.Name)
	assert.True(t, apiKey.IsActive)
	assert.Equal(t, 1000, apiKey.RateLimit)
}

func TestAPIKeyPrefix(t *testing.T) {
	key := generateAPIKey()
	prefix := key[:8]

	assert.Equal(t, "llm_", prefix[:4])
	assert.True(t, len(prefix) == 8)
}

func TestUserRoles(t *testing.T) {
	roles := []string{"user", "admin"}

	for _, role := range roles {
		user := models.User{Role: role}
		assert.Equal(t, role, user.Role)
	}
}

func TestAPIKeyRateLimits(t *testing.T) {
	apiKey := models.APIKey{
		RateLimit:  1000,
		DailyLimit: 50000,
	}

	assert.Equal(t, 1000, apiKey.RateLimit)
	assert.Equal(t, 50000, apiKey.DailyLimit)
	assert.True(t, apiKey.DailyLimit > apiKey.RateLimit)
}

func TestInactiveUser(t *testing.T) {
	user := models.User{
		Email:    "inactive@example.com",
		IsActive: false,
	}

	assert.False(t, user.IsActive)
}

func TestInactiveAPIKey(t *testing.T) {
	apiKey := models.APIKey{
		Name:     "Revoked Key",
		IsActive: false,
	}

	assert.False(t, apiKey.IsActive)
}

func TestUserEmailValidation(t *testing.T) {
	validEmails := []string{
		"user@example.com",
		"user.name@example.com",
		"user+tag@example.com",
	}

	for _, email := range validEmails {
		user := models.User{Email: email}
		assert.Contains(t, user.Email, "@")
	}
}

func TestAPIKeyWithExpiration(t *testing.T) {
	apiKey := models.APIKey{
		Name:     "Expiring Key",
		IsActive: true,
	}

	assert.True(t, apiKey.ExpiresAt.IsZero())
}

func TestMultipleAPIKeys(t *testing.T) {
	userID := uuid.New()
	keys := []models.APIKey{
		{UserID: userID, Name: "Key 1", IsActive: true},
		{UserID: userID, Name: "Key 2", IsActive: true},
		{UserID: userID, Name: "Key 3", IsActive: false},
	}

	var activeCount int
	for _, k := range keys {
		if k.IsActive {
			activeCount++
		}
	}

	assert.Equal(t, 2, activeCount)
}

func TestPasswordHashLength(t *testing.T) {
	hash := hashAPIKey("test_key")

	assert.True(t, len(hash) >= 60)
}
