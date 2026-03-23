package user

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestEmailVerificationToken_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		token  models.EmailVerificationToken
		expect bool
	}{
		{
			name: "valid token",
			token: models.EmailVerificationToken{
				ExpiresAt: time.Now().Add(1 * time.Hour),
				UsedAt:    nil,
			},
			expect: true,
		},
		{
			name: "expired token",
			token: models.EmailVerificationToken{
				ExpiresAt: time.Now().Add(-1 * time.Hour),
				UsedAt:    nil,
			},
			expect: false,
		},
		{
			name: "used token",
			token: models.EmailVerificationToken{
				ExpiresAt: time.Now().Add(1 * time.Hour),
				UsedAt:    ptrTime(time.Now()),
			},
			expect: false,
		},
		{
			name: "used and expired token",
			token: models.EmailVerificationToken{
				ExpiresAt: time.Now().Add(-1 * time.Hour),
				UsedAt:    ptrTime(time.Now()),
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.token.IsValid())
		})
	}
}

func TestEmailVerificationToken_Fields(t *testing.T) {
	userID := uuid.New()
	token := models.EmailVerificationToken{
		UserID:    userID,
		TokenHash: "abc123hash",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	assert.Equal(t, userID, token.UserID)
	assert.Equal(t, "abc123hash", token.TokenHash)
	assert.True(t, token.IsValid())
}

func TestOnboardAccountParams_DefaultCredit(t *testing.T) {
	params := OnboardAccountParams{
		GrantWelcomeCredit: true,
	}
	assert.True(t, params.GrantWelcomeCredit)
	assert.Equal(t, float64(0), params.WelcomeCreditUSD) // zero means service uses default 5.0
}

func TestOnboardAccountParams_CustomCredit(t *testing.T) {
	params := OnboardAccountParams{
		GrantWelcomeCredit: true,
		WelcomeCreditUSD:   10.0,
	}
	assert.Equal(t, 10.0, params.WelcomeCreditUSD)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
