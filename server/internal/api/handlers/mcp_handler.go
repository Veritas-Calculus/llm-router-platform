package handlers

import (
	"net/http"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/mcp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MCPHandler handles MCP management endpoints.
type MCPHandler struct {
	mcpService *mcp.Service
	logger     *zap.Logger
}

// NewMCPHandler creates a new MCP handler.
func NewMCPHandler(ms *mcp.Service, logger *zap.Logger) *MCPHandler {
	return &MCPHandler{
		mcpService: ms,
		logger:     logger,
	}
}

// ListServers returns all MCP servers.
func (h *MCPHandler) ListServers(c *gin.Context) {
	servers, err := h.mcpService.GetAllServers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": servers})
}

// CreateServer creates a new MCP server.
func (h *MCPHandler) CreateServer(c *gin.Context) {
	var server models.MCPServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.mcpService.CreateServer(c.Request.Context(), &server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, server)
}

// GetServer returns an MCP server by ID.
func (h *MCPHandler) GetServer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}

	server, err := h.mcpService.GetServerByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	c.JSON(http.StatusOK, server)
}

// UpdateServer updates an MCP server.
func (h *MCPHandler) UpdateServer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}

	var server models.MCPServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server.ID = id

	if err := h.mcpService.UpdateServer(c.Request.Context(), &server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, server)
}

// DeleteServer deletes an MCP server.
func (h *MCPHandler) DeleteServer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}

	if err := h.mcpService.DeleteServer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "server deleted"})
}

// RefreshTools refreshes tools from an MCP server.
func (h *MCPHandler) RefreshTools(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}

	if err := h.mcpService.RefreshTools(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tools refreshed"})
}

// ListTools returns all active tools across all servers.
func (h *MCPHandler) ListTools(c *gin.Context) {
	tools, err := h.mcpService.GetActiveTools(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tools})
}

// ListResources returns all active resources across all servers.
func (h *MCPHandler) ListResources(c *gin.Context) {
	resources, err := h.mcpService.GetActiveResources(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resources})
}

// ReadResource reads a resource from a specific server.
func (h *MCPHandler) ReadResource(c *gin.Context) {
	serverName := c.Query("server")
	uri := c.Query("uri")

	if serverName == "" || uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server and uri query parameters are required"})
		return
	}

	result, err := h.mcpService.ReadResource(c.Request.Context(), serverName, uri)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
