package resolvers

// This file contains mcp domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"encoding/json"
	"fmt"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
)

// CreateMcpServer is the resolver for the createMcpServer field.
func (r *mutationResolver) CreateMcpServer(ctx context.Context, input model.McpServerInput) (*model.McpServer, error) {
	server := &models.MCPServer{Name: input.Name, Type: input.Type, IsActive: true}
	if input.Command != nil {
		server.Command = *input.Command
	}
	if input.URL != nil {
		server.URL = *input.URL
	}
	if input.IsActive != nil {
		server.IsActive = *input.IsActive
	}
	if input.Args != nil {
		b, _ := json.Marshal(input.Args)
		server.Args = b
	}
	if input.Env != nil {
		envMap := make(map[string]string)
		for _, e := range input.Env {
			envMap[e.Key] = e.Value
		}
		b, _ := json.Marshal(envMap)
		server.Env = b
	}
	if err := r.MCP.CreateServer(ctx, server); err != nil {
		return nil, err
	}
	return mcpServerToGQL(server), nil
}

// UpdateMcpServer is the resolver for the updateMcpServer field.
func (r *mutationResolver) UpdateMcpServer(ctx context.Context, id string, input model.McpServerInput) (*model.McpServer, error) {
	sid, _ := uuid.Parse(id)
	server, err := r.MCP.GetServerByID(ctx, sid)
	if err != nil {
		return nil, fmt.Errorf("server not found")
	}
	server.Name = input.Name
	server.Type = input.Type
	if input.Command != nil {
		server.Command = *input.Command
	}
	if input.URL != nil {
		server.URL = *input.URL
	}
	if input.IsActive != nil {
		server.IsActive = *input.IsActive
	}
	if input.Args != nil {
		b, _ := json.Marshal(input.Args)
		server.Args = b
	}
	if err := r.MCP.UpdateServer(ctx, server); err != nil {
		return nil, err
	}
	return mcpServerToGQL(server), nil
}

// DeleteMcpServer is the resolver for the deleteMcpServer field.
func (r *mutationResolver) DeleteMcpServer(ctx context.Context, id string) (bool, error) {
	sid, _ := uuid.Parse(id)
	return true, r.MCP.DeleteServer(ctx, sid)
}

// RefreshMcpTools is the resolver for the refreshMcpTools field.
func (r *mutationResolver) RefreshMcpTools(ctx context.Context, id string) (*model.McpServer, error) {
	sid, _ := uuid.Parse(id)
	if err := r.MCP.RefreshTools(ctx, sid); err != nil {
		return nil, err
	}
	server, err := r.MCP.GetServerByID(ctx, sid)
	if err != nil {
		return nil, err
	}
	return mcpServerToGQL(server), nil
}

// McpServers is the resolver for the mcpServers field.
func (r *queryResolver) McpServers(ctx context.Context) ([]*model.McpServer, error) {
	servers, err := r.MCP.GetAllServers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.McpServer, len(servers))
	for i := range servers {
		out[i] = mcpServerToGQL(&servers[i])
	}
	return out, nil
}

// McpServer is the resolver for the mcpServer field.
func (r *queryResolver) McpServer(ctx context.Context, id string) (*model.McpServer, error) {
	sid, _ := uuid.Parse(id)
	server, err := r.MCP.GetServerByID(ctx, sid)
	if err != nil {
		return nil, fmt.Errorf("server not found")
	}
	return mcpServerToGQL(server), nil
}

// McpTools is the resolver for the mcpTools field.
func (r *queryResolver) McpTools(ctx context.Context) ([]*model.McpTool, error) {
	tools, err := r.MCP.GetActiveTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.McpTool, len(tools))
	for i := range tools {
		out[i] = mcpToolToGQL(&tools[i])
	}
	return out, nil
}

// McpResources is the resolver for the mcpResources field.
func (r *queryResolver) McpResources(ctx context.Context) ([]*model.McpResource, error) {
	return []*model.McpResource{}, nil
}
