package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"llm-router-platform/internal/models"

	"go.uber.org/zap"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// StdioClient implements the MCP Client interface for stdio transport.
type StdioClient struct {
	server models.MCPServer
	logger *zap.Logger
	
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	
	pending   map[int64]chan *JSONRPCResponse
	pendingMu sync.Mutex
	nextID    int64
	
	initialized bool
}

// NewStdioClient creates a new stdio-based MCP client.
func NewStdioClient(server models.MCPServer, logger *zap.Logger) (*StdioClient, error) {
	return &StdioClient{
		server:  server,
		logger:  logger,
		pending: make(map[int64]chan *JSONRPCResponse),
		nextID:  1,
	}, nil
}

func (c *StdioClient) Connect(ctx context.Context) error {
	var args []string
	if len(c.server.Args) > 0 {
		if err := json.Unmarshal(c.server.Args, &args); err != nil {
			return fmt.Errorf("failed to parse MCP server args: %w", err)
		}
	}

	c.cmd = exec.Command(c.server.Command, args...)
	
	// Set environment variables
	if len(c.server.Env) > 0 {
		var envMap map[string]string
		if err := json.Unmarshal(c.server.Env, &envMap); err == nil {
			env := os.Environ()
			for k, v := range envMap {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
			c.cmd.Env = env
		}
	}

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	
	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := c.cmd.Start(); err != nil {
		return err
	}

	go c.listen()

	// MCP Initialize handshake
	if err := c.initialize(ctx); err != nil {
		_ = c.Close()
		return err
	}

	return nil
}

func (c *StdioClient) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

func (c *StdioClient) listen() {
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			c.logger.Error("failed to unmarshal JSON-RPC response", zap.Error(err), zap.String("line", string(line)))
			continue
		}

		if resp.ID != nil {
			var id int64
			switch v := resp.ID.(type) {
			case float64:
				id = int64(v)
			case int64:
				id = v
			default:
				continue
			}

			c.pendingMu.Lock()
			ch, ok := c.pending[id]
			if ok {
				delete(c.pending, id)
				ch <- &resp
			}
			c.pendingMu.Unlock()
		}
	}
}

func (c *StdioClient) sendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	c.pendingMu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan *JSONRPCResponse, 1)
	c.pending[id] = ch
	c.pendingMu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	_, err = c.stdin.Write(append(data, '\n'))
	if err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response from MCP server")
	}
}

func (c *StdioClient) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": false,
			},
			"sampling": map[string]interface{}{},
		},
		"clientInfo": map[string]interface{}{
			"name":    "llm-router-platform",
			"version": "1.0.0",
		},
	}

	_, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	// Send initialized notification (no response expected in JSON-RPC if it's a notification, but MCP might want a regular message)
	// Actually initialized is a notification
	notify := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notify)
	_, _ = c.stdin.Write(append(data, '\n'))

	c.initialized = true
	return nil
}

func (c *StdioClient) ListTools(ctx context.Context) ([]models.MCPTool, error) {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	tools := make([]models.MCPTool, len(result.Tools))
	for i, t := range result.Tools {
		tools[i] = models.MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			IsActive:    true,
		}
	}

	return tools, nil
}

func (c *StdioClient) CallTool(ctx context.Context, name string, arguments interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *StdioClient) ListResources(ctx context.Context) ([]Resource, error) {
	resp, err := c.sendRequest(ctx, "resources/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Resources []Resource `json:"resources"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return result.Resources, nil
}

func (c *StdioClient) ReadResource(ctx context.Context, uri string) (interface{}, error) {
	params := map[string]interface{}{
		"uri": uri,
	}

	resp, err := c.sendRequest(ctx, "resources/read", params)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return result, nil
}
