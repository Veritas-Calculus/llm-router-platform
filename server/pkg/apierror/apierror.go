// Package apierror provides standardized API error codes and responses.
// All API handlers should use these error constructors for consistent error formatting.
package apierror

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Code represents a standardized API error code.
type Code string

const (
	// Client errors
	CodeBadRequest         Code = "bad_request"
	CodeUnauthorized       Code = "unauthorized"
	CodeForbidden          Code = "forbidden"
	CodeNotFound           Code = "not_found"
	CodeConflict           Code = "conflict"
	CodeValidationError    Code = "validation_error"
	CodeQuotaExceeded      Code = "quota_exceeded"
	CodeRateLimitExceeded  Code = "rate_limit_exceeded"

	// Server errors
	CodeInternalError      Code = "internal_error"
	CodeServiceUnavailable Code = "service_unavailable"
	CodeProviderError      Code = "provider_error"
	CodeNotImplemented     Code = "not_implemented"
	CodeBadGateway         Code = "bad_gateway"
	CodeTimeout            Code = "timeout"

	// Auth errors
	CodeTokenExpired       Code = "token_expired"
	CodeTokenRevoked       Code = "token_revoked"
	CodeAccountDisabled    Code = "account_disabled"
	CodePasswordChangeReq  Code = "password_change_required"
	CodeInvalidCredentials Code = "invalid_credentials"
)

// APIError represents a structured API error response.
type APIError struct {
	HTTPStatus int    `json:"-"`
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Respond writes the error response to the gin context and aborts.
func (e *APIError) Respond(c *gin.Context) {
	c.AbortWithStatusJSON(e.HTTPStatus, gin.H{
		"error": gin.H{
			"code":    e.Code,
			"message": e.Message,
			"detail":  e.Detail,
		},
	})
}

// ─── Constructors ──────────────────────────────────────────────────────────

// BadRequest creates a 400 error.
func BadRequest(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusBadRequest, Code: CodeBadRequest, Message: message}
}

// Unauthorized creates a 401 error.
func Unauthorized(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusUnauthorized, Code: CodeUnauthorized, Message: message}
}

// Forbidden creates a 403 error.
func Forbidden(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusForbidden, Code: CodeForbidden, Message: message}
}

// NotFound creates a 404 error.
func NotFound(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusNotFound, Code: CodeNotFound, Message: message}
}

// QuotaExceeded creates a 429 quota error.
func QuotaExceeded(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusTooManyRequests, Code: CodeQuotaExceeded, Message: message}
}

// RateLimited creates a 429 rate limit error.
func RateLimited(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusTooManyRequests, Code: CodeRateLimitExceeded, Message: message}
}

// Internal creates a 500 error.
func Internal(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusInternalServerError, Code: CodeInternalError, Message: message}
}

// ServiceUnavailable creates a 503 error.
func ServiceUnavailable(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusServiceUnavailable, Code: CodeServiceUnavailable, Message: message}
}

// ProviderError creates a 502 provider error.
func ProviderError(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusBadGateway, Code: CodeProviderError, Message: message}
}

// NotImplementedError creates a 501 error.
func NotImplementedError(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusNotImplemented, Code: CodeNotImplemented, Message: message}
}

// WithDetail adds additional detail to the error.
func (e *APIError) WithDetail(detail string) *APIError {
	e.Detail = detail
	return e
}
