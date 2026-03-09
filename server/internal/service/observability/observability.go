// Package observability provides tracing and telemetry services.
package observability

import (
	"context"

	"github.com/google/uuid"
)

// Trace represents an ongoing trace map.
type Trace interface {
	GetID() string
	End()
}

// Generation represents a single LLM generation event.
type Generation interface {
	End(output string, promptTokens, completionTokens int)
	EndWithError(err error)
}

// Service defines the interface for tracing capabilities.
type Service interface {
	StartTrace(ctx context.Context, id, name, userID, sessionID string, metadata map[string]interface{}) Trace
	StartGeneration(ctx context.Context, trace Trace, name, model string, modelParams map[string]interface{}, input interface{}) Generation
	Shutdown(ctx context.Context) error
}

// NoopService is a dummy implementation when observability is disabled.
type NoopService struct{}

// NewNoopService creates a new NoopService.
func NewNoopService() *NoopService {
	return &NoopService{}
}

func (n *NoopService) StartTrace(ctx context.Context, id, name, userID, sessionID string, metadata map[string]interface{}) Trace {
	return &NoopTrace{id: id}
}

func (n *NoopService) StartGeneration(ctx context.Context, trace Trace, name, model string, modelParams map[string]interface{}, input interface{}) Generation {
	return &NoopGeneration{}
}

func (n *NoopService) Shutdown(ctx context.Context) error {
	return nil
}

type NoopTrace struct{ id string }

func (t *NoopTrace) GetID() string {
	if t.id != "" {
		return t.id
	}
	return uuid.New().String()
}

func (t *NoopTrace) End() {}

type NoopGeneration struct{}

func (g *NoopGeneration) End(output string, promptTokens, completionTokens int) {}
func (g *NoopGeneration) EndWithError(err error)                                {}
