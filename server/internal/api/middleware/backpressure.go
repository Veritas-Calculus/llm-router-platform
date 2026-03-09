package middleware

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// BackpressureMiddleware provides system-level traffic shedding based on resource constraints.
type BackpressureMiddleware struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewBackpressure creates a new BackpressureMiddleware.
func NewBackpressure(db *sql.DB, logger *zap.Logger) *BackpressureMiddleware {
	return &BackpressureMiddleware{
		db:     db,
		logger: logger,
	}
}

// Protect calculates real-time resource usage and sheds load if the system is saturated.
func (m *BackpressureMiddleware) Protect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.db != nil {
			stats := m.db.Stats()

			// Shed load if database connections are over 85% saturated
			if stats.MaxOpenConnections > 0 {
				ratio := float64(stats.InUse) / float64(stats.MaxOpenConnections)
				if ratio > 0.85 {
					m.logger.Warn("backpressure activated: shedding load due to high DB connection usage",
						zap.Int("in_use", stats.InUse),
						zap.Int("max_open", stats.MaxOpenConnections),
						zap.Float64("ratio", ratio),
					)

					c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
						"error": gin.H{
							"message": "system is currently under heavy load, please try again momentarily",
							"type":    "server_error",
							"code":    "backpressure_shedding",
						},
					})
					return
				}
			}
		}

		c.Next()
	}
}
