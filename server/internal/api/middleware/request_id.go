// Package middleware provides HTTP middleware functions.
// This file implements the Request ID middleware for distributed tracing.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// RequestIDHeader is the header name for the request ID.
	RequestIDHeader = "X-Request-Id"
	// RequestIDKey is the context key for the request ID.
	RequestIDKey = "request_id"
)

// RequestIDMiddleware generates or propagates a unique request ID for each request.
type RequestIDMiddleware struct {
	logger *zap.Logger
}

// NewRequestIDMiddleware creates a new request ID middleware.
func NewRequestIDMiddleware(logger *zap.Logger) *RequestIDMiddleware {
	return &RequestIDMiddleware{logger: logger}
}

// Handle generates a unique request ID for each request.
// If the incoming request already has an X-Request-Id header, it is reused.
func (m *RequestIDMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for downstream use
		c.Set(RequestIDKey, requestID)

		// Set response header so clients can correlate
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}
