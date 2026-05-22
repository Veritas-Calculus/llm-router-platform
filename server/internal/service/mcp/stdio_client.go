package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

// allowedMCPCommands is an allowlist of executables permitted for MCP stdio transport.
// Commands not in this list are rejected to prevent arbitrary code execution (G204).
var allowedMCPCommands = map[string]bool{
	"npx":    true,
	"node":   true,
	"python": true, "python3": true,
	"uvx":    true, "uv": true,
	"docker": true,
	"deno":   true,
	"bun":    true,
}

// dangerousMCPArgFlags lists argument prefixes that turn an otherwise-trusted
// interpreter into an arbitrary-code-execution primitive. Validating only the
// executable name is insufficient — `node -e "…"`, `python -c "…"`, `deno eval`,
// etc. let any caller who can edit MCP server config (currently @auth(role:
// ADMIN), so platform admins) execute attacker-controlled code under the
// gateway service account.
//
// This block is defense-in-depth on top of admin gating: it raises the bar so
// a single compromised admin account cannot trivially read ENCRYPTION_KEY/
// JWT_SECRET/cloud IAM tokens from the process environment.
var dangerousMCPArgPrefixes = []string{
	"-c", "-e",
	"--eval", "--command", "--exec",
}

// dangerousDockerFlags rejects docker invocations that bind the host filesystem,
// give the container privileged access, or share the host network — any of
// which trivially defeats process isolation.
var dangerousDockerFlags = []string{
	"--privileged",
	"--cap-add",
	"--security-opt",
	"--device",
	"--pid", "--ipc", "--uts", "--userns",
	"--network=host",
	"--net=host",
	"-v", "--volume",
	"--mount",
}

// validateMCPArgs returns an error if any element of args is a known
// code-injection flag for the given executable.
func validateMCPArgs(cmd string, args []string) error {
	for _, raw := range args {
		a := strings.TrimSpace(raw)
		if a == "" {
			continue
		}
		// Bare "-" means read from stdin (`python -`, `node -`). Refuse.
		if a == "-" {
			return fmt.Errorf("MCP args may not contain bare stdin marker %q", a)
		}
		switch cmd {
		case "node", "deno", "bun", "python", "python3", "uv", "uvx":
			for _, bad := range dangerousMCPArgPrefixes {
				if a == bad || strings.HasPrefix(a, bad+"=") {
					return fmt.Errorf("MCP args may not contain inline-code flag %q for %s", a, cmd)
				}
			}
		case "docker":
			for _, bad := range dangerousDockerFlags {
				if a == bad || strings.HasPrefix(a, bad+"=") {
					return fmt.Errorf("MCP args may not contain isolation-breaking docker flag %q", a)
				}
			}
		}
	}
	return nil
}

func (c *StdioClient) Connect(ctx context.Context) error {
	// G204: Validate command against allowlist before execution
	if !allowedMCPCommands[c.server.Command] {
		return fmt.Errorf("MCP command %q is not in the allowed commands list; permitted: npx, node, python, python3, uvx, uv, docker, deno, bun", c.server.Command)
	}

	var args []string
	if len(c.server.Args) > 0 {
		if err := json.Unmarshal(c.server.Args, &args); err != nil {
			return fmt.Errorf("failed to parse MCP server args: %w", err)
		}
	}

	if err := validateMCPArgs(c.server.Command, args); err != nil {
		return err
	}

	c.cmd = exec.Command(c.server.Command, args...) // #nosec G204 -- command + args validated against allowlist above
	
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

	var result json.RawMessage
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

	var result json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return result, nil
}
