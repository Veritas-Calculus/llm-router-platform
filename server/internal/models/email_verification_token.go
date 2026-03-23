package models

import (
	"time"

	"github.com/google/uuid"
)

// EmailVerificationToken stores hashed email verification tokens with expiry and single-use enforcement.
type EmailVerificationToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"` // nil = not yet used
}

// IsValid returns true if the token has not expired and has not been used.
func (t *EmailVerificationToken) IsValid() bool {
	if t.UsedAt != nil {
		return false
	}
	return time.Now().Before(t.ExpiresAt)
}
