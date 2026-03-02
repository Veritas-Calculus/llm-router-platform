package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// AnthropicClient implements the Client interface for Anthropic.
type AnthropicClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(cfg *config.ProviderConfig, logger *zap.Logger) *AnthropicClient {
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	if cfg.HTTPClient != nil {
		httpClient = cfg.HTTPClient()
	}
	return &AnthropicClient{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// Chat sends a chat completion request to Anthropic.
func (c *AnthropicClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	anthropicReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"max_tokens": req.MaxTokens,
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	var anthropicResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, err
	}

	content := ""
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &ChatResponse{
		ID:    anthropicResp.ID,
		Model: anthropicResp.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// ListModels returns available models from Anthropic.
func (c *AnthropicClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus"},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku"},
	}, nil
}

// CheckHealth verifies the Anthropic API is accessible.
func (c *AnthropicClient) CheckHealth(ctx context.Context) (bool, time.Duration, error) {
	start := time.Now()

	req := &ChatRequest{
		Model: "claude-3-haiku-20240307",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 5,
	}

	_, err := c.Chat(ctx, req)
	latency := time.Since(start)

	return err == nil, latency, err
}

// StreamChat sends a streaming chat completion request to Anthropic.
func (c *AnthropicClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	// Anthropic streaming not yet implemented, fallback to non-streaming
	chunks := make(chan StreamChunk)
	go func() {
		defer close(chunks)
		resp, err := c.Chat(ctx, req)
		if err != nil {
			chunks <- StreamChunk{Error: err}
			return
		}
		if len(resp.Choices) > 0 {
			chunks <- StreamChunk{
				ID:    resp.ID,
				Model: resp.Model,
				Choices: []DeltaChoice{{
					Index: 0,
					Delta: Delta{Content: resp.Choices[0].Message.Content},
				}},
			}
		}
		chunks <- StreamChunk{Done: true}
	}()
	return chunks, nil
}
