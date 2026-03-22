package models

import "github.com/google/uuid"

// NotificationChannel stores the configuration for a notification delivery channel.
type NotificationChannel struct {
	BaseModel
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Type      string    `gorm:"type:varchar(32);not null" json:"type"` // webhook, email, dingtalk, feishu
	IsEnabled bool      `gorm:"default:true" json:"is_enabled"`
	Config    string    `gorm:"type:text" json:"config"` // JSON config blob
	CreatedBy uuid.UUID `gorm:"type:uuid" json:"created_by"`
}
