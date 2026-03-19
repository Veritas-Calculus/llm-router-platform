package models

import "time"

// Announcement represents a system-wide announcement or notice.
type Announcement struct {
	BaseModel
	Title    string     `gorm:"not null" json:"title"`
	Content  string     `gorm:"type:text;not null" json:"content"` // Markdown
	Type     string     `gorm:"not null;default:'info'" json:"type"` // info, warning, maintenance
	Priority int        `gorm:"default:0" json:"priority"`
	IsActive bool       `gorm:"default:false" json:"is_active"`
	StartsAt *time.Time `json:"starts_at,omitempty"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}
