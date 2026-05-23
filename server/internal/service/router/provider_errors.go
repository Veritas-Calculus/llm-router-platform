// Package router — provider_errors.go isolates the classifiers that decide
// whether an upstream-provider error should trip the circuit breaker
// (5xx-class) or rotate the API key (429/402/quota-flavored). The canonical
// signal is the upstream HTTP status carried on *provider.ProviderError;
// substring matching on the message string is the fallback path for
// callers without a typed error (raw net/http failures, context errors).
//
// Pulled out of provider_ops.go per the audit "大文件拆分" item so each
// concern owns its own ~100-line file.
package router

import (
	"errors"
	"net/http"
	"strings"

	"llm-router-platform/internal/service/provider"
)

// isProviderLevelError returns true if err should trip the provider's
// circuit breaker. 5xx-class HTTP status from the upstream is the canonical
// signal; substring matching covers transport-level failures.
func isProviderLevelError(err error) bool {
	if err == nil {
		return false
	}
	var pe *provider.ProviderError
	if errors.As(err, &pe) {
		return pe.StatusCode >= 500 && pe.StatusCode < 600
	}
	return messageLooksProviderLevel(err.Error())
}

// messageLooksProviderLevel keeps the legacy substring heuristic for callers
// that have no typed error (network failures from net/http, context errors).
func messageLooksProviderLevel(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	providerKeywords := []string{
		"timeout", "deadline exceeded", "connection refused",
		"500", "502", "503", "504", "internal server error",
		"bad gateway", "service unavailable", "gateway timeout",
	}
	for _, keyword := range providerKeywords {
		if strings.Contains(errLower, keyword) {
			return true
		}
	}
	return false
}

// isQuotaOrRateLimitError returns true if err should trigger an API-key
// rotation rather than a circuit-breaker trip. 429 is the canonical 429,
// 402 covers "this key is over budget" responses some providers use, and
// 403 is disambiguated by the body — auth failures stay with the key, only
// quota-flavored 403s rotate.
func isQuotaOrRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	var pe *provider.ProviderError
	if errors.As(err, &pe) {
		switch pe.StatusCode {
		case http.StatusTooManyRequests, http.StatusPaymentRequired:
			return true
		case http.StatusForbidden:
			return messageLooksQuotaLimited(string(pe.Body)) || messageLooksQuotaLimited(pe.Message)
		}
		return false
	}
	return messageLooksQuotaLimited(err.Error())
}

// messageLooksQuotaLimited is the legacy substring heuristic, retained as a
// fallback for non-typed errors and exposed for the unit test that pins the
// keyword list.
func messageLooksQuotaLimited(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	quotaKeywords := []string{
		"quota", "rate limit", "rate_limit", "ratelimit",
		"too many requests", "429", "insufficient_quota",
		"billing", "exceeded", "limit reached",
		"resource exhausted", "resourceexhausted",
	}
	for _, keyword := range quotaKeywords {
		if strings.Contains(errLower, keyword) {
			return true
		}
	}
	return false
}
