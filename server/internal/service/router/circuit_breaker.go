// Package router provides LLM request routing logic.
// This file implements a provider-level circuit breaker with a proper 3-state
// machine (Closed → Open → Half-Open) and Prometheus instrumentation.
package router

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// ─── Circuit Breaker States ────────────────────────────────────────────────

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed — normal operation, requests flow through.
	CircuitClosed CircuitState = iota
	// CircuitOpen — provider is unhealthy, all requests are rejected.
	CircuitOpen
	// CircuitHalfOpen — recovery probe in progress, one request allowed.
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ─── Configuration ─────────────────────────────────────────────────────────

// CircuitBreakerConfig holds tunable thresholds for the circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening.
	FailureThreshold int
	// RecoveryTimeout is how long the circuit stays open before transitioning
	// to half-open for a probe request.
	RecoveryTimeout time.Duration
	// HalfOpenMaxProbes is the number of successful probes required in
	// half-open state before transitioning back to closed.
	HalfOpenMaxProbes int
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:  5,
		RecoveryTimeout:   30 * time.Second,
		HalfOpenMaxProbes: 2,
	}
}

// ─── Per-Provider State ────────────────────────────────────────────────────

// providerCircuit tracks the circuit breaker state for a single provider.
type providerCircuit struct {
	state             CircuitState
	consecutiveErrors int
	lastFailureAt     time.Time
	openedAt          time.Time
	halfOpenSuccesses int
	providerName      string // for logging / metrics labels
}

// ─── Package-level Prometheus metrics (registered once) ────────────────────

var (
	cbStateGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "llm_router",
			Name:      "circuit_breaker_state",
			Help:      "Current circuit breaker state per provider (0=closed, 1=open, 2=half_open).",
		},
		[]string{"provider", "provider_id"},
	)
	cbTripsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "circuit_breaker_trips_total",
			Help:      "Total number of times the circuit breaker has tripped (closed→open).",
		},
		[]string{"provider", "provider_id"},
	)
	cbProbeCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "circuit_breaker_probes_total",
			Help:      "Total probe attempts in half-open state.",
		},
		[]string{"provider", "provider_id", "result"}, // result: "success" | "failure"
	)
)

// CircuitBreaker manages per-provider circuit breaker state with Prometheus
// metrics.  It is safe for concurrent use.
type CircuitBreaker struct {
	mu       sync.RWMutex
	circuits map[uuid.UUID]*providerCircuit
	cfg      CircuitBreakerConfig
	logger   *zap.Logger
}

// NewCircuitBreaker creates a CircuitBreaker with the given config and logger.
func NewCircuitBreaker(cfg CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		circuits: make(map[uuid.UUID]*providerCircuit),
		cfg:      cfg,
		logger:   logger,
	}
}

// ─── Public API ────────────────────────────────────────────────────────────

// AllowRequest returns true if a request to the provider should be permitted.
// It also handles the automatic Open→HalfOpen transition after RecoveryTimeout.
func (cb *CircuitBreaker) AllowRequest(providerID uuid.UUID) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	c, ok := cb.circuits[providerID]
	if !ok {
		return true // no state = closed
	}

	switch c.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if recovery timeout has elapsed → transition to half-open
		if time.Since(c.openedAt) >= cb.cfg.RecoveryTimeout {
			c.state = CircuitHalfOpen
			c.halfOpenSuccesses = 0
			cb.updateMetrics(c, providerID)
			cb.logger.Info("circuit breaker transitioning to half-open",
				zap.String("provider", c.providerName),
				zap.String("provider_id", providerID.String()),
			)
			return true // allow probe request
		}
		return false

	case CircuitHalfOpen:
		// In half-open, allow requests (limited — the caller is probing)
		return true

	default:
		return true
	}
}

// RecordSuccess records a successful request. In half-open state, this may
// close the circuit after enough successful probes.
func (cb *CircuitBreaker) RecordSuccess(providerID uuid.UUID, providerName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	c := cb.getOrCreate(providerID, providerName)

	switch c.state {
	case CircuitClosed:
		// Reset consecutive errors on any success
		c.consecutiveErrors = 0

	case CircuitHalfOpen:
		c.halfOpenSuccesses++
		cbProbeCounter.WithLabelValues(c.providerName, providerID.String(), "success").Inc()

		if c.halfOpenSuccesses >= cb.cfg.HalfOpenMaxProbes {
			// Enough successful probes — close the circuit
			cb.logger.Info("circuit breaker closed after successful probes",
				zap.String("provider", c.providerName),
				zap.String("provider_id", providerID.String()),
				zap.Int("probes_passed", c.halfOpenSuccesses),
			)
			c.state = CircuitClosed
			c.consecutiveErrors = 0
			c.halfOpenSuccesses = 0
			cb.updateMetrics(c, providerID)
		}

	case CircuitOpen:
		// Should not happen (AllowRequest would block), but handle gracefully
		c.state = CircuitHalfOpen
		c.halfOpenSuccesses = 1
		cb.updateMetrics(c, providerID)
	}
}

// RecordFailure records a failed request. In closed state, this may trip
// the circuit. In half-open state, this re-opens the circuit.
func (cb *CircuitBreaker) RecordFailure(providerID uuid.UUID, providerName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	c := cb.getOrCreate(providerID, providerName)
	c.lastFailureAt = time.Now()

	switch c.state {
	case CircuitClosed:
		c.consecutiveErrors++
		if c.consecutiveErrors >= cb.cfg.FailureThreshold {
			c.state = CircuitOpen
			c.openedAt = time.Now()
			cb.updateMetrics(c, providerID)
			cbTripsCounter.WithLabelValues(c.providerName, providerID.String()).Inc()
			cb.logger.Warn("circuit breaker OPENED — provider marked unhealthy",
				zap.String("provider", c.providerName),
				zap.String("provider_id", providerID.String()),
				zap.Int("consecutive_failures", c.consecutiveErrors),
				zap.Duration("recovery_timeout", cb.cfg.RecoveryTimeout),
			)
		}

	case CircuitHalfOpen:
		// Probe failed — re-open
		cbProbeCounter.WithLabelValues(c.providerName, providerID.String(), "failure").Inc()
		c.state = CircuitOpen
		c.openedAt = time.Now()
		c.halfOpenSuccesses = 0
		cb.updateMetrics(c, providerID)
		cb.logger.Warn("circuit breaker RE-OPENED — half-open probe failed",
			zap.String("provider", c.providerName),
			zap.String("provider_id", providerID.String()),
		)

	case CircuitOpen:
		// Already open — just update last failure time
	}
}

// GetState returns the current state and consecutive error count for a provider.
func (cb *CircuitBreaker) GetState(providerID uuid.UUID) (CircuitState, int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	c, ok := cb.circuits[providerID]
	if !ok {
		return CircuitClosed, 0
	}

	// Check for auto-transition to half-open
	if c.state == CircuitOpen && time.Since(c.openedAt) >= cb.cfg.RecoveryTimeout {
		return CircuitHalfOpen, c.consecutiveErrors
	}

	return c.state, c.consecutiveErrors
}

// Reset forces a circuit back to closed state. Useful for manual recovery via admin API.
func (cb *CircuitBreaker) Reset(providerID uuid.UUID) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	c, ok := cb.circuits[providerID]
	if !ok {
		return
	}

	cb.logger.Info("circuit breaker manually reset",
		zap.String("provider", c.providerName),
		zap.String("provider_id", providerID.String()),
	)
	c.state = CircuitClosed
	c.consecutiveErrors = 0
	c.halfOpenSuccesses = 0
	cb.updateMetrics(c, providerID)
}

// ─── Internal Helpers ──────────────────────────────────────────────────────

func (cb *CircuitBreaker) getOrCreate(providerID uuid.UUID, providerName string) *providerCircuit {
	c, ok := cb.circuits[providerID]
	if !ok {
		c = &providerCircuit{
			state:        CircuitClosed,
			providerName: providerName,
		}
		cb.circuits[providerID] = c
	}
	// Update name if it was empty (e.g., created by AllowRequest path)
	if c.providerName == "" && providerName != "" {
		c.providerName = providerName
	}
	return c
}

func (cb *CircuitBreaker) updateMetrics(c *providerCircuit, providerID uuid.UUID) {
	cbStateGauge.WithLabelValues(c.providerName, providerID.String()).Set(float64(c.state))
}
