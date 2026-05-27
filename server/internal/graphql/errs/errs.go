// Package errs provides typed GraphQL resolver errors that the GraphQL
// ErrorPresenter unwraps into stable extensions.code values for client
// branching.
//
// The presenter at server/internal/graphql/handler/handler.go masks any
// error it doesn't recognize as a generic "internal error [request_id]"
// to avoid leaking implementation details. Resolvers that need to surface
// a human-readable validation message (e.g. "invalid captcha token") must
// either:
//  1. return a string the presenter's classifyClientError already knows
//     about (the legacy approach — brittle string matching), or
//  2. wrap the failure in errs.Validation(field, message) (the new typed
//     approach — preferred for new resolver paths).
//
// The presenter uses errors.As to find a *ValidationError anywhere in
// the chain, so wrap()-style propagation through %w continues to work.
package errs

import (
	"errors"
	"fmt"
)

// ValidationError represents a user-input validation failure that should
// surface to the GraphQL client verbatim (not be masked as INTERNAL).
//
// Field is the input field name (e.g. "captchaToken", "password",
// "inviteCode") that the client form can highlight; Message is a
// human-readable English string safe to display to end users. The
// ErrorPresenter sets extensions.code = "VALIDATION" and
// extensions.field = Field for any error chain containing a
// *ValidationError.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface. The message is the user-facing
// text; the field prefix only appears when the error is logged as a
// bare string (the presenter reads .Field directly).
func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validation builds a typed validation error for the GraphQL presenter.
// Use this in resolver paths whose failure is a normal-flow user-input
// problem (bad captcha, weak password, missing invite code, etc.)
// rather than a server fault.
func Validation(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// AsValidation reports whether err's chain contains a *ValidationError
// and returns the pointer if so. Equivalent to errors.As with a typed
// receiver — provided as a convenience for the presenter.
func AsValidation(err error) (*ValidationError, bool) {
	if err == nil {
		return nil, false
	}
	var v *ValidationError
	if errors.As(err, &v) {
		return v, true
	}
	return nil, false
}
