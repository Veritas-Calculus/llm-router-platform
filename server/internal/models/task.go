package models

import (
	"time"

	"github.com/google/uuid"
)

// AsyncTask represents a long-running asynchronous task.
type AsyncTask struct {
	BaseModel
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Type        string     `gorm:"not null;index" json:"type"`           // "tts", "batch_tts", "video_analysis", "batch_image"
	Status      string     `gorm:"default:pending;index" json:"status"`  // pending, running, completed, failed, cancelled
	Input       string     `gorm:"type:text" json:"input"`               // JSON-encoded input parameters
	Result      string     `gorm:"type:text" json:"result,omitempty"`    // JSON-encoded result
	WebhookURL  string     `json:"webhook_url,omitempty"`                // Callback URL for completion notification
	Progress    int        `gorm:"default:0" json:"progress"`            // 0-100
	Error       string     `json:"error,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
