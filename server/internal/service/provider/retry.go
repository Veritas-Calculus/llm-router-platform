// Package provider provides LLM provider client implementations.
package provider

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// retryableError wraps an error with upstream HTTP status information.
type retryableError interface {
	StatusCode() int
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries    int           // Maximum number of retries (default 3)
	InitialDelay  time.Duration // Initial delay between retries (default 500ms)
	MaxDelay      time.Duration // Maximum delay between retries (default 30s)
	Multiplier    float64       // Exponential backoff multiplier (default 2.0)
}

// DefaultRetryConfig returns sensible defaults for LLM API retries.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// RetryConfigFromProvider builds a RetryConfig using per-provider settings.
// Zero values fall back to defaults.
func RetryConfigFromProvider(maxRetries, timeoutSecs int) RetryConfig {
	cfg := DefaultRetryConfig()
	if maxRetries > 0 {
		cfg.MaxRetries = maxRetries
	}
	if timeoutSecs > 0 {
		cfg.MaxDelay = time.Duration(timeoutSecs) * time.Second
	}
	return cfg
}

// RetryClient wraps a Client with automatic retry on transient errors.
type RetryClient struct {
	inner  Client
	config RetryConfig
	logger *zap.Logger
}

// NewRetryClient wraps a provider Client with retry logic.
func NewRetryClient(inner Client, config RetryConfig, logger *zap.Logger) *RetryClient {
	return &RetryClient{inner: inner, config: config, logger: logger}
}

// Chat retries on transient failures.
func (r *RetryClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	var resp *ChatResponse
	err := r.retryDo(ctx, "Chat", func(ctx context.Context) error {
		var e error
		resp, e = r.inner.Chat(ctx, req)
		return e
	})
	return resp, err
}

// StreamChat is not retried — streaming responses cannot be replayed.
func (r *RetryClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	return r.inner.StreamChat(ctx, req)
}

// Embeddings retries on transient failures.
func (r *RetryClient) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	var resp *EmbeddingResponse
	err := r.retryDo(ctx, "Embeddings", func(ctx context.Context) error {
		var e error
		resp, e = r.inner.Embeddings(ctx, req)
		return e
	})
	return resp, err
}

// GenerateImage retries on transient failures.
func (r *RetryClient) GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	var resp *ImageGenerationResponse
	err := r.retryDo(ctx, "GenerateImage", func(ctx context.Context) error {
		var e error
		resp, e = r.inner.GenerateImage(ctx, req)
		return e
	})
	return resp, err
}

// TranscribeAudio retries on transient failures.
func (r *RetryClient) TranscribeAudio(ctx context.Context, req *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error) {
	var resp *AudioTranscriptionResponse
	err := r.retryDo(ctx, "TranscribeAudio", func(ctx context.Context) error {
		var e error
		resp, e = r.inner.TranscribeAudio(ctx, req)
		return e
	})
	return resp, err
}

// SynthesizeSpeech retries on transient failures.
func (r *RetryClient) SynthesizeSpeech(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error) {
	var resp *SpeechResponse
	err := r.retryDo(ctx, "SynthesizeSpeech", func(ctx context.Context) error {
		var e error
		resp, e = r.inner.SynthesizeSpeech(ctx, req)
		return e
	})
	return resp, err
}

// ListModels retries on transient failures.
func (r *RetryClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	var models []ModelInfo
	err := r.retryDo(ctx, "ListModels", func(ctx context.Context) error {
		var e error
		models, e = r.inner.ListModels(ctx)
		return e
	})
	return models, err
}

// CheckHealth retries on transient failures.
func (r *RetryClient) CheckHealth(ctx context.Context) (bool, time.Duration, error) {
	var healthy bool
	var latency time.Duration
	err := r.retryDo(ctx, "CheckHealth", func(ctx context.Context) error {
		var e error
		healthy, latency, e = r.inner.CheckHealth(ctx)
		return e
	})
	return healthy, latency, err
}

// retryDo executes fn with exponential backoff retry on transient errors.
func (r *RetryClient) retryDo(ctx context.Context, method string, fn func(ctx context.Context) error) error {
	var lastErr error
	delay := r.config.InitialDelay

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if !r.isRetryable(lastErr) {
			return lastErr
		}

		if attempt == r.config.MaxRetries {
			break
		}

		// Check for upstream Retry-After header hint
		if retryAfter := r.extractRetryAfter(lastErr); retryAfter > 0 {
			delay = retryAfter
		}

		r.logger.Warn("provider call failed, retrying",
			zap.String("method", method),
			zap.Int("attempt", attempt+1),
			zap.Duration("delay", delay),
			zap.Error(lastErr),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff
		delay = time.Duration(float64(delay) * r.config.Multiplier)
		if delay > r.config.MaxDelay {
			delay = r.config.MaxDelay
		}
	}

	return lastErr
}

// isRetryable determines if an error represents a transient failure worth retrying.
func (r *RetryClient) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	errMsg := err.Error()

	// HTTP status-based retryable detection
	var re retryableError
	if errors.As(err, &re) {
		code := re.StatusCode()
		return code == http.StatusTooManyRequests ||
			code == http.StatusServiceUnavailable ||
			code == http.StatusBadGateway ||
			code == http.StatusGatewayTimeout ||
			code >= 500
	}

	// String-based fallback for errors without status codes
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"EOF",
		"429",
		"502",
		"503",
		"504",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// extractRetryAfter looks for a Retry-After hint in the error.
func (r *RetryClient) extractRetryAfter(err error) time.Duration {
	errMsg := err.Error()

	// Look for "Retry-After: <seconds>" pattern
	if idx := strings.Index(errMsg, "Retry-After:"); idx >= 0 {
		rest := strings.TrimSpace(errMsg[idx+12:])
		if spIdx := strings.IndexByte(rest, ' '); spIdx > 0 {
			rest = rest[:spIdx]
		}
		if secs, e := strconv.Atoi(strings.TrimSpace(rest)); e == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}

	return 0
}
