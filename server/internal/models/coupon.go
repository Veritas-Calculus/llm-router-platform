package models

import "time"

// Coupon represents a discount or promotional code.
type Coupon struct {
	BaseModel
	Code           string     `gorm:"uniqueIndex;size:32;not null" json:"code"`
	Name           string     `gorm:"not null" json:"name"`
	Type           string     `gorm:"not null;default:'percent'" json:"type"` // percent, fixed
	DiscountValue  float64    `gorm:"not null" json:"discount_value"`         // percentage (0-100) or fixed USD amount
	MinAmount      float64    `gorm:"default:0" json:"min_amount"`            // minimum order amount
	MaxUses        int        `gorm:"default:0" json:"max_uses"`              // 0 = unlimited
	UseCount       int        `gorm:"default:0" json:"use_count"`
	MaxUsesPerUser int        `gorm:"default:1" json:"max_uses_per_user"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

// IsValid returns true if the coupon can still be used.
func (c *Coupon) IsValid() bool {
	if !c.IsActive {
		return false
	}
	if c.MaxUses > 0 && c.UseCount >= c.MaxUses {
		return false
	}
	if c.ExpiresAt != nil && c.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}
