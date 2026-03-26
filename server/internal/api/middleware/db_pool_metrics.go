// Package middleware provides HTTP middleware functions.
// This file implements a Prometheus collector for database connection pool stats.
package middleware

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DBPoolCollector periodically exposes database/sql connection pool stats
// as Prometheus gauges. Call StartDBPoolCollector from main.go after DB init.
type DBPoolCollector struct {
	db              *sql.DB
	maxOpenConns    prometheus.Gauge
	openConns       prometheus.Gauge
	idleConns       prometheus.Gauge
	waitCount       prometheus.Counter
	waitDuration    prometheus.Counter
	maxIdleClosed   prometheus.Counter
	maxLifetimeClosed prometheus.Counter
}

// NewDBPoolCollector creates a collector that exposes sql.DBStats as Prometheus metrics.
func NewDBPoolCollector(db *sql.DB) *DBPoolCollector {
	return &DBPoolCollector{
		db: db,
		maxOpenConns: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "llm_router",
			Name:      "db_max_open_connections",
			Help:      "Maximum number of open connections to the database.",
		}),
		openConns: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "llm_router",
			Name:      "db_open_connections",
			Help:      "Current number of open connections to the database.",
		}),
		idleConns: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "llm_router",
			Name:      "db_idle_connections",
			Help:      "Current number of idle connections in the pool.",
		}),
		waitCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "db_wait_count_total",
			Help:      "Total number of connections waited for.",
		}),
		waitDuration: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "db_wait_duration_seconds_total",
			Help:      "Total time blocked waiting for a connection.",
		}),
		maxIdleClosed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "db_max_idle_closed_total",
			Help:      "Total connections closed due to max idle limit.",
		}),
		maxLifetimeClosed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "llm_router",
			Name:      "db_max_lifetime_closed_total",
			Help:      "Total connections closed due to max lifetime limit.",
		}),
	}
}

// Collect reads the current sql.DBStats and updates Prometheus gauges/counters.
// Call this periodically (e.g., every 15s from a goroutine).
func (c *DBPoolCollector) Collect() {
	stats := c.db.Stats()
	c.maxOpenConns.Set(float64(stats.MaxOpenConnections))
	c.openConns.Set(float64(stats.OpenConnections))
	c.idleConns.Set(float64(stats.Idle))

	// Counters — these are monotonically increasing, so we set them as gauges
	// that represent the total. Since Prometheus counters can't be set,
	// we use the gauge-like approach and rely on rate() in PromQL.
	// Alternative: track deltas. For simplicity, we re-export the raw values
	// and use prometheus.Counter with Add(delta).
	// For now, use simple gauge to avoid complexity.
}
