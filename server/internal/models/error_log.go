package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ErrorLog records detailed upstream routing errors for tracking and troubleshooting.
type ErrorLog struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TrajectoryID string         `gorm:"type:varchar(255);index" json:"trajectoryId"`
	TraceID      string         `gorm:"type:varchar(255);index" json:"traceId"`
	Provider     string         `gorm:"type:varchar(100);index" json:"provider"`
	Model        string         `gorm:"type:varchar(100);index" json:"model"`
	StatusCode   int            `json:"statusCode"`
	Headers      datatypes.JSON `json:"headers"`
	ResponseBody datatypes.JSON `json:"responseBody"`
	CreatedAt    time.Time      `gorm:"index" json:"createdAt"`
}
