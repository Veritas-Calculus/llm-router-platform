package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ─── Circuit Breaker Tests ─────────────────────────────────────────────────

func newTestCB(threshold int, recovery time.Duration, halfOpenProbes int) *CircuitBreaker {
	return NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold:  threshold,
		RecoveryTimeout:   recovery,
		HalfOpenMaxProbes: halfOpenProbes,
	}, zap.NewNop())
}

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	cb := newTestCB(3, 1*time.Second, 1)
	id := uuid.New()

	state, errors := cb.GetState(id)
	if state != CircuitClosed {
		t.Errorf("expected CircuitClosed, got %v", state)
	}
	if errors != 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}
	if !cb.AllowRequest(id) {
		t.Error("expected AllowRequest to be true for unknown provider")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := newTestCB(3, 5*time.Second, 1)
	id := uuid.New()

	// Record failures below threshold — should stay closed
	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")
	if state, _ := cb.GetState(id); state != CircuitClosed {
		t.Errorf("expected CircuitClosed after 2 failures, got %v", state)
	}

	// Third failure should trip the circuit
	cb.RecordFailure(id, "test-provider")
	state, errCount := cb.GetState(id)
	if state != CircuitOpen {
		t.Errorf("expected CircuitOpen after 3 failures, got %v", state)
	}
	if errCount != 3 {
		t.Errorf("expected 3 consecutive errors, got %d", errCount)
	}

	// Requests should be blocked
	if cb.AllowRequest(id) {
		t.Error("expected AllowRequest to be false when circuit is open")
	}
}

func TestCircuitBreaker_SuccessResetsCounter(t *testing.T) {
	cb := newTestCB(3, 5*time.Second, 1)
	id := uuid.New()

	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")
	cb.RecordSuccess(id, "test-provider") // Reset

	// After success, counter should be 0
	if state, errCount := cb.GetState(id); state != CircuitClosed || errCount != 0 {
		t.Errorf("expected CircuitClosed with 0 errors, got %v with %d", state, errCount)
	}

	// Should need 3 more failures to trip
	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")
	if state, _ := cb.GetState(id); state != CircuitClosed {
		t.Error("should still be closed after 2 failures post-reset")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb := newTestCB(2, 50*time.Millisecond, 1)
	id := uuid.New()

	// Trip the circuit
	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")

	if cb.AllowRequest(id) {
		t.Error("expected blocked immediately after tripping")
	}

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// Should now transition to half-open and allow probe
	if !cb.AllowRequest(id) {
		t.Error("expected AllowRequest to be true after recovery timeout (half-open)")
	}

	state, _ := cb.GetState(id)
	if state != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen, got %v", state)
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	cb := newTestCB(2, 50*time.Millisecond, 2)
	id := uuid.New()

	// Trip → wait → half-open
	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")
	time.Sleep(60 * time.Millisecond)
	cb.AllowRequest(id) // triggers half-open transition

	// First probe success
	cb.RecordSuccess(id, "test-provider")
	if state, _ := cb.GetState(id); state != CircuitHalfOpen {
		t.Errorf("expected still HalfOpen after 1 probe (need 2), got %v", state)
	}

	// Second probe success — should close
	cb.RecordSuccess(id, "test-provider")
	if state, _ := cb.GetState(id); state != CircuitClosed {
		t.Errorf("expected CircuitClosed after 2 probe successes, got %v", state)
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := newTestCB(2, 50*time.Millisecond, 2)
	id := uuid.New()

	// Trip → wait → half-open
	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")
	time.Sleep(60 * time.Millisecond)
	cb.AllowRequest(id) // triggers half-open

	// Probe failure → reopen
	cb.RecordFailure(id, "test-provider")
	state, _ := cb.GetState(id)
	if state != CircuitOpen {
		t.Errorf("expected CircuitOpen after half-open probe failure, got %v", state)
	}

	// Should block again
	if cb.AllowRequest(id) {
		t.Error("expected blocked after re-open")
	}
}

func TestCircuitBreaker_ManualReset(t *testing.T) {
	cb := newTestCB(2, 5*time.Second, 1)
	id := uuid.New()

	cb.RecordFailure(id, "test-provider")
	cb.RecordFailure(id, "test-provider")
	if state, _ := cb.GetState(id); state != CircuitOpen {
		t.Fatal("precondition: expected open circuit")
	}

	cb.Reset(id)
	state, errCount := cb.GetState(id)
	if state != CircuitClosed || errCount != 0 {
		t.Errorf("expected CircuitClosed with 0 errors after reset, got %v with %d", state, errCount)
	}
}

func TestCircuitBreaker_IndependentProviders(t *testing.T) {
	cb := newTestCB(2, 5*time.Second, 1)
	id1 := uuid.New()
	id2 := uuid.New()

	// Trip provider 1
	cb.RecordFailure(id1, "provider-1")
	cb.RecordFailure(id1, "provider-1")

	// Provider 2 should be unaffected
	if !cb.AllowRequest(id2) {
		t.Error("expected provider-2 to be unaffected by provider-1's circuit")
	}
	if cb.AllowRequest(id1) {
		t.Error("expected provider-1 to be blocked")
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half_open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// ─── Retry Tests ───────────────────────────────────────────────────────────

func TestIsRetryableError(t *testing.T) {
	retryable := []string{
		"status 429: too many requests",
		"502 Bad Gateway",
		"503 Service Unavailable",
		"context deadline exceeded",
		"connection refused",
	}
	for _, msg := range retryable {
		if !isRetryableError(msg) {
			t.Errorf("expected %q to be retryable", msg)
		}
	}

	nonRetryable := []string{
		"invalid API key",
		"model not found",
		"400 Bad Request",
		"permission denied",
	}
	for _, msg := range nonRetryable {
		if isRetryableError(msg) {
			t.Errorf("expected %q to NOT be retryable", msg)
		}
	}
}

func TestExecuteWithRetry_SuccessNoRetry(t *testing.T) {
	callCount := 0
	err := executeWithRetry(context.Background(), DefaultRetryConfig(), "test", zap.NewNop(), func() error {
		callCount++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestExecuteWithRetry_NonRetryableError(t *testing.T) {
	callCount := 0
	err := executeWithRetry(context.Background(), DefaultRetryConfig(), "test", zap.NewNop(), func() error {
		callCount++
		return errors.New("invalid API key")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry for non-retryable), got %d", callCount)
	}
}

func TestExecuteWithRetry_RetryThenSuccess(t *testing.T) {
	callCount := 0
	cfg := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
	err := executeWithRetry(context.Background(), cfg, "test", zap.NewNop(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("503 service unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 retries + success), got %d", callCount)
	}
}

func TestExecuteWithRetry_ExhaustedRetries(t *testing.T) {
	callCount := 0
	cfg := RetryConfig{
		MaxRetries:        2,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
	err := executeWithRetry(context.Background(), cfg, "test", zap.NewNop(), func() error {
		callCount++
		return errors.New("429 too many requests")
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 1 initial + 2 retries = 3 calls
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestExecuteWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	cfg := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	// Cancel after first failure
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := executeWithRetry(ctx, cfg, "test", zap.NewNop(), func() error {
		callCount++
		return errors.New("503 service unavailable")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	// Should have stopped early due to context cancellation
	if callCount > 2 {
		t.Errorf("expected at most 2 calls before cancel, got %d", callCount)
	}
}
