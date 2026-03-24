// Package handlers provides HTTP request handlers.
// This file contains operational endpoint handlers (health, readiness, version).
package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// OperationalHandler handles operational endpoints (health, readiness, version).
type OperationalHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	version     string
	gitCommit   string
	buildTime   string
}

// NewOperationalHandler creates a new OperationalHandler.
func NewOperationalHandler(db *gorm.DB, redisClient *redis.Client, version, gitCommit, buildTime string) *OperationalHandler {
	return &OperationalHandler{
		db:          db,
		redisClient: redisClient,
		version:     version,
		gitCommit:   gitCommit,
		buildTime:   buildTime,
	}
}

// Liveness is a basic liveness probe (always returns ok).
// GET /health
func (h *OperationalHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeepHealth checks PG + Redis connectivity and migration status.
// GET /healthz
func (h *OperationalHandler) DeepHealth(c *gin.Context) {
	checks := gin.H{}
	healthy := true

	// Check PostgreSQL
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			checks["postgres"] = gin.H{"status": "error"}
			healthy = false
		} else {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := sqlDB.PingContext(ctx); err != nil {
				checks["postgres"] = gin.H{"status": "error"}
				healthy = false
			} else {
				checks["postgres"] = gin.H{"status": "ok"}
			}

			// Report migration version (best-effort; does not affect health status)
			var version int
			var dirty bool
			row := sqlDB.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1")
			if err := row.Scan(&version, &dirty); err == nil {
				migrationStatus := "ok"
				if dirty {
					migrationStatus = "dirty"
				}
				checks["migration"] = gin.H{"status": migrationStatus, "version": version, "dirty": dirty}
			}
		}
	}

	// Check Redis
	if h.redisClient != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := h.redisClient.Ping(ctx).Err(); err != nil {
			checks["redis"] = gin.H{"status": "error"}
			healthy = false
		} else {
			checks["redis"] = gin.H{"status": "ok"}
		}
	}

	status := "ok"
	httpCode := http.StatusOK
	if !healthy {
		status = "degraded"
		httpCode = http.StatusServiceUnavailable
	}

	c.JSON(httpCode, gin.H{
		"status": status,
		"checks": checks,
	})
}

// Readiness checks that critical dependencies are available (for K8s).
// GET /readyz
func (h *OperationalHandler) Readiness(c *gin.Context) {
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// Version returns build version information.
// GET /version
func (h *OperationalHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    h.version,
		"git_commit": h.gitCommit,
		"build_time": h.buildTime,
	})
}
