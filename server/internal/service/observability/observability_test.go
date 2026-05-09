package observability

import (
	"context"
	"testing"

	"llm-router-platform/internal/config"
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
	gen.End("output", 10, 20) // should not panic
	gen.EndWithError(nil)     // should not panic
}

func TestNoopService_Shutdown(t *testing.T) {
	svc := NewNoopService()
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestIntegrationConfigOverridesRuntimeIgnoresEmptyDefaultRows(t *testing.T) {
	if IntegrationConfigOverridesRuntime(false, []byte(`{}`)) {
		t.Fatal("disabled empty integration row should not override environment config")
	}
	if !IntegrationConfigOverridesRuntime(true, []byte(`{}`)) {
		t.Fatal("enabled integration row should override environment config")
	}
	if !IntegrationConfigOverridesRuntime(false, []byte(`{"baseUrl":"https://cloud.langfuse.com"}`)) {
		t.Fatal("disabled non-empty integration row should override environment config")
	}
}

func TestLangfuseConfigFromIntegration(t *testing.T) {
	base := config.ObservabilityConfig{LangfuseHost: "https://env.langfuse"}
	next, err := LangfuseConfigFromIntegration(base, true, []byte(`{
		"publicKey": "pk-lf-test",
		"secretKey": "sk-lf-test",
		"baseUrl": "https://cloud.langfuse.com/"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.LangfuseEnabled {
		t.Fatal("expected langfuse to be enabled")
	}
	if next.LangfuseHost != "https://cloud.langfuse.com" {
		t.Fatalf("host = %q, want trimmed base url", next.LangfuseHost)
	}
}

func TestLangfuseConfigFromIntegrationRequiresCredentials(t *testing.T) {
	_, err := LangfuseConfigFromIntegration(config.ObservabilityConfig{}, true, []byte(`{"baseUrl":"https://cloud.langfuse.com"}`))
	if err == nil {
		t.Fatal("expected missing credentials to fail")
	}
}
