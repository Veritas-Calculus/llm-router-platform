package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a platform user.
type User struct {
	BaseModel
	Email                 string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash          string    `gorm:"not null" json:"-"`
	Name                  string    `json:"name"`
	Role                  string    `gorm:"default:user" json:"role"`
	IsActive              bool      `gorm:"default:true" json:"is_active"`
	RequirePasswordChange bool      `gorm:"default:false" json:"require_password_change"`
	APIKeys               []APIKey  `gorm:"foreignKey:UserID" json:"-"`
	LastLoginAt           time.Time `json:"last_login_at"`
	MonthlyTokenLimit     int64     `gorm:"default:0" json:"monthly_token_limit"`   // 0 = unlimited
	MonthlyBudgetUSD      float64   `gorm:"default:0" json:"monthly_budget_usd"`    // 0 = unlimited
	RateLimitPerMinute    int       `gorm:"default:0" json:"rate_limit_per_minute"` // 0 = use global default
	Balance               float64   `gorm:"default:0" json:"balance"`               // Current credit balance in USD
	TokensInvalidatedAt   time.Time `json:"-"`                                      // tokens issued before this time are rejected
}

// InviteCode represents a one-time or limited-use invite code for registration.
type InviteCode struct {
	BaseModel
	Code      string     `gorm:"uniqueIndex;not null" json:"code"`
	CreatedBy uuid.UUID  `gorm:"type:uuid;not null" json:"created_by"` // Admin who created this code
	MaxUses   int        `gorm:"default:1" json:"max_uses"`            // 0 = unlimited
	UseCount  int        `gorm:"default:0" json:"use_count"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = never expires
	IsActive  bool       `gorm:"default:true" json:"is_active"`
}

// IsValid returns true if the invite code can still be used.
func (ic *InviteCode) IsValid() bool {
	if !ic.IsActive {
		return false
	}
	if ic.MaxUses > 0 && ic.UseCount >= ic.MaxUses {
		return false
	}
	if ic.ExpiresAt != nil && ic.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// APIKey represents an API key for authentication.
type APIKey struct {
	BaseModel
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	KeyHash    string    `gorm:"not null;uniqueIndex" json:"-"`
	KeyPrefix  string    `gorm:"not null" json:"key_prefix"`
	Name       string    `json:"name"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	RateLimit  int       `gorm:"default:1000" json:"rate_limit"`
	DailyLimit int       `gorm:"default:10000" json:"daily_limit"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	User       User      `gorm:"foreignKey:UserID" json:"-"`
}

// AuditLog records security-relevant events for incident investigation.
type AuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	Action    string    `gorm:"not null;index" json:"action"` // login, login_failed, register, password_change, role_change, user_toggle, apikey_create, apikey_revoke, tokens_invalidated
	ActorID   uuid.UUID `gorm:"type:uuid;index" json:"actor_id"`
	TargetID  uuid.UUID `gorm:"type:uuid;index" json:"target_id"` // affected user (may differ from actor for admin actions)
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Detail    string    `gorm:"type:text" json:"detail"` // JSON metadata
}
