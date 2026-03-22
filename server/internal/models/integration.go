package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// IntegrationConfig represents configuration for external tracking platforms.
type IntegrationConfig struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"type:varchar(100);uniqueIndex" json:"name"` // sentry, loki, langfuse
	Enabled   bool           `json:"enabled"`
	Config    datatypes.JSON `json:"config"` // DSN, Endpoint, API Keys, etc.
	UpdatedAt time.Time      `json:"updatedAt"`
	CreatedAt time.Time      `json:"createdAt"`
}
