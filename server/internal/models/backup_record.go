package models

import (
	"time"

	"github.com/google/uuid"
)

// BackupRecord tracks the execution history and status of database backups.
type BackupRecord struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Type        string     `gorm:"type:varchar(32);not null;default:'manual'" json:"type"` // manual | scheduled
	Status      string     `gorm:"type:varchar(32);not null;default:'running'" json:"status"` // running | success | failed
	SizeBytes   int64      `gorm:"default:0" json:"size_bytes"`
	DurationMs  int64      `gorm:"default:0" json:"duration_ms"`
	Destination string     `gorm:"type:text" json:"destination"` // s3://bucket/path
	ErrorMsg    string     `gorm:"type:text" json:"error_msg"`
	StartedAt   time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
}
