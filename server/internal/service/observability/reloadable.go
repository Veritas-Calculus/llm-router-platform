package observability

import (
	"context"
	"sync"
	"time"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// LangfuseReloader is implemented by services that can swap Langfuse runtime
// configuration without restarting the process.
type LangfuseReloader interface {
	ReloadLangfuse(ctx context.Context, cfg config.ObservabilityConfig) error
}

// ReloadableLangfuseService keeps Langfuse hot-swappable while preserving
// in-flight traces on the client instance that created them.
type ReloadableLangfuseService struct {
	mu            sync.RWMutex
	active        Service
	logger        *zap.Logger
	shutdownGrace time.Duration
}

func NewReloadableLangfuseService(cfg config.ObservabilityConfig, logger *zap.Logger) *ReloadableLangfuseService {
	return &ReloadableLangfuseService{
		active:        NewLangfuseService(cfg, logger),
		logger:        logger,
		shutdownGrace: 2 * time.Minute,
	}
}

func (s *ReloadableLangfuseService) StartTrace(ctx context.Context, id, name, userID, sessionID string, metadata map[string]interface{}) Trace {
	active := s.current()
	return &reloadableTrace{
		service: active,
		trace:   active.StartTrace(ctx, id, name, userID, sessionID, metadata),
	}
}

func (s *ReloadableLangfuseService) StartGeneration(ctx context.Context, trace Trace, name, model string, modelParams map[string]interface{}, input interface{}) Generation {
	if rt, ok := trace.(*reloadableTrace); ok && rt.service != nil && rt.trace != nil {
		return rt.service.StartGeneration(ctx, rt.trace, name, model, modelParams, input)
	}
	return s.current().StartGeneration(ctx, trace, name, model, modelParams, input)
}

func (s *ReloadableLangfuseService) ReloadLangfuse(ctx context.Context, cfg config.ObservabilityConfig) error {
	next := NewLangfuseService(cfg, s.logger)

	s.mu.Lock()
	old := s.active
	s.active = next
	s.mu.Unlock()

	s.logger.Info("langfuse runtime configuration reloaded",
		zap.Bool("enabled", cfg.LangfuseEnabled),
		zap.String("host", cfg.LangfuseHost),
	)
	s.shutdownLater(old)
	return nil
}

func (s *ReloadableLangfuseService) Shutdown(ctx context.Context) error {
	return s.current().Shutdown(ctx)
}

func (s *ReloadableLangfuseService) current() Service {
	s.mu.RLock()
	active := s.active
	s.mu.RUnlock()
	if active == nil {
		return NewNoopService()
	}
	return active
}

func (s *ReloadableLangfuseService) shutdownLater(old Service) {
	if old == nil {
		return
	}
	if _, ok := old.(*NoopService); ok {
		return
	}

	go func() {
		time.Sleep(s.shutdownGrace)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := old.Shutdown(ctx); err != nil {
			s.logger.Warn("old langfuse client shutdown failed after reload", zap.Error(err))
		}
	}()
}

type reloadableTrace struct {
	service Service
	trace   Trace
}

func (t *reloadableTrace) GetID() string {
	if t == nil || t.trace == nil {
		return ""
	}
	return t.trace.GetID()
}

func (t *reloadableTrace) End() {
	if t != nil && t.trace != nil {
		t.trace.End()
	}
}
