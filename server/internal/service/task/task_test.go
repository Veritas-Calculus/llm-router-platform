package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := NewService(nil, logger)
	assert.NotNil(t, svc)
}

func TestWebhookPayload(t *testing.T) {
	payload := webhookPayload{
		TaskID:   "test-id",
		Type:     "tts",
		Status:   "completed",
		Progress: 100,
		Result:   `{"audio_url":"https://example.com/output.mp3"}`,
	}

	assert.Equal(t, "test-id", payload.TaskID)
	assert.Equal(t, "tts", payload.Type)
	assert.Equal(t, "completed", payload.Status)
	assert.Equal(t, 100, payload.Progress)
	assert.Contains(t, payload.Result, "audio_url")
}

func TestWebhookPayloadFailed(t *testing.T) {
	payload := webhookPayload{
		TaskID:   "test-id-2",
		Type:     "batch_image",
		Status:   "failed",
		Progress: 50,
		Error:    "provider timeout",
	}

	assert.Equal(t, "failed", payload.Status)
	assert.Equal(t, "provider timeout", payload.Error)
	assert.Empty(t, payload.Result)
}
