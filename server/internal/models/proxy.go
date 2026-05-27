package models

import (
	"time"

	"github.com/google/uuid"
)

// Proxy represents a proxy server.
type Proxy struct {
	BaseModel
	PoolID            *uuid.UUID `gorm:"type:uuid;index" json:"pool_id,omitempty"`
	Pool              *ProxyPool `gorm:"foreignKey:PoolID" json:"-"`
	URL               string     `gorm:"not null" json:"url"`
	Type              string     `gorm:"default:http" json:"type"`
	Username          string     `json:"username,omitempty"`
	EncryptedPassword string     `gorm:"->" json:"-"`              // Read-only, for migration
	Password          string     `gorm:"column:password" json:"-"` // Encrypted password
	Region            string     `json:"region"`
	UpstreamProxyID   *uuid.UUID `gorm:"type:uuid;index" json:"upstream_proxy_id,omitempty"`
	UpstreamProxy     *Proxy     `gorm:"foreignKey:UpstreamProxyID" json:"-"`
	IsActive          bool       `gorm:"default:true" json:"is_active"`
	Weight            float64    `gorm:"default:1.0" json:"weight"`
	SuccessCount      int64      `gorm:"default:0" json:"success_count"`
	FailureCount      int64      `gorm:"default:0" json:"failure_count"`
	AvgLatency        float64    `gorm:"default:0" json:"avg_latency"`
	LastChecked       time.Time  `json:"last_checked"`
}

// HasAuth returns true if the proxy has authentication configured.
func (p *Proxy) HasAuth() bool {
	return p.Username != "" && p.Password != ""
}

// ProxyPool groups proxy nodes for account-level routing.
type ProxyPool struct {
	BaseModel
	Name        string  `gorm:"uniqueIndex;not null" json:"name"`
	Description string  `json:"description"`
	IsActive    bool    `gorm:"default:true;index:idx_proxy_pools_is_active" json:"is_active"`
	Strategy    string  `gorm:"default:weighted" json:"strategy"`
	Proxies     []Proxy `gorm:"foreignKey:PoolID" json:"proxies,omitempty"`
}
