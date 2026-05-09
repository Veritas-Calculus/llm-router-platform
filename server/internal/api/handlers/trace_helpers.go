package handlers

import (
	"strings"

	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/service/observability"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func requestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get(middleware.RequestIDKey); ok {
		if requestID, ok := v.(string); ok && requestID != "" {
			return requestID
		}
	}

	if requestID := c.GetHeader(middleware.RequestIDHeader); requestID != "" {
		c.Set(middleware.RequestIDKey, requestID)
		c.Header(middleware.RequestIDHeader, requestID)
		return requestID
	}

	requestID := uuid.New().String()
	c.Set(middleware.RequestIDKey, requestID)
	c.Header(middleware.RequestIDHeader, requestID)
	return requestID
}

func (h *ChatHandler) startRequestTrace(c *gin.Context, name, userID, sessionID string, metadata map[string]interface{}) observability.Trace {
	requestID := requestIDFromContext(c)
	traceMetadata := make(map[string]interface{}, len(metadata)+1)
	for key, value := range metadata {
		traceMetadata[key] = value
	}
	traceMetadata["request_id"] = requestID

	trace := h.obsInfo.StartTrace(c.Request.Context(), requestID, name, userID, sessionID, traceMetadata)
	traceID := requestID
	if trace != nil {
		if id := strings.TrimSpace(trace.GetID()); id != "" {
			traceID = id
		}
	}

	c.Set(middleware.TraceIDKey, traceID)
	c.Header("X-Trace-Id", traceID)
	c.Header("X-Langfuse-Trace-Id", traceID)
	return trace
}
