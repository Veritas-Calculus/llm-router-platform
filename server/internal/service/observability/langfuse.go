package observability

import (
	"context"

	"go.uber.org/zap"

	"llm-router-platform/internal/config"

	langfuse "github.com/git-hulk/langfuse-go"
	"github.com/git-hulk/langfuse-go/pkg/traces"
)

// LangfuseService implements the Observability Service using the Langfuse SDK.
type LangfuseService struct {
	client *langfuse.Langfuse
	logger *zap.Logger
}

// NewLangfuseService initializes a new Langfuse observability client.
func NewLangfuseService(cfg config.ObservabilityConfig, logger *zap.Logger) Service {
	if !cfg.LangfuseEnabled || cfg.LangfusePublicKey == "" || cfg.LangfuseSecretKey == "" {
		logger.Info("Langfuse observability is disabled or missing credentials")
		return NewNoopService()
	}

	client := langfuse.NewClient(cfg.LangfuseHost, cfg.LangfusePublicKey, cfg.LangfuseSecretKey)

	logger.Info("Langfuse observability initialized successfully", zap.String("host", cfg.LangfuseHost))

	return &LangfuseService{
		client: client,
		logger: logger,
	}
}

// StartTrace creates a base trace for an incoming request.
func (s *LangfuseService) StartTrace(ctx context.Context, id, name, userID, sessionID string, metadata map[string]interface{}) Trace {
	// Note: go SDK StartTrace takes name only; we manually assign ID and metadata to the underlying struct if available.
	// But according to git-hulk SDK docs, StartTrace acts as a simplified wrapper, so we map fields directly to the Trace struct.
	t := s.client.StartTrace(ctx, name)

	if id != "" {
		t.ID = id
	}
	t.UserID = userID
	t.SessionID = sessionID
	t.Metadata = metadata

	return &LangfuseTrace{trace: t}
}

// StartGeneration kicks off an LLM call record tied to the given trace.
func (s *LangfuseService) StartGeneration(ctx context.Context, pTrace Trace, name, model string, modelParams map[string]interface{}, input interface{}) Generation {
	lt, ok := pTrace.(*LangfuseTrace)
	if !ok {
		return &NoopGeneration{}
	}

	obs := lt.trace.StartGeneration(name)
	obs.Model = model
	obs.ModelParameters = modelParams
	obs.Input = input

	return &LangfuseGeneration{obs: obs}
}

// Shutdown ensures all traces are flushed before the application exits.
func (s *LangfuseService) Shutdown(ctx context.Context) error {
	s.logger.Info("flushing local Langfuse telemetry buffer")
	s.client.Flush()
	return s.client.Close()
}

// LangfuseTrace wraps the underlying SDK Trace.
type LangfuseTrace struct {
	trace *traces.Trace
}

func (lt *LangfuseTrace) GetID() string {
	return lt.trace.ID
}

func (lt *LangfuseTrace) End() {
	lt.trace.End()
}

// LangfuseGeneration wraps the underlying SDK Observation specific to Generation types.
type LangfuseGeneration struct {
	obs *traces.Observation
}

func (lg *LangfuseGeneration) End(output string, promptTokens, completionTokens int) {
	lg.obs.Output = output
	lg.obs.Usage = traces.Usage{
		Input:  promptTokens,
		Output: completionTokens,
		Total:  promptTokens + completionTokens,
		Unit:   traces.UnitTokens,
	}
	lg.obs.End()
}

func (lg *LangfuseGeneration) EndWithError(err error) {
	lg.obs.Level = traces.ObservationLevelError
	lg.obs.StatusMessage = err.Error()
	lg.obs.End()
}
