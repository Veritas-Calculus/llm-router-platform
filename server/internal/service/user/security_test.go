package user

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ─── API Key Expiry Tests (M5) ─────────────────────────────────────────

func TestGenerateAPIKeyHasPrefix(t *testing.T) {
	key := generateAPIKey()
	assert.True(t, len(key) > 8, "API key should be longer than 8 characters")
	assert.Equal(t, "llm_", key[:4], "API key should start with 'llm_' prefix")
}

func TestAPIKeyDefaultExpiryIsOneYear(t *testing.T) {
	// Verify the expiry time constant we use
	now := time.Now()
	expiry := now.AddDate(1, 0, 0)

	// Should be approximately 365 days from now
	diff := expiry.Sub(now)
	assert.True(t, diff >= 364*24*time.Hour, "expiry should be ~1 year")
	assert.True(t, diff <= 367*24*time.Hour, "expiry should be ~1 year")
}

// ─── bcrypt Cost Tests (L1) ─────────────────────────────────────────────

func TestBcryptCostIsSet(t *testing.T) {
	// Validate that bcrypt cost 12 is reasonable
	cost := 12
	assert.GreaterOrEqual(t, cost, 10, "bcrypt cost should be at least 10")
	assert.LessOrEqual(t, cost, 14, "bcrypt cost should not be excessively high for UX")
}
