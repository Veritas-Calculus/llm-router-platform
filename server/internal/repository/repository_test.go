package repository

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestUserModelFields(t *testing.T) {
	user := &models.User{
		Email:        "test@example.com",
		PasswordHash: "hashed",
		Name:         "Test User",
		Role:         "user",
		IsActive:     true,
	}

	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "user", user.Role)
	assert.True(t, user.IsActive)
}

func TestAPIKeyModelFields(t *testing.T) {
	userID := uuid.New()
	apiKey := &models.APIKey{
		UserID:     userID,
		KeyHash:    "hashed-key",
		KeyPrefix:  "llm_abc",
		Name:       "Test Key",
		IsActive:   true,
		RateLimit:  1000,
		DailyLimit: 10000,
	}

	assert.Equal(t, userID, apiKey.UserID)
	assert.Equal(t, "llm_abc", apiKey.KeyPrefix)
	assert.Equal(t, 1000, apiKey.RateLimit)
}

func TestProviderModelFields(t *testing.T) {
	provider := &models.Provider{
		Name:       "openai",
		BaseURL:    "https://api.openai.com/v1",
		IsActive:   true,
		Priority:   10,
		Weight:     1.0,
		MaxRetries: 3,
		Timeout:    30,
	}

	assert.Equal(t, "openai", provider.Name)
	assert.True(t, provider.IsActive)
	assert.Equal(t, 10, provider.Priority)
}

func TestProviderAPIKeyModelFields(t *testing.T) {
	providerID := uuid.New()
	key := &models.ProviderAPIKey{
		ProviderID:      providerID,
		EncryptedAPIKey: "encrypted",
		KeyPrefix:       "sk-xxx",
		IsActive:        true,
		UsageCount:      100,
	}

	assert.Equal(t, providerID, key.ProviderID)
	assert.True(t, key.IsActive)
	assert.Equal(t, int64(100), key.UsageCount)
}

func TestModelModelFields(t *testing.T) {
	providerID := uuid.New()
	model := &models.Model{
		ProviderID:       providerID,
		Name:             "gpt-4",
		DisplayName:      "GPT-4",
		InputPricePer1K:  0.03,
		OutputPricePer1K: 0.06,
		MaxTokens:        8192,
		IsActive:         true,
	}

	assert.Equal(t, "gpt-4", model.Name)
	assert.Equal(t, 8192, model.MaxTokens)
}

func TestProxyModelFields(t *testing.T) {
	proxy := &models.Proxy{
		URL:      "http://proxy.example.com:8080",
		Type:     "http",
		Username: "user",
		Password: "pass",
		Region:   "us-east-1",
		IsActive: true,
		Weight:   1.0,
	}

	assert.Equal(t, "http", proxy.Type)
	assert.Equal(t, "us-east-1", proxy.Region)
}

func TestUsageLogModelFields(t *testing.T) {
	log := &models.UsageLog{
		UserID:         uuid.New(),
		APIKeyID:       uuid.New(),
		ProviderID:     uuid.New(),
		RequestTokens:  100,
		ResponseTokens: 200,
		TotalTokens:    300,
		Cost:           0.01,
		Latency:        500,
		StatusCode:     200,
	}

	assert.Equal(t, 300, log.TotalTokens)
	assert.Equal(t, 200, log.StatusCode)
}

func TestHealthHistoryModelFields(t *testing.T) {
	history := &models.HealthHistory{
		TargetType:   "api_key",
		TargetID:     uuid.New(),
		IsHealthy:    true,
		ResponseTime: 150,
	}

	assert.Equal(t, "api_key", history.TargetType)
	assert.True(t, history.IsHealthy)
}

func TestAlertModelFields(t *testing.T) {
	alert := &models.Alert{
		TargetType: "proxy",
		TargetID:   uuid.New(),
		AlertType:  "health_check_failed",
		Message:    "Proxy unreachable",
		Status:     "active",
	}

	assert.Equal(t, "health_check_failed", alert.AlertType)
	assert.Equal(t, "active", alert.Status)
}

func TestAlertConfigModelFields(t *testing.T) {
	config := &models.AlertConfig{
		TargetType:       "api_key",
		TargetID:         uuid.New(),
		IsEnabled:        true,
		FailureThreshold: 3,
		WebhookURL:       "https://webhook.example.com",
		Email:            "admin@example.com",
	}

	assert.True(t, config.IsEnabled)
	assert.Equal(t, 3, config.FailureThreshold)
}

func TestConversationMemoryModelFields(t *testing.T) {
	memory := &models.ConversationMemory{
		UserID:         uuid.New(),
		ConversationID: "conv-123",
		Role:           "user",
		Content:        "Hello",
		TokenCount:     5,
		Sequence:       1,
	}

	assert.Equal(t, "conv-123", memory.ConversationID)
	assert.Equal(t, "user", memory.Role)
	assert.Equal(t, 1, memory.Sequence)
}
