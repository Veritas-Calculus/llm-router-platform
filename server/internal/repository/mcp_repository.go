package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MCPRepository handles MCP server and tool data access.
type MCPRepository struct {
	db *gorm.DB
}

// NewMCPRepository creates a new MCP repository.
func NewMCPRepository(db *gorm.DB) *MCPRepository {
	return &MCPRepository{db: db}
}

// CreateServer inserts a new MCP server.
func (r *MCPRepository) CreateServer(ctx context.Context, server *models.MCPServer) error {
	return r.db.WithContext(ctx).Create(server).Error
}

// GetServerByID retrieves an MCP server by ID.
func (r *MCPRepository) GetServerByID(ctx context.Context, id uuid.UUID) (*models.MCPServer, error) {
	var server models.MCPServer
	if err := r.db.WithContext(ctx).First(&server, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

// GetServerByName retrieves an MCP server by name.
func (r *MCPRepository) GetServerByName(ctx context.Context, name string) (*models.MCPServer, error) {
	var server models.MCPServer
	if err := r.db.WithContext(ctx).First(&server, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

// GetAllServers retrieves all MCP servers.
func (r *MCPRepository) GetAllServers(ctx context.Context) ([]models.MCPServer, error) {
	var servers []models.MCPServer
	if err := r.db.WithContext(ctx).Preload("Tools").Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

// GetActiveServers retrieves all active MCP servers.
func (r *MCPRepository) GetActiveServers(ctx context.Context) ([]models.MCPServer, error) {
	var servers []models.MCPServer
	if err := r.db.WithContext(ctx).Preload("Tools").Where("is_active = ?", true).Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

// UpdateServer updates an MCP server.
func (r *MCPRepository) UpdateServer(ctx context.Context, server *models.MCPServer) error {
	return r.db.WithContext(ctx).Save(server).Error
}

// DeleteServer permanently removes an MCP server by ID.
func (r *MCPRepository) DeleteServer(ctx context.Context, id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete tools first
		if err := tx.WithContext(ctx).Unscoped().Delete(&models.MCPTool{}, "server_id = ?", id).Error; err != nil {
			return err
		}
		// Delete server
		return tx.WithContext(ctx).Unscoped().Delete(&models.MCPServer{}, "id = ?", id).Error
	})
}

// SyncTools synchronizes tools for an MCP server.
func (r *MCPRepository) SyncTools(ctx context.Context, serverID uuid.UUID, tools []models.MCPTool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Get existing tools to preserve IsActive status if possible
		var existingTools []models.MCPTool
		if err := tx.WithContext(ctx).Where("server_id = ?", serverID).Find(&existingTools).Error; err != nil {
			return err
		}

		activeMap := make(map[string]bool)
		for _, t := range existingTools {
			activeMap[t.Name] = t.IsActive
		}

		// Delete all existing tools for this server
		if err := tx.WithContext(ctx).Unscoped().Delete(&models.MCPTool{}, "server_id = ?", serverID).Error; err != nil {
			return err
		}

		// Insert new tools
		if len(tools) > 0 {
			for i := range tools {
				tools[i].ServerID = serverID
				if active, ok := activeMap[tools[i].Name]; ok {
					tools[i].IsActive = active
				}
			}
			if err := tx.WithContext(ctx).Create(&tools).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetToolsByServer retrieves all tools for an MCP server.
func (r *MCPRepository) GetToolsByServer(ctx context.Context, serverID uuid.UUID) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Find(&tools).Error; err != nil {
		return nil, err
	}
	return tools, nil
}

// GetActiveTools retrieves all active tools across all active servers.
func (r *MCPRepository) GetActiveTools(ctx context.Context) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	err := r.db.WithContext(ctx).
		Joins("JOIN mcp_servers ON mcp_tools.server_id = mcp_servers.id").
		Where("mcp_tools.is_active = ? AND mcp_servers.is_active = ? AND mcp_servers.status = ?", true, true, "connected").
		Find(&tools).Error
	if err != nil {
		return nil, err
	}
	return tools, nil
}

// GetToolByName retrieves a tool by server name and tool name.
func (r *MCPRepository) GetToolByName(ctx context.Context, serverName, toolName string) (*models.MCPTool, error) {
	var tool models.MCPTool
	err := r.db.WithContext(ctx).
		Joins("JOIN mcp_servers ON mcp_tools.server_id = mcp_servers.id").
		Where("mcp_servers.name = ? AND mcp_tools.name = ?", serverName, toolName).
		First(&tool).Error
	if err != nil {
		return nil, err
	}
	return &tool, nil
}
