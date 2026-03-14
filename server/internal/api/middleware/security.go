// Package middleware provides HTTP middleware functions.
// This file implements security-specific middleware and metrics.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─── Security Prometheus Metrics ────────────────────────────────────────

var (
	// AuthFailuresTotal tracks authentication failures by type.
	AuthFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "auth_failures_total",
			Help:      "Total number of authentication failures.",
		},
		[]string{"type"}, // login_failed, invalid_token, token_revoked, account_disabled, rate_limited
	)

	// RateLimitFailOpenTotal tracks when rate limiting fails open due to Redis errors.
	RateLimitFailOpenTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "ratelimit_failopen_total",
			Help:      "Number of times rate limiting failed open due to Redis unavailability.",
		},
	)

	// ActiveUsersGauge tracks the number of unique users making requests.
	QuotaExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "quota_exceeded_total",
			Help:      "Total number of requests rejected due to quota limits.",
		},
		[]string{"type"}, // token_limit, budget_limit
	)
)

// ─── Security Response Headers ──────────────────────────────────────────

// SecurityHeaders adds security-related HTTP headers to all API responses.
// These complement the nginx headers for direct API access scenarios.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Disable XSS auditor (modern browsers don't need it, can cause issues)
		c.Header("X-XSS-Protection", "0")

		// Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy — restrict resources to same-origin
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")

		// Disable browser features not needed by API
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// Cache control for API responses (no caching sensitive data)
		if c.Request.URL.Path != "/health" && c.Request.URL.Path != "/readyz" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
		}

		c.Next()
	}
}

// BodySizeLimit limits the request body size to prevent OOM attacks.
func BodySizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
