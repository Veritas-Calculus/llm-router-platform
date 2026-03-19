package models

import (
	"time"

	"github.com/google/uuid"
)

// RedeemCode represents a redeemable code for credits or plan activation.
type RedeemCode struct {
	BaseModel
	Code         string     `gorm:"uniqueIndex;size:32;not null" json:"code"`
	Type         string     `gorm:"not null;default:'credit'" json:"type"` // credit, plan
	CreditAmount float64    `gorm:"default:0" json:"credit_amount"`        // Amount to credit (for type=credit)
	PlanID       *uuid.UUID `gorm:"type:uuid;index" json:"plan_id"`        // Plan to activate (for type=plan)
	PlanDays     int        `gorm:"default:30" json:"plan_days"`           // Duration of plan activation
	UsedByID     *uuid.UUID `gorm:"type:uuid;index" json:"used_by_id"`
	UsedAt       *time.Time `json:"used_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	BatchID      string     `gorm:"index" json:"batch_id"` // Group codes from same generation batch
	Note         string     `json:"note"`

	UsedBy *User `gorm:"foreignKey:UsedByID" json:"used_by,omitempty"`
	Plan   *Plan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
}
