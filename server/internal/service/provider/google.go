package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// GoogleClient implements the Client interface for Google Gemini API.
type GoogleClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewGoogleClient creates a new Google Gemini client.
func NewGoogleClient(cfg *config.ProviderConfig, logger *zap.Logger) *GoogleClient {
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	if cfg.HTTPClient != nil {
		httpClient = cfg.HTTPClient()
	}
	return &GoogleClient{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// geminiRequest represents a Google Gemini API request.
type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

// geminiContent represents content in a Gemini request.
type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart represents a part of content.
type geminiPart struct {
	Text string `json:"text"`
}

// geminiGenerationConfig represents generation configuration.
type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

// geminiResponse represents a Google Gemini API response.
type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// geminiCandidate represents a response candidate.
type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

// geminiUsageMetadata represents token usage.
type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// convertRoleToGemini converts OpenAI-style role to Gemini role.
func convertRoleToGemini(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "system":
		return "user" // Gemini doesn't have system role, prepend to first user message
	default:
		return role
	}
}

// buildGeminiContents converts ChatRequest messages to Gemini format.
func buildGeminiContents(messages []Message) []geminiContent {
	var contents []geminiContent
	var systemPrompt string

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}

		content := geminiContent{
			Role: convertRoleToGemini(msg.Role),
			Parts: []geminiPart{
				{Text: msg.Content},
			},
		}

		// Prepend system prompt to first user message
		if msg.Role == "user" && systemPrompt != "" {
			content.Parts[0].Text = systemPrompt + "\n\n" + msg.Content
			systemPrompt = ""
		}

		contents = append(contents, content)
	}

	return contents
}

// Chat sends a chat completion request to Google Gemini.
func (c *GoogleClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	contents := buildGeminiContents(req.Messages)

	geminiReq := geminiRequest{
		Contents: contents,
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		geminiReq.GenerationConfig = &geminiGenerationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}

	// Gemini API endpoint: /v1beta/models/{model}:generateContent
	endpoint := c.baseURL + "/v1beta/models/" + req.Model + ":generateContent?key=" + c.apiKey

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	// Convert Gemini response to standard format
	content := ""
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		content = geminiResp.Candidates[0].Content.Parts[0].Text
	}

	usage := Usage{}
	if geminiResp.UsageMetadata != nil {
		usage.PromptTokens = geminiResp.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount
	}

	finishReason := "stop"
	if len(geminiResp.Candidates) > 0 {
		finishReason = strings.ToLower(geminiResp.Candidates[0].FinishReason)
	}

	return &ChatResponse{
		ID:    "gemini-" + req.Model,
		Model: req.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: content},
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}, nil
}

// StreamChat sends a streaming chat completion request to Google Gemini.
func (c *GoogleClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	contents := buildGeminiContents(req.Messages)

	geminiReq := geminiRequest{
		Contents: contents,
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		geminiReq.GenerationConfig = &geminiGenerationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}

	// Gemini streaming endpoint: /v1beta/models/{model}:streamGenerateContent
	endpoint := c.baseURL + "/v1beta/models/" + req.Model + ":streamGenerateContent?key=" + c.apiKey + "&alt=sse"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var geminiResp geminiResponse
			if err := json.Unmarshal([]byte(data), &geminiResp); err != nil {
				continue
			}

			if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
				chunks <- StreamChunk{
					Model: req.Model,
					Choices: []DeltaChoice{{
						Index: 0,
						Delta: Delta{Content: geminiResp.Candidates[0].Content.Parts[0].Text},
					}},
				}
			}
		}

		chunks <- StreamChunk{Done: true}
	}()

	return chunks, nil
}

// ListModels returns available models from Google Gemini.
func (c *GoogleClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	endpoint := c.baseURL + "/v1beta/models?key=" + c.apiKey

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to list models")
	}

	var result struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		// Extract model ID from "models/gemini-pro" format
		modelID := strings.TrimPrefix(m.Name, "models/")
		models = append(models, ModelInfo{
			ID:   modelID,
			Name: m.DisplayName,
		})
	}

	return models, nil
}

// CheckHealth verifies the Google Gemini API is accessible.
func (c *GoogleClient) CheckHealth(ctx context.Context) (bool, time.Duration, error) {
	start := time.Now()

	endpoint := c.baseURL + "/v1beta/models?key=" + c.apiKey

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, 0, err
	}

	resp, err := c.httpClient.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		return false, latency, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK, latency, nil
}
