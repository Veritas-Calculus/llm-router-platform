package models

// Document represents a documentation page managed through the admin panel.
type Document struct {
	BaseModel
	Title       string `gorm:"not null" json:"title"`
	Slug        string `gorm:"uniqueIndex;not null" json:"slug"`
	Content     string `gorm:"type:text" json:"content"` // Markdown
	Category    string `gorm:"not null;default:'general';index" json:"category"` // getting-started, api-reference, faq, general
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
	IsPublished bool   `gorm:"default:false" json:"is_published"`
}
