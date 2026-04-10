package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles MCP server lifecycle and tool management.
type Service struct {
	repo       repository.MCPRepo
	clients    map[string]Client // name -> MCP client
	mu         sync.RWMutex
	logger     *zap.Logger
	ctx        context.Context    // lifecycle context for background goroutines
	cancel     context.CancelFunc // cancels all background goroutines on shutdown
	allowLocal bool               // whether MCP SSE clients may dial private IPs
}

// Client defines the interface for an MCP client (stdio or sse).
type Client interface {
	Connect(ctx context.Context) error
	Close() error
	ListTools(ctx context.Context) ([]models.MCPTool, error)
	CallTool(ctx context.Context, name string, arguments interface{}) (interface{}, error)
	ListResources(ctx context.Context) ([]Resource, error)
	ReadResource(ctx context.Context, uri string) (interface{}, error)
}

// Resource represents an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// NewService creates a new MCP service. allowLocal mirrors the server
// ALLOW_LOCAL_PROVIDERS flag and gates SSRF-sensitive MCP SSE connections.
func NewService(repo repository.MCPRepo, logger *zap.Logger, allowLocal bool) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		repo:       repo,
		clients:    make(map[string]Client),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		allowLocal: allowLocal,
	}
}

// Shutdown cancels all background MCP connections and releases resources.
func (s *Service) Shutdown() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, client := range s.clients {
		if err := client.Close(); err != nil {
			s.logger.Error("error closing MCP client", zap.String("name", name), zap.Error(err))
		}
		delete(s.clients, name)
	}
}

// Initialize connects to all active MCP servers.
func (s *Service) Initialize(ctx context.Context) error {
	servers, err := s.repo.GetActiveServers(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		go s.connectServer(s.ctx, server) // G118: uses lifecycle context, not context.Background()
	}

	return nil
}

// ─── CRUD Operations ────────────────────────────────────────────────────

func (s *Service) CreateServer(ctx context.Context, server *models.MCPServer) error {
	if err := s.repo.CreateServer(ctx, server); err != nil {
		return err
	}
	if server.IsActive {
		go s.connectServer(s.ctx, *server) // G118: lifecycle-scoped
	}
	return nil
}

func (s *Service) GetServerByID(ctx context.Context, id uuid.UUID) (*models.MCPServer, error) {
	return s.repo.GetServerByID(ctx, id)
}

func (s *Service) GetAllServers(ctx context.Context) ([]models.MCPServer, error) {
	return s.repo.GetAllServers(ctx)
}

func (s *Service) UpdateServer(ctx context.Context, server *models.MCPServer) error {
	if err := s.repo.UpdateServer(ctx, server); err != nil {
		return err
	}
	
	// Reconnect if status changed or active status changed
	if server.IsActive {
		go s.connectServer(s.ctx, *server) // G118: lifecycle-scoped
	} else {
		s.mu.Lock()
		if client, ok := s.clients[server.Name]; ok {
			_ = client.Close()
			delete(s.clients, server.Name)
		}
		s.mu.Unlock()
	}
	return nil
}

func (s *Service) DeleteServer(ctx context.Context, id uuid.UUID) error {
	server, err := s.repo.GetServerByID(ctx, id)
	if err == nil {
		s.mu.Lock()
		if client, ok := s.clients[server.Name]; ok {
			_ = client.Close()
			delete(s.clients, server.Name)
		}
		s.mu.Unlock()
	}
	return s.repo.DeleteServer(ctx, id)
}

func (s *Service) RefreshTools(ctx context.Context, id uuid.UUID) error {
	server, err := s.repo.GetServerByID(ctx, id)
	if err != nil {
		return err
	}
	
	s.mu.RLock()
	client, ok := s.clients[server.Name]
	s.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("server not connected")
	}
	
	tools, err := client.ListTools(ctx)
	if err != nil {
		return err
	}
	
	return s.repo.SyncTools(ctx, server.ID, tools)
}

// GetActiveTools returns all active tools from connected MCP servers.
func (s *Service) GetActiveTools(ctx context.Context) ([]models.MCPTool, error) {
	return s.repo.GetActiveTools(ctx)
}

// GetToolsForLLM returns active tools formatted for LLM provider consumption (OpenAI format).
func (s *Service) GetToolsForLLM(ctx context.Context) ([]interface{}, error) {
	dbTools, err := s.repo.GetActiveTools(ctx)
	if err != nil {
		return nil, err
	}

	llmTools := make([]interface{}, len(dbTools))
	for i, t := range dbTools {
		// Construct OpenAI tool format: { type: "function", function: { name: "...", description: "...", parameters: {...} } }
		// Tool name should be prefixed with server name to ensure uniqueness
		server, err := s.repo.GetServerByID(ctx, t.ServerID)
		fullName := t.Name
		if err == nil {
			fullName = server.Name + "__" + t.Name
		}

		llmTools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        fullName,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		}
	}

	return llmTools, nil
}

// CallTool executes a tool on the specified MCP server.
func (s *Service) CallTool(ctx context.Context, serverName, toolName string, arguments interface{}) (interface{}, error) {
	s.mu.RLock()
	client, ok := s.clients[serverName]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server %q not connected", serverName)
	}

	return client.CallTool(ctx, toolName, arguments)
}

// GetActiveResources returns all active resources from connected MCP servers.
func (s *Service) GetActiveResources(ctx context.Context) (map[string][]Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make(map[string][]Resource)
	for name, client := range s.clients {
		res, err := client.ListResources(ctx)
		if err == nil {
			resources[name] = res
		}
	}

	return resources, nil
}

// ReadResource reads a resource from the specified MCP server.
func (s *Service) ReadResource(ctx context.Context, serverName, uri string) (interface{}, error) {
	s.mu.RLock()
	client, ok := s.clients[serverName]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server %q not connected", serverName)
	}

	return client.ReadResource(ctx, uri)
}

func (s *Service) connectServer(ctx context.Context, server models.MCPServer) {
	s.logger.Info("connecting to MCP server", zap.String("name", sanitize.LogValue(server.Name)), zap.String("type", sanitize.LogValue(server.Type)))
	
	var client Client
	var err error
	
	switch server.Type {
	case "stdio":
		client, err = NewStdioClient(server, s.logger)
	case "sse":
		client, err = NewSSEClient(server, s.logger, s.allowLocal)
	default:
		s.logger.Error("unsupported MCP transport type", zap.String("type", sanitize.LogValue(server.Type)))
		return
	}

	if err != nil {
		s.updateServerStatus(server.ID, "error", err.Error())
		return
	}

	if err := client.Connect(ctx); err != nil {
		s.updateServerStatus(server.ID, "error", err.Error())
		return
	}

	s.mu.Lock()
	s.clients[server.Name] = client
	s.mu.Unlock()

	// Sync tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		s.logger.Error("failed to list tools for MCP server", zap.String("name", sanitize.LogValue(server.Name)), zap.Error(err))
	} else {
		if err := s.repo.SyncTools(ctx, server.ID, tools); err != nil {
			s.logger.Error("failed to sync tools for MCP server", zap.String("name", sanitize.LogValue(server.Name)), zap.Error(err))
		}
	}

	s.updateServerStatus(server.ID, "connected", "")
}

func (s *Service) updateServerStatus(id uuid.UUID, status, lastError string) {
	ctx := context.Background()
	server, err := s.repo.GetServerByID(ctx, id)
	if err != nil {
		return
	}
	server.Status = status
	server.LastError = lastError
	server.LastCheckedAt = time.Now()
	_ = s.repo.UpdateServer(ctx, server)
}
