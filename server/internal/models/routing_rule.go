package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RoutingRule defines visual routing logic for incoming models.
type RoutingRule struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name               string         `gorm:"type:varchar(255);not null" json:"name"`
	Description        string         `gorm:"type:text" json:"description"`
	ModelPattern       string         `gorm:"type:varchar(255);not null" json:"model_pattern"` // Regex or exact match
	TargetProviderID   uuid.UUID      `gorm:"type:uuid;not null" json:"target_provider_id"`
	FallbackProviderID *uuid.UUID     `gorm:"type:uuid" json:"fallback_provider_id"`           // Optional fallback
	Priority           int            `gorm:"type:integer;not null;default:0" json:"priority"` // Higher executes first
	IsEnabled          bool           `gorm:"type:boolean;not null;default:true" json:"is_enabled"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	TargetProvider     *Provider `gorm:"foreignKey:TargetProviderID" json:"target_provider,omitempty"`
	FallbackProvider   *Provider `gorm:"foreignKey:FallbackProviderID" json:"fallback_provider,omitempty"`
}
