package user

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// maxLoginAttempts is the (email,ip) hard-lockout threshold. After 5
	// failures within loginLockoutWindow the request is refused outright.
	maxLoginAttempts = 5
	// loginCaptchaThreshold is the per-email soft threshold that flips
	// "this login attempt requires a CAPTCHA solve" — independent of IP, so
	// a botnet rotating residential IPs can't keep guessing without solving
	// CAPTCHA after a few tries against a single account.
	loginCaptchaThreshold = 3
	// loginLockoutWindow is the rolling window for both counters.
	loginLockoutWindow = 15 * time.Minute
)

// LoginLimiter enforces two complementary brute-force defenses:
//
//   - (email, ip) hard lockout after 5 failures in 15min. This is precise:
//     a legitimate user on a single browser can't trigger global lockout for
//     other users.
//   - per-email soft trigger after 3 failures in 15min. This survives an
//     attacker rotating IPs because the counter is IP-independent. The
//     handler must require CAPTCHA verification once this is hit, even if
//     CAPTCHA is otherwise disabled.
type LoginLimiter struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewLoginLimiter creates a login rate limiter. If redis is nil, all checks pass (fail-open).
func NewLoginLimiter(redis *redis.Client, logger *zap.Logger) *LoginLimiter {
	return &LoginLimiter{redis: redis, logger: logger}
}

// Check returns an error if the email+IP has exceeded the hard lockout
// threshold. Must be called BEFORE authentication.
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

// RequireCaptcha returns true if the per-email failure counter has crossed
// loginCaptchaThreshold within the lockout window. The login handler must
// then force CAPTCHA verification even if CAPTCHA is otherwise disabled.
// Returns false when Redis is unavailable so a Redis outage doesn't lock
// every user out of password login.
func (l *LoginLimiter) RequireCaptcha(ctx context.Context, email string) bool {
	if l.redis == nil {
		return false
	}
	count, err := l.redis.Get(ctx, perEmailKey(email)).Int()
	if err != nil && err != redis.Nil {
		l.logger.Warn("login limiter: per-email redis read error, allowing attempt", zap.Error(err))
		return false
	}
	return count >= loginCaptchaThreshold
}

// RecordFailure increments both the per-(email,ip) counter and the per-email
// counter. Either path bumps both because an attacker that rotates IPs would
// otherwise reset the per-(email,ip) counter to 0 on every new IP.
func (l *LoginLimiter) RecordFailure(ctx context.Context, email, ip string) {
	if l.redis == nil {
		return
	}

	pipe := l.redis.Pipeline()
	ipKey := loginKey(email, ip)
	pipe.Incr(ctx, ipKey)
	pipe.Expire(ctx, ipKey, loginLockoutWindow)
	emailK := perEmailKey(email)
	pipe.Incr(ctx, emailK)
	pipe.Expire(ctx, emailK, loginLockoutWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		l.logger.Warn("login limiter: redis write error", zap.Error(err))
	}
}

// ResetOnSuccess clears both counters after a successful login.
func (l *LoginLimiter) ResetOnSuccess(ctx context.Context, email, ip string) {
	if l.redis == nil {
		return
	}
	l.redis.Del(ctx, loginKey(email, ip), perEmailKey(email))
}

func loginKey(email, ip string) string {
	return fmt.Sprintf("login_fail:%s:%s", email, ip)
}

func perEmailKey(email string) string {
	return fmt.Sprintf("login_fail_email:%s", email)
}
