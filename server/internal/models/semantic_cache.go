package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type SemanticCache struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Hash      string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"hash"`
	Embedding pgvector.Vector `gorm:"type:vector(1536);not null" json:"-"`
	Response  datatypes.JSON `gorm:"type:jsonb;not null" json:"response"`
	Provider  string         `gorm:"type:varchar(64);not null" json:"provider"`
	Model     string         `gorm:"type:varchar(64);not null" json:"model"`
	Metadata  datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	HitCount  int            `gorm:"default:0" json:"hitCount"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
