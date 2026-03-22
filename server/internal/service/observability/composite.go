package observability

import (
	"context"

	"github.com/google/uuid"
)

// CompositeService combines multiple observability services into one interface.
type CompositeService struct {
	services []Service
}

// NewCompositeService creates a new CompositeService from a list of delegates.
func NewCompositeService(services ...Service) *CompositeService {
	var active []Service
	for _, s := range services {
		if _, ok := s.(*NoopService); !ok {
			active = append(active, s)
		}
	}
	
	// If no active services, just append one NoopService to satisfy assumptions
	if len(active) == 0 {
		active = append(active, NewNoopService())
	}
	
	return &CompositeService{services: active}
}

func (c *CompositeService) StartTrace(ctx context.Context, id, name, userID, sessionID string, metadata map[string]interface{}) Trace {
	traces := make([]Trace, len(c.services))
	for i, s := range c.services {
		traces[i] = s.StartTrace(ctx, id, name, userID, sessionID, metadata)
	}
	return &CompositeTrace{traces: traces}
}

func (c *CompositeService) StartGeneration(ctx context.Context, trace Trace, name, model string, modelParams map[string]interface{}, input interface{}) Generation {
	ct, ok := trace.(*CompositeTrace)
	if !ok || len(ct.traces) != len(c.services) {
		return &NoopGeneration{}
	}
	
	generations := make([]Generation, len(c.services))
	for i, s := range c.services {
		generations[i] = s.StartGeneration(ctx, ct.traces[i], name, model, modelParams, input)
	}
	return &CompositeGeneration{generations: generations}
}

func (c *CompositeService) Shutdown(ctx context.Context) error {
	var lastErr error
	for _, s := range c.services {
		if err := s.Shutdown(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type CompositeTrace struct {
	traces []Trace
}

func (ct *CompositeTrace) GetID() string {
	if len(ct.traces) > 0 {
		return ct.traces[0].GetID()
	}
	return uuid.New().String()
}

func (ct *CompositeTrace) End() {
	for _, t := range ct.traces {
		if t != nil {
			t.End()
		}
	}
}

type CompositeGeneration struct {
	generations []Generation
}

func (cg *CompositeGeneration) End(output string, promptTokens, completionTokens int) {
	for _, g := range cg.generations {
		if g != nil {
			g.End(output, promptTokens, completionTokens)
		}
	}
}

func (cg *CompositeGeneration) EndWithError(err error) {
	for _, g := range cg.generations {
		if g != nil {
			g.EndWithError(err)
		}
	}
}
