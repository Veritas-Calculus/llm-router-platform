package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"go.uber.org/zap"
)

// SSEClient implements the MCP Client interface for SSE transport.
type SSEClient struct {
	server models.MCPServer
	logger *zap.Logger

	httpClient *http.Client
	postURL    string // The URL to send POST requests to (received via SSE)
	
	pending   map[int64]chan *JSONRPCResponse
	pendingMu sync.Mutex
	nextID    int64
	
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSSEClient creates a new SSE-based MCP client. allowLocal gates SSRF.
func NewSSEClient(server models.MCPServer, logger *zap.Logger, allowLocal bool) (*SSEClient, error) {
	return &SSEClient{
		server:     server,
		logger:     logger,
		pending:    make(map[int64]chan *JSONRPCResponse),
		nextID:     1,
		httpClient: sanitize.SafeHTTPClient(allowLocal, 60*time.Second),
	}, nil
}

func (c *SSEClient) Connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.server.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use the safe client (not http.DefaultClient) so the SSE upgrade request
	// also goes through the SSRF dial guard.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return fmt.Errorf("failed to connect to SSE: status %d", resp.StatusCode)
	}

	// We need to find the endpoint URL from the SSE stream before we can send requests
	endpointFound := make(chan bool, 1)
	
	c.wg.Add(1)
	go c.listen(resp.Body, endpointFound)

	// Wait for endpoint event
	select {
	case <-endpointFound:
		// Success
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for MCP SSE endpoint event")
	}

	// Initialize MCP
	return c.initialize(ctx)
}

func (c *SSEClient) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *SSEClient) listen(body io.ReadCloser, endpointFound chan bool) {
	defer func() { _ = body.Close() }()
	defer c.wg.Done()

	scanner := bufio.NewScanner(body)
	var eventType string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			switch eventType {
			case "endpoint":
				// This event provides the URL for POST requests
				c.postURL = data
				if !strings.HasPrefix(c.postURL, "http") {
					// Handle relative URLs if necessary
					// For now assume absolute or relative to base
					if strings.HasPrefix(c.postURL, "/") {
						// Extract base from server URL
						c.postURL = c.server.URL + c.postURL
					}
				}
				select {
				case endpointFound <- true:
				default:
				}
			case "message":
				var resp JSONRPCResponse
				if err := json.Unmarshal([]byte(data), &resp); err != nil {
					c.logger.Error("failed to unmarshal SSE JSON-RPC message", zap.Error(err))
					continue
				}
				c.handleResponse(&resp)
			}
			eventType = "" // Reset for next event
		}
	}
}

func (c *SSEClient) handleResponse(resp *JSONRPCResponse) {
	if resp.ID != nil {
		var id int64
		switch v := resp.ID.(type) {
		case float64:
			id = int64(v)
		case int64:
			id = v
		default:
			return
		}

		c.pendingMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
			ch <- resp
		}
		c.pendingMu.Unlock()
	}
}

func (c *SSEClient) sendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.postURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST request failed: status %d", resp.StatusCode)
	}

	select {
	case r := <-ch:
		if r.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error: %s (code: %d)", r.Error.Message, r.Error.Code)
		}
		return r, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response from MCP SSE server")
	}
}

func (c *SSEClient) initialize(ctx context.Context) error {
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

	// Send initialized notification
	notify := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notify)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.postURL, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	_, _ = c.httpClient.Do(httpReq)

	return nil
}

func (c *SSEClient) ListTools(ctx context.Context) ([]models.MCPTool, error) {
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

func (c *SSEClient) CallTool(ctx context.Context, name string, arguments interface{}) (interface{}, error) {
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

func (c *SSEClient) ListResources(ctx context.Context) ([]Resource, error) {
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

func (c *SSEClient) ReadResource(ctx context.Context, uri string) (interface{}, error) {
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
