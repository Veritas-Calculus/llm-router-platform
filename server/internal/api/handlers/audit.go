package handlers

import (
	"encoding/csv"
	"net/http"
	"time"

	"llm-router-platform/internal/service/audit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuditHandler provides HTTP endpoints for audit log operational tasks like CSV export.
type AuditHandler struct {
	auditService *audit.Service
	logger       *zap.Logger
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(auditService *audit.Service, logger *zap.Logger) *AuditHandler {
	return &AuditHandler{auditService: auditService, logger: logger}
}

// ExportCSV godoc
// @Summary Export audit logs to CSV
// @Description Streams audit logs as a downloadable CSV file.
// @Tags Audit
// @Produce text/csv
// @Param actor_id query string false "Filter by actor ID"
// @Param action query string false "Filter by action name"
// @Param start_at query string false "Filter start time (RFC3339)"
// @Param end_at query string false "Filter end time (RFC3339)"
// @Security BearerAuth
// @Router /api/v1/audit/export/csv [get]
func (h *AuditHandler) ExportCSV(c *gin.Context) {
	var filter audit.QueryFilter

	if actorIDStr := c.Query("actor_id"); actorIDStr != "" {
		if id, err := uuid.Parse(actorIDStr); err == nil {
			filter.ActorID = &id
		}
	}
	if action := c.Query("action"); action != "" {
		filter.Action = action
	}
	if startAtStr := c.Query("start_at"); startAtStr != "" {
		if t, err := time.Parse(time.RFC3339, startAtStr); err == nil {
			filter.StartAt = &t
		}
	}
	if endAtStr := c.Query("end_at"); endAtStr != "" {
		if t, err := time.Parse(time.RFC3339, endAtStr); err == nil {
			filter.EndAt = &t
		}
	}

	filename := "audit_export_" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	// Instruct Gin to flush the header directly to the client before writing the body
	c.Writer.WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Writer)
	err := h.auditService.ExportCSV(c.Request.Context(), filter, csvWriter)
	if err != nil {
		h.logger.Error("failed to stream audit logs to csv", zap.Error(err))
		return
	}
}
