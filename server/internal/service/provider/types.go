// Package provider provides LLM provider client implementations.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	// ErrNotImplemented is returned when a provider does not support an operation.
	ErrNotImplemented = errors.New("operation not implemented by this provider")
)

// ProviderError encapsulates an error from an upstream LLM provider, preserving HTTP details.
type ProviderError struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Message    string
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Message != "" && len(e.Body) > 0 {
		return e.Message + ": " + string(e.Body)
	}
	if len(e.Body) > 0 {
		return string(e.Body)
	}
	if e.Message != "" {
		return e.Message
	}
	return "unknown provider error"
}

// FlexibleContent handles the OpenAI-compatible content field which can be
// either a plain string or an array of content parts (multimodal format).
// After unmarshalling, Text contains the concatenated text content.
type FlexibleContent struct {
	Text string
	// Raw preserves the original JSON for transparent forwarding to upstream.
	Raw json.RawMessage
}

// ContentPart represents a single part in the array content format.
type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// UnmarshalJSON implements custom unmarshalling for flexible content.
func (fc *FlexibleContent) UnmarshalJSON(data []byte) error {
	fc.Raw = append(fc.Raw[:0], data...)

	// Handle null content (common in tool_call responses where content is null)
	if string(data) == "null" {
		return nil
	}

	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		fc.Text = s
		return nil
	}

	// Try array of content parts
	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		var texts []string
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		fc.Text = strings.Join(texts, "\n")
		return nil
	}

	// Fallback: treat as raw string
	fc.Text = string(data)
	return nil
}

// MarshalJSON outputs the original raw JSON to preserve the format for upstream.
func (fc FlexibleContent) MarshalJSON() ([]byte, error) {
	if len(fc.Raw) > 0 {
		return fc.Raw, nil
	}
	return json.Marshal(fc.Text)
}

// StringContent creates a FlexibleContent from a plain string.
func StringContent(s string) FlexibleContent {
	raw, _ := json.Marshal(s)
	return FlexibleContent{Text: s, Raw: raw}
}

// Client defines the interface for LLM provider clients.
type Client interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
	Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
	GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error)
	TranscribeAudio(ctx context.Context, req *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error)
	SynthesizeSpeech(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	CheckHealth(ctx context.Context) (bool, time.Duration, error)
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	ID      string        `json:"id,omitempty"`
	Model   string        `json:"model,omitempty"`
	Choices []DeltaChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
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
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls json.RawMessage `json:"tool_calls,omitempty"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model         string                 `json:"model"`
	Messages      []Message              `json:"messages"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`
	Temperature   float64                `json:"temperature,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	StreamOptions map[string]interface{} `json:"stream_options,omitempty"`
	Tools         json.RawMessage        `json:"tools,omitempty"`
	ToolChoice    json.RawMessage        `json:"tool_choice,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role       string          `json:"role"`
	Content    FlexibleContent `json:"content"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
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
	// Extra holds any additional fields from the upstream /v1/models response
	// (e.g., type, capabilities, input_modalities, output_modalities).
	// These are transparently forwarded to clients so they can detect
	// model features such as vision support.
	Extra map[string]json.RawMessage `json:"-"`
}

// UnmarshalJSON implements custom unmarshalling to capture extra upstream fields.
func (m *ModelInfo) UnmarshalJSON(data []byte) error {
	// Unmarshal known fields first
	type plain ModelInfo
	if err := json.Unmarshal(data, (*plain)(m)); err != nil {
		return err
	}
	// Capture ALL fields into a map
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	// Remove known fields, keep the rest as extras
	delete(all, "id")
	delete(all, "name")
	delete(all, "created")
	if len(all) > 0 {
		m.Extra = all
	}
	return nil
}

// EmbeddingRequest represents an embeddings request.
type EmbeddingRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"` // Can be string or []string
	EncodingFormat string      `json:"encoding_format,omitempty"`
}

// EmbeddingData represents a single embedding.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingResponse represents an embeddings response.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// ImageGenerationRequest represents a request to generate an image.
type ImageGenerationRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // "url" or "b64_json"
}

// ImageData represents generated image metadata.
type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageGenerationResponse represents a response containing generated images.
type ImageGenerationResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

// AudioTranscriptionRequest represents a request to transcribe audio.
type AudioTranscriptionRequest struct {
	File           []byte  `json:"-"`
	FileName       string  `json:"-"`
	Model          string  `json:"model"`
	Language       string  `json:"language,omitempty"`
	Prompt         string  `json:"prompt,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"` // "json", "text", "srt", "verbose_json", "vtt"
	Temperature    float64 `json:"temperature,omitempty"`
}

// AudioTranscriptionResponse represents a transcription response.
type AudioTranscriptionResponse struct {
	Text string `json:"text"`
}

// SpeechRequest represents a text-to-speech synthesis request (OpenAI-compatible).
type SpeechRequest struct {
	Model          string  `json:"model"`                       // e.g., "tts-1", "tts-1-hd", "cosyvoice-v2"
	Input          string  `json:"input"`                       // The text to synthesize
	Voice          string  `json:"voice"`                       // e.g., "alloy", "echo", "fable", "onyx", "nova", "shimmer"
	ResponseFormat string  `json:"response_format,omitempty"`   // "mp3", "opus", "aac", "flac", "wav", "pcm"
	Speed          float64 `json:"speed,omitempty"`             // 0.25 to 4.0, default 1.0
}

// SpeechResponse holds the synthesized audio binary data.
type SpeechResponse struct {
	Audio       []byte `json:"-"`  // Raw audio bytes
	ContentType string `json:"-"`  // MIME type, e.g., "audio/mpeg"
}
