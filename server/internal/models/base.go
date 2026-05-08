// Package models defines database models for the application.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides common fields for all models.
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
