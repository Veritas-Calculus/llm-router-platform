package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// OpenAIClient implements the Client interface for OpenAI.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(cfg *config.ProviderConfig, logger *zap.Logger) *OpenAIClient {
	httpClient := &http.Client{
		Timeout: 600 * time.Second,
	}
	if cfg.HTTPClient != nil {
		httpClient = cfg.HTTPClient()
	}
	return &OpenAIClient{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// Chat sends a chat completion request to OpenAI.
func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

// Embeddings sends an embeddings request to OpenAI.
func (c *OpenAIClient) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, err
	}

	return &embResp, nil
}

// StreamChat sends a streaming chat completion request to OpenAI.
func (c *OpenAIClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	req.Stream = true
	if req.StreamOptions == nil {
		req.StreamOptions = make(map[string]interface{})
	}
	req.StreamOptions["include_usage"] = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, errors.New(string(respBody))
	}

	chunks := make(chan StreamChunk)
	go func() {
		defer close(chunks)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		// Increase buffer size to handle large streaming responses
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				chunks <- StreamChunk{Done: true}
				return
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			chunks <- chunk
		}
	}()

	return chunks, nil
}

// ListModels returns available models from OpenAI.
func (c *OpenAIClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to list models")
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// CheckHealth verifies the OpenAI API is accessible.
func (c *OpenAIClient) CheckHealth(ctx context.Context) (bool, time.Duration, error) {
	start := time.Now()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return false, 0, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		return false, latency, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK, latency, nil
}

// GenerateImage sends an image generation request to OpenAI.
func (c *OpenAIClient) GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	var imgResp ImageGenerationResponse
	if err := json.NewDecoder(resp.Body).Decode(&imgResp); err != nil {
		return nil, err
	}

	return &imgResp, nil
}

// TranscribeAudio sends an audio transcription request to OpenAI.
func (c *OpenAIClient) TranscribeAudio(ctx context.Context, req *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create a form file field
	fw, err := w.CreateFormFile("file", req.FileName)
	if err != nil {
		return nil, err
	}
	if _, err = fw.Write(req.File); err != nil {
		return nil, err
	}

	// Add other fields
	_ = w.WriteField("model", req.Model)
	if req.Language != "" {
		_ = w.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = w.WriteField("prompt", req.Prompt)
	}
	if req.ResponseFormat != "" {
		_ = w.WriteField("response_format", req.ResponseFormat)
	}
	if req.Temperature > 0 {
		_ = w.WriteField("temperature", fmt.Sprintf("%f", req.Temperature))
	}

	// Close the multipart writer to finalize the payload
	if err := w.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &b)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	// For specific response formats, OpenAI returns raw text or JSON
	// Assume JSON or plain text handling based on typical behavior
	if req.ResponseFormat == "text" || req.ResponseFormat == "srt" || req.ResponseFormat == "vtt" {
		respBody, _ := io.ReadAll(resp.Body)
		return &AudioTranscriptionResponse{Text: string(respBody)}, nil
	}

	var audioResp AudioTranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&audioResp); err != nil {
		return nil, err
	}

	return &audioResp, nil
}
