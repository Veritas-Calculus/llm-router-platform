package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

// PromptTemplate represents a managed prompt template.
type PromptTemplate struct {
	BaseModel
	Name            string          `gorm:"uniqueIndex;not null" json:"name"`
	Description     string          `json:"description"`
	ProjectID       *uuid.UUID      `gorm:"type:uuid;index" json:"project_id,omitempty"`
	IsActive        bool            `gorm:"default:true" json:"is_active"`
	ActiveVersionID *uuid.UUID      `gorm:"type:uuid" json:"active_version_id,omitempty"`

	Versions        []PromptVersion `gorm:"foreignKey:TemplateID" json:"versions,omitempty"`
}

// PromptVersion represents a versioned snapshot of a prompt template.
type PromptVersion struct {
	BaseModel
	TemplateID uuid.UUID       `gorm:"type:uuid;not null;index" json:"template_id"`
	Version    int             `gorm:"not null" json:"version"`
	Content    string          `gorm:"type:text;not null" json:"content"`
	Model      string          `json:"model,omitempty"`
	Parameters json.RawMessage `gorm:"type:jsonb" json:"parameters,omitempty"`
	ChangeLog  string          `json:"change_log,omitempty"`

	Template   PromptTemplate  `gorm:"foreignKey:TemplateID" json:"-"`
}
