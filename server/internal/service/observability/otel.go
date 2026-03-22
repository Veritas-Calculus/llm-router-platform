package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"llm-router-platform/internal/config"
)

// OTelService implements Observability Service using standard OpenTelemetry Go SDK.
type OTelService struct {
	logger        *zap.Logger
	tracer        trace.Tracer
	meter         metric.Meter
	traceProvider *sdktrace.TracerProvider
	meterProvider *sdkmetric.MeterProvider

	// Metrics
	promptTokensCounter     metric.Int64Counter
	completionTokensCounter metric.Int64Counter
	generationDuration      metric.Float64Histogram
	errorCounter            metric.Int64Counter
}

// NewOTelService creates and boots the OpenTelemetry trace/metric pipelines.
func NewOTelService(ctx context.Context, cfg config.ObservabilityConfig, logger *zap.Logger) Service {
	if !cfg.OTelEnabled || cfg.OTelEndpoint == "" {
		return NewNoopService()
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.OTelServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		logger.Error("failed to create otel resource", zap.Error(err))
		return NewNoopService()
	}

	// 1. Trace Provider
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(cfg.OTelEndpoint), otlptracehttp.WithInsecure())
	if err != nil {
		logger.Error("failed to create otel trace exporter", zap.Error(err))
		return NewNoopService()
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)

	// 2. Meter Provider
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(cfg.OTelEndpoint), otlpmetrichttp.WithInsecure())
	if err != nil {
		logger.Error("failed to create otel metric exporter", zap.Error(err))
		return NewNoopService()
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	otel.SetMeterProvider(mp)

	// 3. Instruments
	tracer := tp.Tracer(cfg.OTelServiceName)
	meter := mp.Meter(cfg.OTelServiceName)

	promptTokens, _ := meter.Int64Counter("llm.generation.prompt_tokens", metric.WithDescription("Prompt tokens consumed"))
	completionTokens, _ := meter.Int64Counter("llm.generation.completion_tokens", metric.WithDescription("Completion tokens consumed"))
	genDuration, _ := meter.Float64Histogram("llm.generation.durationSeconds", metric.WithUnit("s"), metric.WithDescription("Generation latency stringency recording"))
	errCnt, _ := meter.Int64Counter("llm.generation.errors", metric.WithDescription("Number of LLM generation errors across proxies"))

	logger.Info("OpenTelemetry observability initialized successfully", zap.String("endpoint", cfg.OTelEndpoint))

	return &OTelService{
		logger:                  logger,
		tracer:                  tracer,
		meter:                   meter,
		traceProvider:           tp,
		meterProvider:           mp,
		promptTokensCounter:     promptTokens,
		completionTokensCounter: completionTokens,
		generationDuration:      genDuration,
		errorCounter:            errCnt,
	}
}

func (s *OTelService) StartTrace(ctx context.Context, id, name, userID, sessionID string, metadata map[string]interface{}) Trace {
	ctx, span := s.tracer.Start(ctx, name)

	if id != "" {
		span.SetAttributes(attribute.String("trace.id.override", id))
	}
	span.SetAttributes(attribute.String("user.id", userID))
	span.SetAttributes(attribute.String("session.id", sessionID))

	for k, v := range metadata {
		if sVal, ok := v.(string); ok {
			span.SetAttributes(attribute.String("meta."+k, sVal))
		}
	}

	return &OTelTrace{ctx: ctx, span: span}
}

func (s *OTelService) StartGeneration(ctx context.Context, t Trace, name, model string, modelParams map[string]interface{}, input interface{}) Generation {
	ot, ok := t.(*OTelTrace)
	if !ok {
		return &NoopGeneration{}
	}

	genCtx, span := s.tracer.Start(ot.ctx, name)
	span.SetAttributes(attribute.String("llm.model", model))

	return &OTelGeneration{
		service:   s,
		ctx:       genCtx,
		span:      span,
		startTime: time.Now(),
		model:     model,
	}
}

func (s *OTelService) Shutdown(ctx context.Context) error {
	s.logger.Info("flushing and shutting down OpenTelemetry streams")
	_ = s.traceProvider.Shutdown(ctx)
	_ = s.meterProvider.Shutdown(ctx)
	return nil
}

type OTelTrace struct {
	ctx  context.Context
	span trace.Span
}

func (t *OTelTrace) GetID() string {
	if t.span.SpanContext().HasTraceID() {
		return t.span.SpanContext().TraceID().String()
	}
	return ""
}

func (t *OTelTrace) End() {
	t.span.End()
}

type OTelGeneration struct {
	service   *OTelService
	ctx       context.Context
	span      trace.Span
	startTime time.Time
	model     string
}

func (g *OTelGeneration) End(output string, promptTokens, completionTokens int) {
	dur := time.Since(g.startTime).Seconds()

	g.span.SetAttributes(
		attribute.Int("llm.usage.prompt_tokens", promptTokens),
		attribute.Int("llm.usage.completion_tokens", completionTokens),
		attribute.Int("llm.usage.total_tokens", promptTokens+completionTokens),
	)
	g.span.End()

	attrs := metric.WithAttributes(attribute.String("model", g.model))
	g.service.promptTokensCounter.Add(context.Background(), int64(promptTokens), attrs)
	g.service.completionTokensCounter.Add(context.Background(), int64(completionTokens), attrs)
	g.service.generationDuration.Record(context.Background(), dur, attrs)
}

func (g *OTelGeneration) EndWithError(err error) {
	g.span.RecordError(err)
	g.span.End()

	attrs := metric.WithAttributes(attribute.String("model", g.model))
	g.service.errorCounter.Add(context.Background(), 1, attrs)
}
