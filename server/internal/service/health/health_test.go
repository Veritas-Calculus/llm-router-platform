package health

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestAPIKeyHealthStatus(t *testing.T) {
	health := APIKeyHealthStatus{
		ID:           uuid.New(),
		ProviderName: "openai",
		KeyPrefix:    "sk-xxx",
		IsActive:     true,
		IsHealthy:    true,
		LastCheck:    time.Now(),
		ResponseTime: 150,
		SuccessRate:  99.5,
	}

	assert.True(t, health.IsHealthy)
	assert.Equal(t, "openai", health.ProviderName)
	assert.Equal(t, int64(150), health.ResponseTime)
	assert.True(t, health.IsActive)
}

func TestAPIKeyHealthStatusUnhealthy(t *testing.T) {
	health := APIKeyHealthStatus{
		ID:           uuid.New(),
		ProviderName: "openai",
		IsHealthy:    false,
		SuccessRate:  50.0,
	}

	assert.False(t, health.IsHealthy)
	assert.InDelta(t, 50.0, health.SuccessRate, 0.1)
}

func TestProxyHealthStatus(t *testing.T) {
	health := ProxyHealthStatus{
		ID:           uuid.New(),
		URL:          "http://proxy.example.com:8080",
		Type:         "http",
		Region:       "us-east-1",
		IsActive:     true,
		IsHealthy:    true,
		ResponseTime: 100,
		LastCheck:    time.Now(),
		SuccessRate:  98.0,
	}

	assert.True(t, health.IsHealthy)
	assert.Equal(t, int64(100), health.ResponseTime)
	assert.Contains(t, health.URL, "proxy.example.com")
}

func TestProxyHealthStatusUnhealthy(t *testing.T) {
	health := ProxyHealthStatus{
		ID:        uuid.New(),
		URL:       "http://proxy.example.com:8080",
		IsHealthy: false,
	}

	assert.False(t, health.IsHealthy)
}

func TestHealthHistoryModel(t *testing.T) {
	entry := models.HealthHistory{
		TargetType:   "api_key",
		TargetID:     uuid.New(),
		IsHealthy:    true,
		ResponseTime: 150,
		CheckedAt:    time.Now(),
	}

	assert.Equal(t, "api_key", entry.TargetType)
	assert.True(t, entry.IsHealthy)
	assert.Equal(t, int64(150), entry.ResponseTime)
}

func TestHealthHistoryWithError(t *testing.T) {
	entry := models.HealthHistory{
		TargetType:   "proxy",
		TargetID:     uuid.New(),
		IsHealthy:    false,
		ResponseTime: 0,
		ErrorMessage: "Connection refused",
		CheckedAt:    time.Now(),
	}

	assert.False(t, entry.IsHealthy)
	assert.Equal(t, "Connection refused", entry.ErrorMessage)
}

func TestAlertModel(t *testing.T) {
	alert := models.Alert{
		TargetType: "api_key",
		TargetID:   uuid.New(),
		AlertType:  "health_check_failed",
		Message:    "API key health check failed 3 times",
		Status:     "active",
	}

	assert.Equal(t, "api_key", alert.TargetType)
	assert.Equal(t, "health_check_failed", alert.AlertType)
	assert.Equal(t, "active", alert.Status)
}

func TestAlertConfigModel(t *testing.T) {
	config := models.AlertConfig{
		TargetType:       "proxy",
		TargetID:         uuid.New(),
		IsEnabled:        true,
		FailureThreshold: 3,
		WebhookURL:       "https://webhook.example.com/alert",
		Email:            "admin@example.com",
	}

	assert.True(t, config.IsEnabled)
	assert.Equal(t, 3, config.FailureThreshold)
	assert.NotEmpty(t, config.WebhookURL)
}

func TestHealthCheckInterval(t *testing.T) {
	interval := 60 * time.Second

	assert.True(t, interval >= 10*time.Second)
	assert.True(t, interval <= 5*time.Minute)
}

func TestHealthThreshold(t *testing.T) {
	threshold := 3
	failures := 0

	for i := 0; i < 5; i++ {
		failures++
		if failures >= threshold {
			break
		}
	}

	assert.Equal(t, 3, failures)
}

func TestLatencyThreshold(t *testing.T) {
	latencyThreshold := int64(1000)
	latencies := []int64{100, 250, 500, 1200, 800}

	var slowChecks int
	for _, l := range latencies {
		if l > latencyThreshold {
			slowChecks++
		}
	}

	assert.Equal(t, 1, slowChecks)
}

func TestHealthAggregation(t *testing.T) {
	statuses := []APIKeyHealthStatus{
		{IsHealthy: true},
		{IsHealthy: true},
		{IsHealthy: false},
		{IsHealthy: true},
	}

	var healthyCount, unhealthyCount int
	for _, h := range statuses {
		if h.IsHealthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	assert.Equal(t, 3, healthyCount)
	assert.Equal(t, 1, unhealthyCount)
}

func TestAlertStatusTransitions(t *testing.T) {
	validTransitions := map[string][]string{
		"active":       {"acknowledged", "resolved"},
		"acknowledged": {"resolved"},
		"resolved":     {},
	}

	alert := models.Alert{Status: "active"}

	nextStates := validTransitions[alert.Status]
	assert.Contains(t, nextStates, "acknowledged")

	alert.Status = "acknowledged"
	nextStates = validTransitions[alert.Status]
	assert.Contains(t, nextStates, "resolved")
	assert.NotContains(t, nextStates, "active")

	alert.Status = "resolved"
	nextStates = validTransitions[alert.Status]
	assert.Len(t, nextStates, 0)
}
