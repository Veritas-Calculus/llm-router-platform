// Package middleware provides HTTP middleware functions.
// This file implements Prometheus metrics collection.
package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metric collectors.
type Metrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight prometheus.Gauge
	responseSize     *prometheus.HistogramVec
	tokensConsumed   *prometheus.CounterVec
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "llm_router",
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests processed.",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "llm_router",
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"method", "path"},
		),
		requestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "llm_router",
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being processed.",
			},
		),
		responseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "llm_router",
				Name:      "http_response_size_bytes",
				Help:      "HTTP response size in bytes.",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 7), // 100B to 100MB
			},
			[]string{"method", "path"},
		),
		tokensConsumed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "llm_router",
				Name:      "tokens_consumed_total",
				Help:      "Total number of LLM tokens consumed.",
			},
			[]string{"provider", "model", "direction"}, // direction: input/output
		),
	}
}

// Middleware returns a Gin middleware that records HTTP metrics.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Normalize path to avoid high-cardinality label explosion
		path := normalizePath(c.FullPath())
		if path == "" {
			path = "unknown"
		}
		method := c.Request.Method

		m.requestsInFlight.Inc()
		start := time.Now()

		c.Next()

		m.requestsInFlight.Dec()

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()
		size := float64(c.Writer.Size())

		m.requestsTotal.WithLabelValues(method, path, status).Inc()
		m.requestDuration.WithLabelValues(method, path).Observe(duration)
		if size > 0 {
			m.responseSize.WithLabelValues(method, path).Observe(size)
		}
	}
}

// RecordTokens records token consumption for a specific provider/model.
func (m *Metrics) RecordTokens(provider, model string, inputTokens, outputTokens int) {
	if inputTokens > 0 {
		m.tokensConsumed.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		m.tokensConsumed.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
	}
}

// MetricsHandler returns the Prometheus metrics HTTP handler.
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// normalizePath converts parameterized Gin paths to stable label values.
// e.g., /api/v1/providers/123/api-keys → /api/v1/providers/:id/api-keys
func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	return path
}
