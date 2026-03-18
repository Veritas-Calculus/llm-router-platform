package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MCPServer represents a Model Context Protocol server.
type MCPServer struct {
	BaseModel
	Name          string          `gorm:"uniqueIndex;not null" json:"name"`
	Type          string          `gorm:"not null" json:"type"` // "stdio", "sse"
	// Stdio configuration
	Command       string          `json:"command,omitempty"`
	Args          json.RawMessage `gorm:"type:jsonb" json:"args,omitempty"`
	Env           json.RawMessage `gorm:"type:jsonb" json:"env,omitempty"`
	// SSE configuration
	URL           string          `json:"url,omitempty"`
	
	IsActive      bool            `gorm:"default:true" json:"is_active"`
	Status        string          `gorm:"default:'disconnected'" json:"status"`
	LastError     string          `json:"last_error,omitempty"`
	LastCheckedAt time.Time       `json:"last_checked_at"`
	
	Tools         []MCPTool       `gorm:"foreignKey:ServerID" json:"tools,omitempty"`
}

// MCPTool represents a tool provided by an MCP server.
type MCPTool struct {
	BaseModel
	ServerID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"server_id"`
	Name        string          `gorm:"not null" json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `gorm:"type:jsonb" json:"input_schema"`
	IsActive    bool            `gorm:"default:true" json:"is_active"`
	
	Server      MCPServer       `gorm:"foreignKey:ServerID" json:"-"`
}
