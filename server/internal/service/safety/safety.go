// Package safety provides content safety classification for LLM requests.
// It defines the SafetyClassifier interface and implementations for
// detecting unsafe content (prompt injection, harmful instructions, etc.)
// before forwarding requests to upstream LLM providers.
package safety

import (
	"context"

	"llm-router-platform/internal/service/provider"
)

// SafetyResult represents the outcome of a safety classification.
type SafetyResult struct {
	// Safe indicates whether the content passed safety checks.
	Safe bool `json:"safe"`
	// Category identifies the type of unsafe content detected (if any).
	// Examples: "S1" (violent crimes), "S2" (non-violent crimes), "S6" (self-harm),
	// "S13" (prompt injection). Maps to Llama Guard taxonomy.
	Category string `json:"category,omitempty"`
	// Score represents the confidence of the classification (0.0 to 1.0).
	Score float64 `json:"score,omitempty"`
	// Reason provides a human-readable explanation of the classification.
	Reason string `json:"reason,omitempty"`
}

// Classifier defines the interface for content safety classification.
// Implementations should analyze the conversation messages and determine
// whether the content is safe to forward to an LLM provider.
type Classifier interface {
	// Classify analyzes the given messages and returns a safety result.
	// Returns an error only for infrastructure failures (network, timeout, etc.),
	// not for unsafe content detection (which is returned in SafetyResult).
	Classify(ctx context.Context, messages []provider.Message) (*SafetyResult, error)
}

// NoopClassifier is a no-op safety classifier that always returns safe.
// Used as the default when no safety service is configured.
type NoopClassifier struct{}

// Classify always returns a safe result. This is the default behavior
// when Llama Guard or another safety service is not deployed.
func (n *NoopClassifier) Classify(_ context.Context, _ []provider.Message) (*SafetyResult, error) {
	return &SafetyResult{Safe: true}, nil
}

// LlamaGuardClassifier classifies content using Meta's Llama Guard model
// deployed as an internal HTTP service (via vLLM or similar inference server).
// This is a placeholder that will be fully implemented when the Llama Guard
// model is deployed in the infrastructure (P2 task).
type LlamaGuardClassifier struct {
	// Endpoint is the HTTP URL of the Llama Guard inference service.
	Endpoint string
	// Model is the specific Llama Guard model to use (e.g., "meta-llama/Llama-Guard-3-8B").
	Model string
}

// Classify sends the conversation to the Llama Guard model for safety classification.
// TODO(P2): Implement actual HTTP call to Llama Guard inference endpoint.
func (l *LlamaGuardClassifier) Classify(_ context.Context, _ []provider.Message) (*SafetyResult, error) {
	// Placeholder: returns safe until Llama Guard model is deployed
	return &SafetyResult{Safe: true}, nil
}
