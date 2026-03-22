package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
		Timeout: 600 * time.Second,
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
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "Anthropic API error",
		}
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
				Message:      Message{Role: "assistant", Content: StringContent(content)},
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

// Embeddings returns ErrNotImplemented as Anthropic doesn't natively support this endpoint format.
func (c *AnthropicClient) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, ErrNotImplemented
}

// GenerateImage returns ErrNotImplemented.
func (c *AnthropicClient) GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	return nil, ErrNotImplemented
}

// TranscribeAudio returns ErrNotImplemented.
func (c *AnthropicClient) TranscribeAudio(_ context.Context, _ *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error) {
	return nil, ErrNotImplemented
}

// SynthesizeSpeech returns ErrNotImplemented.
func (c *AnthropicClient) SynthesizeSpeech(_ context.Context, _ *SpeechRequest) (*SpeechResponse, error) {
	return nil, ErrNotImplemented
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
			{Role: "user", Content: StringContent("Hi")},
		},
		MaxTokens: 5,
	}

	_, err := c.Chat(ctx, req)
	latency := time.Since(start)

	return err == nil, latency, err
}

// StreamChat sends a real SSE streaming request to Anthropic Messages API.
func (c *AnthropicClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	anthropicReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"max_tokens": maxTokens,
		"stream":     true,
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

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "Anthropic API error",
		}
	}

	chunks := make(chan StreamChunk)
	go func() {
		defer close(chunks)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event struct {
				Type  string `json:"type"`
				Index int    `json:"index"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					chunks <- StreamChunk{
						Model: req.Model,
						Choices: []DeltaChoice{{
							Index: 0,
							Delta: Delta{Content: event.Delta.Text},
						}},
					}
				}
			case "message_delta":
				// Final usage info
				if event.Usage.OutputTokens > 0 {
					chunks <- StreamChunk{
						Usage: &Usage{
							CompletionTokens: event.Usage.OutputTokens,
						},
					}
				}
			case "message_stop":
				chunks <- StreamChunk{Done: true}
				return
			}
		}

		// If we exit the loop without message_stop, send done
		chunks <- StreamChunk{Done: true}
	}()

	return chunks, nil
}
