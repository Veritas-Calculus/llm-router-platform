// Package provider provides LLM provider client implementations.
package provider

import (
	"context"
	"time"
)

// Client defines the interface for LLM provider clients.
type Client interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	CheckHealth(ctx context.Context) (bool, time.Duration, error)
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	ID      string        `json:"id,omitempty"`
	Model   string        `json:"model,omitempty"`
	Choices []DeltaChoice `json:"choices,omitempty"`
	Error   error         `json:"-"`
	Done    bool          `json:"-"`
}

// DeltaChoice represents a streaming choice with delta content.
type DeltaChoice struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// Delta represents the delta content in a streaming chunk.
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelInfo represents model information.
type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
}
