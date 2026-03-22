package models

import (
	"github.com/google/uuid"
)

// IdentityProvider represents an enterprise SSO configuration (OIDC or SAML) for an organization.
type IdentityProvider struct {
	BaseModel
	OrgID        uuid.UUID    `gorm:"type:uuid;not null;index" json:"org_id"`
	Organization Organization `gorm:"foreignKey:OrgID" json:"-"`
	
	Type         string       `gorm:"type:varchar(32);not null" json:"type"` // "oidc" or "saml"
	Name         string       `gorm:"type:varchar(128);not null" json:"name"` // e.g., "Okta", "Azure AD"
	IsActive     bool         `gorm:"default:true" json:"is_active"`
	
	// Domain mapping for tenant-discovery (e.g., "acme.com,corp.acme.com")
	Domains      string       `gorm:"type:text" json:"domains"`

	// OIDC specific fields
	OIDCClientID     string `gorm:"type:varchar(255)" json:"oidc_client_id,omitempty"`
	OIDCClientSecret string `gorm:"type:varchar(255)" json:"-"` // Store securely
	OIDCIssuerURL    string `gorm:"type:text" json:"oidc_issuer_url,omitempty"`
	
	// SAML specific fields
	SAMLEntityID   string `gorm:"type:text" json:"saml_entity_id,omitempty"`
	SAMLSSOURL     string `gorm:"type:text" json:"saml_sso_url,omitempty"`
	SAMLIdPCert    string `gorm:"type:text" json:"saml_idp_cert,omitempty"`

	// JIT Provisioning settings
	EnableJIT      bool   `gorm:"default:true" json:"enable_jit"`
	DefaultRole    string `gorm:"type:varchar(64);default:'MEMBER'" json:"default_role"` // default role for auto-provisioned members

	// JSON mapping of IdP group names to System Roles (e.g., {"engineering": "ADMIN", "contractors": "READONLY"})
	GroupRoleMapping string `gorm:"type:text" json:"group_role_mapping"`
}
