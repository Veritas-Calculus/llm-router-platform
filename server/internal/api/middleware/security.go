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

	// RateLimitExceededTotal tracks rate limit rejections by source type.
	RateLimitExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "security",
			Name:      "ratelimit_exceeded_total",
			Help:      "Total number of requests rejected due to rate limits.",
		},
		[]string{"source"}, // per_key, per_user, fallback — fixed cardinality
	)

	// BillingLockTimeoutTotal counts FOR UPDATE waits that exceeded the
	// configured timeout and aborted the deduction transaction.
	BillingLockTimeoutTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "billing",
			Name:      "lock_timeout_total",
			Help:      "FOR UPDATE row-lock timeouts on the user balance row.",
		},
		[]string{"operation"}, // record_usage_and_deduct, deduct_balance, add_balance, update_usage_and_deduct
	)

	// LocalTokenFallbackTotal counts streaming responses whose token totals
	// were estimated locally because the provider did not return a usage block.
	// The estimate drives billing, so a sustained spike means we may be
	// over- or under-charging customers relative to provider invoices.
	LocalTokenFallbackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "billing",
			Name:      "local_token_fallback_total",
			Help:      "Streaming responses billed using locally estimated token counts.",
		},
		[]string{"provider", "model"},
	)

	// PaymentAmountMismatchTotal counts payment notifications where the
	// signed-event amount disagreed with the order amount we created. This
	// must always be zero in a healthy system; any non-zero value indicates
	// either a bug or a tampering attempt.
	PaymentAmountMismatchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "billing",
			Name:      "payment_amount_mismatch_total",
			Help:      "Payment webhook amounts that disagreed with the stored order amount.",
		},
		[]string{"provider"}, // stripe, alipay, wechat
	)
)

// ─── Security Response Headers ──────────────────────────────────────────

// SecurityHeaders adds the minimal set of HTTP headers that the Go backend
// is authoritative for. Everything else (CSP, HSTS, X-Frame-Options,
// Referrer-Policy, X-XSS-Protection, Permissions-Policy) is set at the
// edge by nginx — see web/snippets/security-headers.conf — to avoid the
// dual-source header drift that caused H-01/L-07.
//
// We deliberately keep two headers here as defense-in-depth for the rare
// case where this binary is reached directly (e.g. operator port-forward,
// internal docker network, or k8s probe bypassing the ingress):
//
//   - X-Content-Type-Options: nosniff — cheap, never harmful.
//   - Cache-Control: no-store on non-health API responses, so that even
//     direct backend hits don't accidentally cache auth/billing payloads.
//
// Health endpoints are intentionally excluded so LBs can cache liveness
// probes; nginx is the single source of cache-control for /healthz et al.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing — overlaps with nginx but is harmless
		// and protects direct backend access.
		c.Header("X-Content-Type-Options", "nosniff")

		// Cache control for API responses (no caching of sensitive data).
		// Health/readiness endpoints are excluded so LBs can cache them;
		// nginx remains the single source of truth for those paths.
		path := c.Request.URL.Path
		if path != "/health" && path != "/readyz" && path != "/healthz" {
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
