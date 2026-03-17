package models

import "github.com/google/uuid"

// ConversationMemory stores conversation context.
type ConversationMemory struct {
	BaseModel
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	ConversationID string    `gorm:"not null;index" json:"conversation_id"`
	Role           string    `gorm:"not null" json:"role"`
	Content        string    `gorm:"type:text" json:"content"`
	TokenCount     int       `json:"token_count"`
	Sequence       int       `gorm:"not null" json:"sequence"`
}
