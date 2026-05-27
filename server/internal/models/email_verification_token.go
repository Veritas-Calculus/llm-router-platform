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

	// email_verification_tokens_user_id_fkey: CASCADE — orphan tokens are
	// useless if the user row is gone (added in migration 000023). The
	// explicit constraint name matches the Postgres-default SQL name.
	User *User `gorm:"foreignKey:UserID;constraint:email_verification_tokens_user_id_fkey,OnDelete:CASCADE" json:"-"`
}

// IsValid returns true if the token has not expired and has not been used.
func (t *EmailVerificationToken) IsValid() bool {
	if t.UsedAt != nil {
		return false
	}
	return time.Now().Before(t.ExpiresAt)
}
