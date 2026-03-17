package models

import (
	"time"

	"github.com/google/uuid"
)

// HealthHistory records health check results.
type HealthHistory struct {
	BaseModel
	TargetType   string    `gorm:"not null;index" json:"target_type"`
	TargetID     uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	IsHealthy    bool      `json:"is_healthy"`
	ResponseTime int64     `json:"response_time"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `gorm:"index" json:"checked_at"`
}

// Alert represents a health alert.
type Alert struct {
	BaseModel
	TargetType     string    `gorm:"not null;index" json:"target_type"`
	TargetID       uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	AlertType      string    `gorm:"not null" json:"alert_type"`
	Message        string    `json:"message"`
	Status         string    `gorm:"default:active;index" json:"status"`
	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     time.Time `json:"resolved_at,omitempty"`
}

// AlertConfig stores alert configuration per target.
type AlertConfig struct {
	BaseModel
	TargetType       string    `gorm:"not null;index" json:"target_type"`
	TargetID         uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	IsEnabled        bool      `gorm:"default:true" json:"is_enabled"`
	FailureThreshold int       `gorm:"default:3" json:"failure_threshold"`
	WebhookURL       string    `json:"webhook_url,omitempty"`
	Email            string    `json:"email,omitempty"`
}
