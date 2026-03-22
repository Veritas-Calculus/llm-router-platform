// Package errors provides structured error codes for the LLM Router Platform.
package errors

import (
	"fmt"
)

// ErrorCode is a string prefix used to categorize and track specific failure mechanisms.
type ErrorCode string

const (
	// ErrCodeProxyTimeout indicates the upstream provider timed out.
	ErrCodeProxyTimeout ErrorCode = "LLM_ROUTER_ERR_001"

	// ErrCodeRateLimitExceeded indicates the user or org exceeded token/request rate limits.
	ErrCodeRateLimitExceeded ErrorCode = "LLM_ROUTER_ERR_002"

	// ErrCodeContextLengthExceeded indicates the user request exceeded max model tokens.
	ErrCodeContextLengthExceeded ErrorCode = "LLM_ROUTER_ERR_003"

	// ErrCodeProviderParseFailed indicates the upstream payload was invalid or unparseable.
	ErrCodeProviderParseFailed ErrorCode = "LLM_ROUTER_ERR_004"

	// ErrCodeAuthenticationFailed indicates the user API key or session token is invalid.
	ErrCodeAuthenticationFailed ErrorCode = "LLM_ROUTER_ERR_005"

	// ErrCodeInsufficientFunds indicates the user has no balance or active subscription.
	ErrCodeInsufficientFunds ErrorCode = "LLM_ROUTER_ERR_006"

	// ErrCodeInternalSystemError indicates an unhandled server error, DB connection failure, etc.
	ErrCodeInternalSystemError ErrorCode = "LLM_ROUTER_ERR_007"
	
	// ErrCodeModelNotFound indicates the requested virtual model does not exist or isn't accessible.
	ErrCodeModelNotFound ErrorCode = "LLM_ROUTER_ERR_008"
	
	// ErrCodeProviderQuotaExceeded indicates the upstream proxy provider (e.g. OpenAI) threw a 429 quota error.
	ErrCodeProviderQuotaExceeded ErrorCode = "LLM_ROUTER_ERR_009"
)

// RouterError implements the built-in error interface while carrying machine-readable dimensions.
// This structure maps nicely into OpenAI's native API error envelope:
// { "error": { "code": "...", "message": "...", "type": "server_error" } }
type RouterError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
	Type       string // e.g. "invalid_request_error", "server_error"
	InnerError error
}

func (e *RouterError) Error() string {
	if e.InnerError != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.InnerError)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewRouterError creates a structured domain error tracking.
func NewRouterError(code ErrorCode, httpStatus int, typ string, msg string, inner error) *RouterError {
	return &RouterError{
		Code:       code,
		Message:    msg,
		HTTPStatus: httpStatus,
		Type:       typ,
		InnerError: inner,
	}
}

// MapToOpenAIResponse converts the internal RouterError to the structure the OpenAI SDK expects.
func (e *RouterError) MapToOpenAIResponse() map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"message": e.Message,
			"type":    e.Type,
			"code":    string(e.Code),
		},
	}
}
