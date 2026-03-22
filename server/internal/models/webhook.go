package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// StringArray is a custom type to handle JSON array of strings in PostgreSQL
type StringArray []string

func (a *StringArray) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, &a)
}

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "[]", nil
	}
	return json.Marshal(a)
}

// WebhookEndpoint represents a destination URL configured by a tenant to receive events.
// A webhook belongs to a specific Project.
type WebhookEndpoint struct {
	BaseModel
	ProjectID   uuid.UUID   `gorm:"type:uuid;not null;index" json:"projectId"`
	Project     *Project    `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	URL         string      `gorm:"type:text;not null" json:"url"`
	Secret      string      `gorm:"type:varchar(255);not null" json:"-"` // Hidden from JSON serialization, used to compute HMAC
	Events      StringArray `gorm:"type:jsonb;not null;default:'[]'" json:"events"`
	IsActive    bool        `gorm:"not null;default:true" json:"isActive"`
	Description string      `gorm:"type:text" json:"description"`
}

// WebhookDelivery records the history and result of a webhook payload dispatch.
type WebhookDelivery struct {
	BaseModel
	EndpointID   uuid.UUID        `gorm:"type:uuid;not null;index" json:"endpointId"`
	Endpoint     *WebhookEndpoint `gorm:"foreignKey:EndpointID;constraint:OnDelete:CASCADE" json:"endpoint,omitempty"`
	EventType    string           `gorm:"type:varchar(100);not null;index" json:"eventType"`
	Payload      string           `gorm:"type:jsonb;not null" json:"payload"` // The actual JSON sent
	Status       string           `gorm:"type:varchar(50);not null;default:'pending';index" json:"status"` // pending, success, failed
	StatusCode   int              `gorm:"type:int;not null;default:0" json:"statusCode"`
	ResponseBody string           `gorm:"type:text" json:"responseBody"` // Truncated response body from destination
	ErrorMessage string           `gorm:"type:text" json:"errorMessage"`
	RetryCount   int              `gorm:"type:int;not null;default:0" json:"retryCount"`
}
