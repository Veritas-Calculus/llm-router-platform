package models

// SystemConfig represents dynamic system settings stored in DB.
type SystemConfig struct {
	BaseModel
	Key         string `gorm:"uniqueIndex;not null" json:"key"`
	Value       string `gorm:"type:text" json:"value"`
	Description string `json:"description"`
	Category    string `gorm:"index" json:"category"` // payment, general, auth
	IsSecret    bool   `gorm:"default:false" json:"-"` // If true, value is encrypted
}
