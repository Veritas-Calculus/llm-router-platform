// Package observability provides telemetry and error tracking services.
// This file implements Sentry error tracking integration.
package observability

import (
	"fmt"
	"time"

	"llm-router-platform/internal/config"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// InitSentry initializes the Sentry SDK for error tracking and performance monitoring.
// Returns nil if Sentry is disabled or DSN is empty.
func InitSentry(cfg config.ObservabilityConfig, logger *zap.Logger) error {
	if !cfg.SentryEnabled || cfg.SentryDSN == "" {
		logger.Info("Sentry error tracking is disabled or missing DSN")
		return nil
	}

	env := cfg.SentryEnvironment
	if env == "" {
		env = "production"
	}

	sampleRate := cfg.SentrySampleRate
	if sampleRate <= 0 {
		sampleRate = 1.0
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Environment:      env,
		SampleRate:       sampleRate,
		TracesSampleRate: sampleRate,
		EnableTracing:    true,
		AttachStacktrace: true,
		ServerName:       "llm-router",
	})
	if err != nil {
		return fmt.Errorf("sentry init failed: %w", err)
	}

	logger.Info("Sentry error tracking initialized",
		zap.String("environment", env),
		zap.Float64("sample_rate", sampleRate),
	)
	return nil
}

// ShutdownSentry flushes any buffered events before the application exits.
func ShutdownSentry(logger *zap.Logger) {
	if sentry.CurrentHub().Client() == nil {
		return
	}
	logger.Info("flushing Sentry event buffer")
	sentry.Flush(2 * time.Second)
}
