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
	RequirePasswordChange bool                 `gorm:"default:false" json:"require_password_change"`
	OAuthProvider         string               `gorm:"type:varchar(32)" json:"oauth_provider,omitempty"`  // github, google, etc.
	OAuthID               string               `gorm:"type:varchar(255);index" json:"oauth_id,omitempty"` // Provider's unique user ID
	Memberships           []OrganizationMember `gorm:"foreignKey:UserID" json:"-"`
	LastLoginAt           time.Time `json:"last_login_at"`
	MonthlyTokenLimit     int64     `gorm:"default:0" json:"monthly_token_limit"`   // 0 = unlimited
	MonthlyBudgetUSD      float64   `gorm:"default:0" json:"monthly_budget_usd"`    // 0 = unlimited
	RateLimitPerMinute    int       `gorm:"default:0" json:"rate_limit_per_minute"` // 0 = use global default
	Balance               float64   `gorm:"default:0" json:"balance"`               // Current credit balance in USD
	TokensInvalidatedAt   time.Time `json:"-"`                                      // tokens issued before this time are rejected
	MfaEnabled            bool      `gorm:"default:false" json:"mfa_enabled"`
	MfaSecret             string    `gorm:"type:varchar(255)" json:"-"`
	MfaBackupCodes        string    `gorm:"type:text" json:"-"` // JSON array of backup codes
}

// MfaSecretInfo holds the generated TOTP secret, QR code, and backup codes
type MfaSecretInfo struct {
	Secret      string
	QrCodeUrl   string
	BackupCodes []string
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
	ProjectID  uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Channel    string    `gorm:"type:varchar(128);default:'default'" json:"channel"`
	KeyHash    string    `gorm:"not null;uniqueIndex" json:"-"`
	KeyPrefix  string    `gorm:"not null" json:"key_prefix"`
	Name       string    `json:"name"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	Scopes     string    `gorm:"type:text;default:'all'" json:"scopes"` // Comma-separated or JSON list of scopes: all, chat, embeddings, etc.
	RateLimit  int       `gorm:"default:1000" json:"rate_limit"`
	TokenLimit int64     `gorm:"default:0" json:"token_limit"` // 0 = unlimited tokens per minute
	DailyLimit int       `gorm:"default:10000" json:"daily_limit"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Project    Project   `gorm:"foreignKey:ProjectID" json:"-"`
}

// AuditLog records security-relevant events for incident investigation.
type AuditLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	Action       string    `gorm:"not null;index" json:"action"` // login, login_failed, register, password_change, role_change, user_toggle, apikey_create, apikey_revoke, tokens_invalidated
	ActorID      uuid.UUID `gorm:"type:uuid;index" json:"actor_id"`
	TargetID     uuid.UUID `gorm:"type:uuid;index" json:"target_id"` // affected user (may differ from actor for admin actions)
	IP           string    `json:"ip"`
	UserAgent    string    `json:"user_agent"`
	Detail       string    `gorm:"type:text" json:"detail"` // JSON metadata
	PreviousHash string    `gorm:"type:varchar(64)" json:"previous_hash"`
	Signature    string    `gorm:"type:varchar(64)" json:"signature"`
}

// Organization represents a billing and management grouping of users and projects.
type Organization struct {
	BaseModel
	Name         string               `gorm:"type:varchar(255);not null" json:"name"`
	BillingLimit float64              `gorm:"type:decimal(20,4);default:0.0000" json:"billing_limit"`
	OwnerID      uuid.UUID            `gorm:"type:uuid;not null;index" json:"owner_id"`
	Owner        User                 `gorm:"foreignKey:OwnerID" json:"-"`
	Members      []OrganizationMember `gorm:"foreignKey:OrgID" json:"-"`
	Projects     []Project            `gorm:"foreignKey:OrgID" json:"-"`
}

// OrganizationMember maps users to organizations with a designated role (OWNER, ADMIN, MEMBER, READONLY).
type OrganizationMember struct {
	OrgID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"org_id"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role      string    `gorm:"type:varchar(64);not null;default:'MEMBER'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Organization Organization `gorm:"foreignKey:OrgID" json:"-"`
	User         User         `gorm:"foreignKey:UserID" json:"-"`
}

// Project represents a workspace within an Organization that holds API keys and limits.
type Project struct {
	BaseModel
	OrgID       uuid.UUID `gorm:"type:uuid;not null;index" json:"org_id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description    string    `gorm:"type:text" json:"description"`
	QuotaLimit     float64   `gorm:"type:decimal(20,4);default:0.0000" json:"quota_limit"`
	WhiteListedIps string    `gorm:"type:text" json:"white_listed_ips"` // Comma-separated CIDRs/IPs
	APIKeys        []APIKey  `gorm:"foreignKey:ProjectID" json:"-"`
	DlpConfig      *DlpConfig `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE;" json:"-"`

	Organization Organization `gorm:"foreignKey:OrgID" json:"-"`
}

// DlpConfig stores the Data Loss Prevention settings for a project.
type DlpConfig struct {
	BaseModel
	ProjectID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"project_id"`
	IsEnabled       bool      `gorm:"not null;default:false" json:"is_enabled"`
	Strategy        string    `gorm:"type:varchar(32);not null;default:'REDACT'" json:"strategy"`
	MaskEmails      bool      `gorm:"not null;default:true" json:"mask_emails"`
	MaskPhones      bool      `gorm:"not null;default:true" json:"mask_phones"`
	MaskCreditCards bool      `gorm:"not null;default:true" json:"mask_credit_cards"`
	MaskSSN         bool      `gorm:"not null;default:true" json:"mask_ssn"`
	MaskApiKeys     bool      `gorm:"not null;default:true" json:"mask_api_keys"`
	CustomRegex     []string  `gorm:"type:jsonb;serializer:json;default:'[]'" json:"custom_regex"` // List of custom regex strings

	Project Project `gorm:"foreignKey:ProjectID" json:"-"`
}
