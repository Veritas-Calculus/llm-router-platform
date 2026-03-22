package dlp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/dlp"
)

func TestScrubText(t *testing.T) {
	config := &models.DlpConfig{
		IsEnabled:       true,
		Strategy:        dlp.StrategyRedact,
		MaskEmails:      true,
		MaskPhones:      true,
		MaskCreditCards: true,
		MaskSSN:         true,
		MaskApiKeys:     true,
		CustomRegex:     []string{`(?i)top\s*secret`, `\bCLASSIFIED\b`},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no pii",
			input:    "Hello world, how are you?",
			expected: "Hello world, how are you?",
		},
		{
			name:     "email redaction",
			input:    "Contact me at admin@example.com for details.",
			expected: "Contact me at [EMAIL_REDACTED] for details.",
		},
		{
			name:     "phone redaction",
			input:    "Call 123-456-7890 if you have issues.",
			expected: "Call [PHONE_REDACTED] if you have issues.",
		},
		{
			name:     "credit card redaction",
			input:    "My card is 4111 1111 1111 1111 and expiry is 12/25",
			expected: "My card is [CREDIT_CARD_REDACTED] and expiry is 12/25",
		},
		{
			name:     "custom regex",
			input:    "This project is TOP SECRET and highly CLASSIFIED.",
			expected: "This project is [CUSTOM_REDACTED] and highly [CUSTOM_REDACTED].",
		},
		{
			name:     "api key redaction",
			input:    "My openai key is sk-1234567890abcdef1234567890abcdef1234567890abcdef here",
			expected: "My openai key is [API_KEY_REDACTED] here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dlp.ScrubText(tt.input, config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasPII(t *testing.T) {
	config := &models.DlpConfig{
		IsEnabled:       true,
		Strategy:        dlp.StrategyBlock,
		MaskEmails:      true,
		CustomRegex:     []string{`[bB]ad[wW]ord`},
	}

	assert.True(t, dlp.HasPII("Here is my email test@test.com", config))
	assert.True(t, dlp.HasPII("This text contains badword!", config))
	assert.False(t, dlp.HasPII("This text is clean.", config))

	config.IsEnabled = false
	assert.False(t, dlp.HasPII("Here is my email test@test.com", config))
}
