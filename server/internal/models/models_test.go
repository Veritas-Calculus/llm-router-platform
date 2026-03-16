package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUserModel(t *testing.T) {
	user := User{
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
	apiKey := APIKey{
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

func TestProviderModel(t *testing.T) {
	provider := Provider{
		Name:       "openai",
		BaseURL:    "https://api.openai.com/v1",
		IsActive:   true,
		Priority:   10,
		Weight:     1.0,
		MaxRetries: 3,
		Timeout:    30,
	}

	assert.Equal(t, "openai", provider.Name)
	assert.Equal(t, "https://api.openai.com/v1", provider.BaseURL)
	assert.True(t, provider.IsActive)
	assert.Equal(t, 10, provider.Priority)
}

func TestModelModel(t *testing.T) {
	providerID := uuid.New()
	model := Model{
		ProviderID:       providerID,
		Name:             "gpt-4",
		DisplayName:      "GPT-4",
		InputPricePer1K:  0.03,
		OutputPricePer1K: 0.06,
		MaxTokens:        8192,
		IsActive:         true,
	}

	assert.Equal(t, providerID, model.ProviderID)
	assert.Equal(t, "gpt-4", model.Name)
	assert.Equal(t, "GPT-4", model.DisplayName)
	assert.Equal(t, 0.03, model.InputPricePer1K)
}

func TestProxyModel(t *testing.T) {
	proxy := Proxy{
		URL:      "http://proxy.example.com:8080",
		Type:     "http",
		Region:   "us-east-1",
		IsActive: true,
		Weight:   1.0,
	}

	assert.Equal(t, "http://proxy.example.com:8080", proxy.URL)
	assert.Equal(t, "http", proxy.Type)
	assert.Equal(t, "us-east-1", proxy.Region)
	assert.True(t, proxy.IsActive)
}

func TestUsageLogModel(t *testing.T) {
	log := UsageLog{
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

	assert.Equal(t, 100, log.RequestTokens)
	assert.Equal(t, 200, log.ResponseTokens)
	assert.Equal(t, 300, log.TotalTokens)
	assert.Equal(t, 200, log.StatusCode)
}

func TestHealthHistoryModel(t *testing.T) {
	history := HealthHistory{
		TargetType:   "api_key",
		TargetID:     uuid.New(),
		IsHealthy:    true,
		ResponseTime: 150,
		CheckedAt:    time.Now(),
	}

	assert.Equal(t, "api_key", history.TargetType)
	assert.True(t, history.IsHealthy)
	assert.Equal(t, int64(150), history.ResponseTime)
}

func TestAlertModel(t *testing.T) {
	alert := Alert{
		TargetType: "proxy",
		TargetID:   uuid.New(),
		AlertType:  "health_check_failed",
		Message:    "Proxy unreachable",
		Status:     "active",
	}

	assert.Equal(t, "proxy", alert.TargetType)
	assert.Equal(t, "health_check_failed", alert.AlertType)
	assert.Equal(t, "active", alert.Status)
}

func TestAlertConfigModel(t *testing.T) {
	config := AlertConfig{
		TargetType:       "api_key",
		TargetID:         uuid.New(),
		IsEnabled:        true,
		FailureThreshold: 3,
		WebhookURL:       "https://webhook.example.com",
		Email:            "admin@example.com",
	}

	assert.Equal(t, "api_key", config.TargetType)
	assert.True(t, config.IsEnabled)
	assert.Equal(t, 3, config.FailureThreshold)
}

func TestModelMultiDimensionalPricing(t *testing.T) {
	providerID := uuid.New()

	// TTS model with per-second pricing
	ttsModel := Model{
		ProviderID:      providerID,
		Name:            "tts-1",
		DisplayName:     "TTS-1",
		PricePerSecond:  0.015,
		MaxTokens:       0,
		IsActive:        true,
	}
	assert.Equal(t, "tts-1", ttsModel.Name)
	assert.Equal(t, 0.015, ttsModel.PricePerSecond)
	assert.Equal(t, 0.0, ttsModel.InputPricePer1K)

	// Image model with per-image pricing
	imageModel := Model{
		ProviderID:    providerID,
		Name:          "dall-e-3",
		DisplayName:   "DALL-E 3",
		PricePerImage: 0.04,
		IsActive:      true,
	}
	assert.Equal(t, 0.04, imageModel.PricePerImage)
	assert.Equal(t, 0.0, imageModel.PricePerSecond)

	// Video model with per-minute pricing
	videoModel := Model{
		ProviderID:     providerID,
		Name:           "gemini-2.0-flash",
		DisplayName:    "Gemini 2.0 Flash",
		PricePerMinute: 0.05,
		IsActive:       true,
	}
	assert.Equal(t, 0.05, videoModel.PricePerMinute)
}

func TestUsageLogExtendedFields(t *testing.T) {
	// TTS usage log with duration
	ttsLog := UsageLog{
		UserID:     uuid.New(),
		APIKeyID:   uuid.New(),
		ProviderID: uuid.New(),
		ModelName:  "tts-1",
		DurationMs: 15000, // 15 seconds of audio
		Cost:       0.225, // 15 * 0.015
		StatusCode: 200,
	}
	assert.Equal(t, int64(15000), ttsLog.DurationMs)
	assert.Equal(t, 0, ttsLog.RequestTokens)
	assert.Equal(t, 0, ttsLog.TotalTokens)

	// Image generation usage log with item count
	imageLog := UsageLog{
		UserID:     uuid.New(),
		APIKeyID:   uuid.New(),
		ProviderID: uuid.New(),
		ModelName:  "dall-e-3",
		ItemCount:  2,
		Cost:       0.08, // 2 * 0.04
		StatusCode: 200,
	}
	assert.Equal(t, 2, imageLog.ItemCount)

	// Video analysis usage log with bytes processed
	videoLog := UsageLog{
		UserID:         uuid.New(),
		APIKeyID:       uuid.New(),
		ProviderID:     uuid.New(),
		ModelName:      "gemini-2.0-flash",
		BytesProcessed: 52428800, // 50 MB
		DurationMs:     120000,   // 2 minutes
		Cost:           0.10,
		StatusCode:     200,
	}
	assert.Equal(t, int64(52428800), videoLog.BytesProcessed)
	assert.Equal(t, int64(120000), videoLog.DurationMs)
}

func TestAsyncTaskModel(t *testing.T) {
	task := AsyncTask{
		UserID:     uuid.New(),
		Type:       "batch_tts",
		Status:     "pending",
		Input:      `[{"text":"hello"},{"text":"world"}]`,
		WebhookURL: "https://example.com/callback",
		Progress:   0,
	}

	assert.Equal(t, "batch_tts", task.Type)
	assert.Equal(t, "pending", task.Status)
	assert.Equal(t, 0, task.Progress)
	assert.Contains(t, task.Input, "hello")
	assert.NotEmpty(t, task.WebhookURL)
	assert.Nil(t, task.CompletedAt)
	assert.Empty(t, task.Error)
	assert.Empty(t, task.Result)
}

func TestAsyncTaskStatusTransitions(t *testing.T) {
	now := time.Now()

	// Completed task
	completed := AsyncTask{
		UserID:      uuid.New(),
		Type:        "tts",
		Status:      "completed",
		Input:       `{"text":"test"}`,
		Result:      `{"audio_url":"https://cdn.example.com/output.mp3"}`,
		Progress:    100,
		CompletedAt: &now,
	}
	assert.Equal(t, "completed", completed.Status)
	assert.Equal(t, 100, completed.Progress)
	assert.NotNil(t, completed.CompletedAt)
	assert.Contains(t, completed.Result, "audio_url")

	// Failed task
	failed := AsyncTask{
		UserID:      uuid.New(),
		Type:        "video_analysis",
		Status:      "failed",
		Input:       `{"video_url":"https://example.com/video.mp4"}`,
		Error:       "provider timeout after 30s",
		Progress:    45,
		CompletedAt: &now,
	}
	assert.Equal(t, "failed", failed.Status)
	assert.Equal(t, 45, failed.Progress)
	assert.Contains(t, failed.Error, "timeout")
}


