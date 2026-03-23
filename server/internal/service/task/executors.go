package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"

	"go.uber.org/zap"
)

// ─── Executor Implementations ──────────────────────────────────────
// These executors use the router to call actual LLM providers.

// BatchTTSInput represents the input for batch TTS tasks.
type BatchTTSInput struct {
	Items []BatchTTSItem `json:"items"`
}

// BatchTTSItem is a single text-to-speech item.
type BatchTTSItem struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	Model string `json:"model,omitempty"`
}

// BatchTTSExecutor processes batch text-to-speech tasks.
type BatchTTSExecutor struct {
	router *router.Router
	logger *zap.Logger
}

// NewBatchTTSExecutor creates a new batch TTS executor.
func NewBatchTTSExecutor(r *router.Router, logger *zap.Logger) *BatchTTSExecutor {
	return &BatchTTSExecutor{router: r, logger: logger.Named("batch-tts")}
}

// Execute processes a batch TTS task.
func (e *BatchTTSExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input BatchTTSInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if len(input.Items) == 0 {
		return "", fmt.Errorf("no items to process")
	}

	results := make([]map[string]interface{}, 0, len(input.Items))
	for i, item := range input.Items {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		model := item.Model
		if model == "" {
			model = "tts-1"
		}
		voice := item.Voice
		if voice == "" {
			voice = "alloy"
		}

		e.logger.Debug("processing TTS item",
			zap.Int("index", i),
			zap.String("text_preview", truncate(item.Text, 50)),
		)

		selectedProvider, apiKey, err := e.router.Route(ctx, model)
		if err != nil {
			results = append(results, map[string]interface{}{
				"index": i, "status": "error", "error": fmt.Sprintf("no provider for model %s: %s", model, err.Error()),
			})
		} else {
			req := &provider.SpeechRequest{
				Model: model,
				Input: item.Text,
				Voice: voice,
			}
			resp, speechErr := e.router.ExecuteSpeech(ctx, selectedProvider, apiKey, req, 2)
			if speechErr != nil {
				results = append(results, map[string]interface{}{
					"index": i, "status": "error", "error": speechErr.Error(),
				})
			} else {
				results = append(results, map[string]interface{}{
					"index": i, "status": "ok", "size_bytes": len(resp.Response.Audio),
				})
			}
		}

		pct := int(float64(i+1) / float64(len(input.Items)) * 100)
		progressFn(pct)
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"processed": len(results),
		"items":     results,
	})
	return string(resultJSON), nil
}

// ─── Batch Image Executor ──────────────────────────────────────────

// BatchImageInput represents the input for batch image generation tasks.
type BatchImageInput struct {
	Prompts []string `json:"prompts"`
	Model   string   `json:"model,omitempty"`
	Size    string   `json:"size,omitempty"`
}

// BatchImageExecutor processes batch image generation tasks.
type BatchImageExecutor struct {
	router *router.Router
	logger *zap.Logger
}

// NewBatchImageExecutor creates a new batch image executor.
func NewBatchImageExecutor(r *router.Router, logger *zap.Logger) *BatchImageExecutor {
	return &BatchImageExecutor{router: r, logger: logger.Named("batch-image")}
}

// Execute processes a batch image generation task.
func (e *BatchImageExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input BatchImageInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if len(input.Prompts) == 0 {
		return "", fmt.Errorf("no prompts to process")
	}

	model := input.Model
	if model == "" {
		model = "dall-e-3"
	}
	size := input.Size
	if size == "" {
		size = "1024x1024"
	}

	results := make([]map[string]interface{}, 0, len(input.Prompts))
	for i, prompt := range input.Prompts {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		e.logger.Debug("processing image prompt",
			zap.Int("index", i),
			zap.String("prompt_preview", truncate(prompt, 50)),
		)

		selectedProvider, apiKey, err := e.router.Route(ctx, model)
		if err != nil {
			results = append(results, map[string]interface{}{
				"index": i, "status": "error", "error": fmt.Sprintf("no provider for model %s: %s", model, err.Error()),
			})
		} else {
			imgReq := &provider.ImageGenerationRequest{
				Model:  model,
				Prompt: prompt,
				N:      1,
				Size:   size,
			}
			resp, imgErr := e.router.ExecuteImage(ctx, selectedProvider, apiKey, imgReq, 2)
			if imgErr != nil {
				results = append(results, map[string]interface{}{
					"index": i, "status": "error", "error": imgErr.Error(),
				})
			} else {
				urls := make([]string, 0)
				for _, d := range resp.Response.Data {
					if d.URL != "" {
						urls = append(urls, d.URL)
					}
				}
				results = append(results, map[string]interface{}{
					"index": i, "status": "ok", "urls": urls,
				})
			}
		}

		pct := int(float64(i+1) / float64(len(input.Prompts)) * 100)
		progressFn(pct)
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"processed": len(results),
		"images":    results,
	})
	return string(resultJSON), nil
}

// ─── Video Analysis Executor ───────────────────────────────────────

// VideoAnalysisInput represents the input for video analysis tasks.
type VideoAnalysisInput struct {
	VideoURL string `json:"video_url"`
	Model    string `json:"model,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// VideoAnalysisExecutor processes video analysis tasks using multimodal chat.
type VideoAnalysisExecutor struct {
	router *router.Router
	logger *zap.Logger
}

// NewVideoAnalysisExecutor creates a new video analysis executor.
func NewVideoAnalysisExecutor(r *router.Router, logger *zap.Logger) *VideoAnalysisExecutor {
	return &VideoAnalysisExecutor{router: r, logger: logger.Named("video-analysis")}
}

// Execute processes a video analysis task by sending the video URL to a multimodal model.
func (e *VideoAnalysisExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input VideoAnalysisInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.VideoURL == "" {
		return "", fmt.Errorf("video_url is required")
	}

	model := input.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	prompt := input.Prompt
	if prompt == "" {
		prompt = "Analyze this video and describe its contents in detail."
	}

	e.logger.Debug("processing video analysis",
		zap.String("video_url", input.VideoURL),
		zap.String("model", model),
	)

	progressFn(10)

	selectedProvider, apiKey, err := e.router.Route(ctx, model)
	if err != nil {
		return "", fmt.Errorf("no provider for model %s: %w", model, err)
	}

	progressFn(20)

	// Build multimodal chat request with video URL reference in prompt
	combinedPrompt := fmt.Sprintf("%s\n\nVideo URL: %s", prompt, input.VideoURL)
	chatReq := &provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: provider.StringContent(combinedPrompt),
			},
		},
		MaxTokens: 4096,
	}

	progressFn(30)

	result, err := e.router.ExecuteChat(ctx, selectedProvider, apiKey, chatReq, 2)
	if err != nil {
		return "", fmt.Errorf("provider error: %w", err)
	}

	progressFn(90)

	analysis := ""
	if result.Response != nil && len(result.Response.Choices) > 0 {
		analysis = result.Response.Choices[0].Message.Content.Text
	}

	resultData, _ := json.Marshal(map[string]interface{}{
		"video_url": input.VideoURL,
		"model":     model,
		"analysis":  analysis,
		"tokens": map[string]int{
			"input":  result.Response.Usage.PromptTokens,
			"output": result.Response.Usage.CompletionTokens,
		},
	})

	progressFn(100)
	return string(resultData), nil
}

// ─── Single TTS Executor ───────────────────────────────────────────

// TTSInput represents the input for a single TTS task.
type TTSInput struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	Model string `json:"model,omitempty"`
}

// TTSExecutor processes single text-to-speech tasks.
type TTSExecutor struct {
	router *router.Router
	logger *zap.Logger
}

// NewTTSExecutor creates a new TTS executor.
func NewTTSExecutor(r *router.Router, logger *zap.Logger) *TTSExecutor {
	return &TTSExecutor{router: r, logger: logger.Named("tts")}
}

// Execute processes a single TTS task.
func (e *TTSExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input TTSInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.Text == "" {
		return "", fmt.Errorf("text is required")
	}

	model := input.Model
	if model == "" {
		model = "tts-1"
	}
	voice := input.Voice
	if voice == "" {
		voice = "alloy"
	}

	e.logger.Debug("processing TTS",
		zap.String("text_preview", truncate(input.Text, 50)),
		zap.String("model", model),
	)

	progressFn(10)

	selectedProvider, apiKey, err := e.router.Route(ctx, model)
	if err != nil {
		return "", fmt.Errorf("no provider for model %s: %w", model, err)
	}

	progressFn(30)

	req := &provider.SpeechRequest{
		Model: model,
		Input: input.Text,
		Voice: voice,
	}

	resp, err := e.router.ExecuteSpeech(ctx, selectedProvider, apiKey, req, 2)
	if err != nil {
		return "", fmt.Errorf("TTS provider error: %w", err)
	}

	progressFn(100)

	result, _ := json.Marshal(map[string]interface{}{
		"text_length": len(input.Text),
		"size_bytes":  len(resp.Response.Audio),
		"status":      "synthesized",
	})
	return string(result), nil
}

// ─── Helpers ───────────────────────────────────────────────────────

// truncate shortens a string to maxLen characters with ellipsis.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// RegisterDefaultExecutors registers all built-in task executors on the pool.
func RegisterDefaultExecutors(pool *WorkerPool, r *router.Router, logger *zap.Logger) {
	pool.RegisterExecutor("tts", NewTTSExecutor(r, logger))
	pool.RegisterExecutor("batch_tts", NewBatchTTSExecutor(r, logger))
	pool.RegisterExecutor("batch_image", NewBatchImageExecutor(r, logger))
	pool.RegisterExecutor("video_analysis", NewVideoAnalysisExecutor(r, logger))
}
