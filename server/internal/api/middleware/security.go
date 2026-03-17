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

	// QuotaExceededTotal tracks requests rejected due to quota limits.
	QuotaExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "quota_exceeded_total",
			Help:      "Total number of requests rejected due to quota limits.",
		},
		[]string{"type"}, // token_limit, budget_limit
	)

	// RateLimitFallbackTotal tracks when the in-memory fallback rate limiter is engaged.
	RateLimitFallbackTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "ratelimit_fallback_total",
			Help:      "Number of times the in-memory rate limiter fallback was used.",
		},
	)

	// RateLimitExceededTotal tracks rate limit rejections by identifier.
	RateLimitExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "ratelimit_exceeded_total",
			Help:      "Total number of requests rejected due to rate limits.",
		},
		[]string{"identifier"},
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

		// Restrict resources to same-origin
		// CSP: self for all sources, no unsafe-inline if possible (but web app might need it), 
		// frame-ancestors 'none' to prevent any framing (clickjacking defense).
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';")

		// Disable browser features not needed by API
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")

		// R8: HSTS — enforce HTTPS for direct API access (complements nginx HSTS)
		// 1 year max-age, include subdomains and preload.
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Cache control for API responses (no caching sensitive data)
		// We explicitly exclude health checks to allow load balancer caching if needed.
		if c.Request.URL.Path != "/health" && c.Request.URL.Path != "/readyz" && c.Request.URL.Path != "/healthz" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private, proxy-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
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
