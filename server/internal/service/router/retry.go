// Package router provides LLM request routing logic.
// This file implements exponential backoff retry for transient errors.
package router

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// ─── Retry Configuration ───────────────────────────────────────────────────

// RetryConfig holds tunable parameters for the retry mechanism.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries).
	MaxRetries int
	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential growth.
	MaxBackoff time.Duration
	// BackoffMultiplier is the exponential factor (typically 2).
	BackoffMultiplier float64
}

// DefaultRetryConfig returns sensible defaults for LLM API retries.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        8 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// ─── Prometheus Metrics ────────────────────────────────────────────────────

var (
	retryAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "retry_attempts_total",
			Help:      "Total retry attempts for transient errors.",
		},
		[]string{"provider", "result"}, // result: "success" | "exhausted"
	)
)

// ─── Retry Logic ───────────────────────────────────────────────────────────

// isRetryableError checks if an error is transient and worth retrying.
// Retries on: 429 (rate limit), 502 (bad gateway), 503 (service unavailable),
// 504 (gateway timeout), connection refused, and deadline exceeded.
func isRetryableError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	retryableKeywords := []string{
		"429", "too many requests",
		"502", "bad gateway",
		"503", "service unavailable",
		"504", "gateway timeout",
		"connection refused", "connection reset",
		"deadline exceeded", "timeout",
		"temporary failure",
	}
	for _, kw := range retryableKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// executeWithRetry wraps a function call with exponential backoff retry.
// It only retries on transient errors (as defined by isRetryableError).
// Non-retryable errors are returned immediately.
func executeWithRetry(ctx context.Context, cfg RetryConfig, providerName string, logger *zap.Logger, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 0 {
				retryAttemptsTotal.WithLabelValues(providerName, "success").Inc()
				logger.Info("request succeeded after retry",
					zap.String("provider", providerName),
					zap.Int("attempt", attempt+1),
				)
			}
			return nil
		}

		lastErr = err

		// Don't retry non-transient errors
		if !isRetryableError(err.Error()) {
			return err
		}

		// Don't retry if we've exhausted attempts
		if attempt >= cfg.MaxRetries {
			break
		}

		// Calculate backoff: initial * multiplier^attempt, capped at max
		backoff := time.Duration(float64(cfg.InitialBackoff) * math.Pow(cfg.BackoffMultiplier, float64(attempt)))
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}

		logger.Warn("transient error, retrying with backoff",
			zap.String("provider", providerName),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", cfg.MaxRetries),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		// Wait with context awareness
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	retryAttemptsTotal.WithLabelValues(providerName, "exhausted").Inc()
	return lastErr
}
