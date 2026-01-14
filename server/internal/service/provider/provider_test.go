package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	assert.Equal(t, "gpt-4", req.Model)
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, 1000, req.MaxTokens)
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, how are you?",
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, how are you?", msg.Content)
}

func TestChatResponse(t *testing.T) {
	resp := ChatResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Hello!"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "Hello!", resp.Choices[0].Message.Content)
}

func TestChoice(t *testing.T) {
	choice := Choice{
		Index:        0,
		Message:      Message{Role: "assistant", Content: "Response"},
		FinishReason: "stop",
	}

	assert.Equal(t, 0, choice.Index)
	assert.Equal(t, "stop", choice.FinishReason)
	assert.Equal(t, "assistant", choice.Message.Role)
}

func TestUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}

	assert.Equal(t, 100, usage.PromptTokens)
	assert.Equal(t, 200, usage.CompletionTokens)
	assert.Equal(t, 300, usage.TotalTokens)
}

func TestModelInfo(t *testing.T) {
	model := ModelInfo{
		ID:      "gpt-4",
		Name:    "GPT-4",
		Created: 1678000000,
	}

	assert.Equal(t, "gpt-4", model.ID)
	assert.Equal(t, "GPT-4", model.Name)
	assert.Equal(t, int64(1678000000), model.Created)
}

func TestTokenCalculation(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}

	calculatedTotal := usage.PromptTokens + usage.CompletionTokens
	assert.Equal(t, usage.TotalTokens, calculatedTotal)
}

func TestMultipleMessages(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	assert.Len(t, messages, 4)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "assistant", messages[2].Role)
}

func TestChatRequestWithStream(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Stream test"},
		},
		Stream: true,
	}

	assert.True(t, req.Stream)
}

func TestEmptyResponse(t *testing.T) {
	resp := ChatResponse{}

	assert.Empty(t, resp.ID)
	assert.Empty(t, resp.Model)
	assert.Empty(t, resp.Choices)
	assert.Equal(t, 0, resp.Usage.TotalTokens)
}
