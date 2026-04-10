package user

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestLimiter(t *testing.T) (*LoginLimiter, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewLoginLimiter(client, zap.NewNop())
	return limiter, mr
}

func TestLoginLimiter_AllowsFirstAttempt(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	defer mr.Close()

	err := limiter.Check(context.Background(), "user@test.com", "1.2.3.4")
	assert.NoError(t, err)
}

func TestLoginLimiter_BlocksAfterMaxAttempts(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	defer mr.Close()

	ctx := context.Background()
	email, ip := "user@test.com", "1.2.3.4"

	// Record maxLoginAttempts failures
	for i := 0; i < maxLoginAttempts; i++ {
		limiter.RecordFailure(ctx, email, ip)
	}

	// Next check should fail
	err := limiter.Check(ctx, email, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many")
}

func TestLoginLimiter_DifferentIPsAreIndependent(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	defer mr.Close()

	ctx := context.Background()
	email := "user@test.com"

	// Lock out IP 1
	for i := 0; i < maxLoginAttempts; i++ {
		limiter.RecordFailure(ctx, email, "1.2.3.4")
	}

	// IP 2 should still work
	err := limiter.Check(ctx, email, "5.6.7.8")
	assert.NoError(t, err)
}

func TestLoginLimiter_ResetOnSuccess(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	defer mr.Close()

	ctx := context.Background()
	email, ip := "user@test.com", "1.2.3.4"

	// Record some failures
	for i := 0; i < maxLoginAttempts-1; i++ {
		limiter.RecordFailure(ctx, email, ip)
	}

	// Successful login resets counter
	limiter.ResetOnSuccess(ctx, email, ip)

	// Should allow again
	err := limiter.Check(ctx, email, ip)
	assert.NoError(t, err)
}

func TestLoginLimiter_NilRedisPassesThrough(t *testing.T) {
	limiter := NewLoginLimiter(nil, zap.NewNop())

	err := limiter.Check(context.Background(), "user@test.com", "1.2.3.4")
	assert.NoError(t, err) // Nil Redis → fail-open

	// These should not panic
	limiter.RecordFailure(context.Background(), "user@test.com", "1.2.3.4")
	limiter.ResetOnSuccess(context.Background(), "user@test.com", "1.2.3.4")
}

func TestLoginLimiter_AllowsJustUnderLimit(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	defer mr.Close()

	ctx := context.Background()
	email, ip := "user@test.com", "1.2.3.4"

	// Record maxLoginAttempts - 1 failures
	for i := 0; i < maxLoginAttempts-1; i++ {
		limiter.RecordFailure(ctx, email, ip)
	}

	// Should still allow
	err := limiter.Check(ctx, email, ip)
	assert.NoError(t, err)
}

func TestLoginLimiter_ExpiresAfterWindow(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	defer mr.Close()

	ctx := context.Background()
	email, ip := "user@test.com", "1.2.3.4"

	for i := 0; i < maxLoginAttempts; i++ {
		limiter.RecordFailure(ctx, email, ip)
	}

	// Verify locked
	err := limiter.Check(ctx, email, ip)
	assert.Error(t, err)

	// Fast-forward time in miniredis
	mr.FastForward(loginLockoutWindow)

	// Should be unlocked
	err = limiter.Check(ctx, email, ip)
	assert.NoError(t, err)
}
