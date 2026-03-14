package observability

import (
	"context"
	"testing"
)

func TestNoopService_ImplementsService(t *testing.T) {
	var _ Service = (*NoopService)(nil) // compile-time check
}

func TestNoopService_StartTrace(t *testing.T) {
	svc := NewNoopService()
	trace := svc.StartTrace(context.Background(), "trace-1", "test", "user-1", "session-1", nil)
	if trace == nil {
		t.Fatal("expected non-nil trace")
	}
	if trace.GetID() != "trace-1" {
		t.Errorf("expected trace ID 'trace-1', got %q", trace.GetID())
	}
	trace.End() // should not panic
}

func TestNoopTrace_GetID_GeneratesUUID(t *testing.T) {
	trace := &NoopTrace{} // empty id
	id := trace.GetID()
	if id == "" {
		t.Error("expected non-empty generated UUID")
	}
}

func TestNoopService_StartGeneration(t *testing.T) {
	svc := NewNoopService()
	trace := svc.StartTrace(context.Background(), "t1", "test", "", "", nil)
	gen := svc.StartGeneration(context.Background(), trace, "gen", "gpt-4", nil, "hello")
	if gen == nil {
		t.Fatal("expected non-nil generation")
	}
	gen.End("output", 10, 20)       // should not panic
	gen.EndWithError(nil)            // should not panic
}

func TestNoopService_Shutdown(t *testing.T) {
	svc := NewNoopService()
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
