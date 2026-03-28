package user

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

const (
	// maxLoginAttempts is the number of failed login attempts before lockout.
	maxLoginAttempts = 5
	// loginLockoutWindow is the duration for which login attempts are tracked / locked out.
	loginLockoutWindow = 15 * time.Minute
)

// LoginLimiter enforces a maximum number of failed login attempts per email+IP
// combination. After 5 consecutive failures within 15 minutes, subsequent login
// attempts are rejected without checking credentials.
type LoginLimiter struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewLoginLimiter creates a login rate limiter. If redis is nil, all checks pass (fail-open).
func NewLoginLimiter(redis *redis.Client, logger *zap.Logger) *LoginLimiter {
	return &LoginLimiter{redis: redis, logger: logger}
}

// Check returns an error if the email+IP has exceeded the maximum allowed failed
// login attempts. Must be called BEFORE authentication.
func (l *LoginLimiter) Check(ctx context.Context, email, ip string) error {
	if l.redis == nil {
		return nil // No Redis → fail-open (don't block logins if infra is down)
	}

	key := loginKey(email, ip)
	count, err := l.redis.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		l.logger.Warn("login limiter: redis read error, allowing attempt", zap.Error(err))
		return nil // Redis error → fail-open
	}

	if count >= maxLoginAttempts {
		ttl, err := l.redis.TTL(ctx, key).Result()
		if err == nil && ttl > 0 {
			minutes := int(ttl.Minutes())
			if minutes < 1 {
				minutes = 1
			}
			return fmt.Errorf("too many failed login attempts, please try again in %d minutes", minutes)
		}
		return fmt.Errorf("too many failed login attempts, please try again later")
	}

	return nil
}

// RecordFailure increments the failed login counter for the email+IP.
func (l *LoginLimiter) RecordFailure(ctx context.Context, email, ip string) {
	if l.redis == nil {
		return
	}

	key := loginKey(email, ip)
	pipe := l.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginLockoutWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		l.logger.Warn("login limiter: redis write error", zap.Error(err))
	}
}

// ResetOnSuccess clears the failed login counter after a successful login.
func (l *LoginLimiter) ResetOnSuccess(ctx context.Context, email, ip string) {
	if l.redis == nil {
		return
	}

	key := loginKey(email, ip)
	l.redis.Del(ctx, key)
}

func loginKey(email, ip string) string {
	return fmt.Sprintf("login_fail:%s:%s", email, ip)
}
