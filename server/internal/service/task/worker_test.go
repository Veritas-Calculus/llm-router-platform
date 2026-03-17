package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ─── Test Helpers ──────────────────────────────────────────────────

var testLogger, _ = zap.NewDevelopment()

func noopProgress(_ int) {}

func testAsyncTask(taskType, input string) *models.AsyncTask {
	return &models.AsyncTask{
		BaseModel: models.BaseModel{ID: uuid.New()},
		UserID:    uuid.New(),
		Type:      taskType,
		Status:    "pending",
		Input:     input,
	}
}

// ─── Executor Unit Tests ───────────────────────────────────────────

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello…"},
		{"hello\nworld", 20, "hello world"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestBatchTTSExecutor_InvalidInput(t *testing.T) {
	exec := NewBatchTTSExecutor(testLogger)
	task := testAsyncTask("batch_tts", "invalid json")
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestBatchTTSExecutor_EmptyItems(t *testing.T) {
	exec := NewBatchTTSExecutor(testLogger)
	input, _ := json.Marshal(BatchTTSInput{Items: []BatchTTSItem{}})
	task := testAsyncTask("batch_tts", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for empty items")
	}
}

func TestBatchTTSExecutor_Success(t *testing.T) {
	exec := NewBatchTTSExecutor(testLogger)
	input, _ := json.Marshal(BatchTTSInput{
		Items: []BatchTTSItem{
			{Text: "Hello"},
			{Text: "World"},
		},
	})
	task := testAsyncTask("batch_tts", string(input))

	var lastProgress int
	progressFn := func(pct int) { lastProgress = pct }

	result, err := exec.Execute(context.Background(), task, progressFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	if lastProgress != 100 {
		t.Errorf("expected final progress 100, got %d", lastProgress)
	}
}

func TestBatchImageExecutor_InvalidInput(t *testing.T) {
	exec := NewBatchImageExecutor(testLogger)
	task := testAsyncTask("batch_image", "bad")
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestBatchImageExecutor_Success(t *testing.T) {
	exec := NewBatchImageExecutor(testLogger)
	input, _ := json.Marshal(BatchImageInput{Prompts: []string{"a cat", "a dog"}})
	task := testAsyncTask("batch_image", string(input))

	result, err := exec.Execute(context.Background(), task, noopProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestVideoAnalysisExecutor_MissingURL(t *testing.T) {
	exec := NewVideoAnalysisExecutor(testLogger)
	input, _ := json.Marshal(VideoAnalysisInput{VideoURL: ""})
	task := testAsyncTask("video_analysis", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for missing video_url")
	}
}

func TestVideoAnalysisExecutor_Success(t *testing.T) {
	exec := NewVideoAnalysisExecutor(testLogger)
	input, _ := json.Marshal(VideoAnalysisInput{VideoURL: "https://example.com/test.mp4"})
	task := testAsyncTask("video_analysis", string(input))

	result, err := exec.Execute(context.Background(), task, noopProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestTTSExecutor_EmptyText(t *testing.T) {
	exec := NewTTSExecutor(testLogger)
	input, _ := json.Marshal(TTSInput{Text: ""})
	task := testAsyncTask("tts", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestTTSExecutor_Success(t *testing.T) {
	exec := NewTTSExecutor(testLogger)
	input, _ := json.Marshal(TTSInput{Text: "Hello world"})
	task := testAsyncTask("tts", string(input))

	result, err := exec.Execute(context.Background(), task, noopProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	exec := NewBatchTTSExecutor(testLogger)
	input, _ := json.Marshal(BatchTTSInput{
		Items: make([]BatchTTSItem, 100), // many items to process
	})
	for i := range 100 {
		inputParsed := BatchTTSInput{}
		_ = json.Unmarshal(input, &inputParsed)
		inputParsed.Items[i] = BatchTTSItem{Text: "test"}
		input, _ = json.Marshal(inputParsed)
	}

	task := testAsyncTask("batch_tts", string(input))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := exec.Execute(ctx, task, noopProgress)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

// ─── WorkerPool Config Tests ──────────────────────────────────────

func TestDefaultWorkerPoolConfig(t *testing.T) {
	cfg := DefaultWorkerPoolConfig()
	if cfg.Concurrency != 4 {
		t.Errorf("expected concurrency 4, got %d", cfg.Concurrency)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected poll interval 5s, got %v", cfg.PollInterval)
	}
	if cfg.StaleTimeout != 30*time.Minute {
		t.Errorf("expected stale timeout 30m, got %v", cfg.StaleTimeout)
	}
}

func TestRegisterDefaultExecutors(t *testing.T) {
	pool := NewWorkerPool(nil, nil, DefaultWorkerPoolConfig(), testLogger)
	RegisterDefaultExecutors(pool, testLogger)

	expectedTypes := []string{"tts", "batch_tts", "batch_image", "video_analysis"}
	for _, typ := range expectedTypes {
		if _, ok := pool.executors[typ]; !ok {
			t.Errorf("expected executor for type %q to be registered", typ)
		}
	}
}
