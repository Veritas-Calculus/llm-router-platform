package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: StringContent("Hello")},
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
		Content: StringContent("Hello, how are you?"),
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, how are you?", msg.Content.Text)
}

func TestChatResponse(t *testing.T) {
	resp := ChatResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: StringContent("Hello!")},
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
	assert.Equal(t, "Hello!", resp.Choices[0].Message.Content.Text)
}

func TestChoice(t *testing.T) {
	choice := Choice{
		Index:        0,
		Message:      Message{Role: "assistant", Content: StringContent("Response")},
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
		{Role: "system", Content: StringContent("You are a helpful assistant.")},
		{Role: "user", Content: StringContent("Hello")},
		{Role: "assistant", Content: StringContent("Hi there!")},
		{Role: "user", Content: StringContent("How are you?")},
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
			{Role: "user", Content: StringContent("Stream test")},
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

func TestSpeechRequest(t *testing.T) {
	req := SpeechRequest{
		Model:          "tts-1",
		Input:          "Hello, this is a test.",
		Voice:          "alloy",
		ResponseFormat: "mp3",
		Speed:          1.0,
	}

	assert.Equal(t, "tts-1", req.Model)
	assert.Equal(t, "Hello, this is a test.", req.Input)
	assert.Equal(t, "alloy", req.Voice)
	assert.Equal(t, "mp3", req.ResponseFormat)
	assert.Equal(t, 1.0, req.Speed)
}

func TestSpeechRequestDefaults(t *testing.T) {
	req := SpeechRequest{
		Model: "tts-1-hd",
		Input: "Test",
		Voice: "nova",
	}

	assert.Equal(t, "tts-1-hd", req.Model)
	assert.Empty(t, req.ResponseFormat)
	assert.Equal(t, 0.0, req.Speed)
}

func TestSpeechResponse(t *testing.T) {
	resp := SpeechResponse{
		Audio:       []byte{0x49, 0x44, 0x33}, // ID3 header
		ContentType: "audio/mpeg",
	}

	assert.Equal(t, 3, len(resp.Audio))
	assert.Equal(t, "audio/mpeg", resp.ContentType)
}

func TestCapabilityConstants(t *testing.T) {
	assert.Equal(t, Capability("chat"), CapChat)
	assert.Equal(t, Capability("stream"), CapStream)
	assert.Equal(t, Capability("embeddings"), CapEmbeddings)
	assert.Equal(t, Capability("image"), CapImage)
	assert.Equal(t, Capability("audio"), CapAudio)
	assert.Equal(t, Capability("tts"), CapTTS)
	assert.Equal(t, Capability("video"), CapVideo)
}

func TestRegistryTTSCapability(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewRegistry(logger)

	// Register a mock provider with TTS capability
	registry.RegisterWithCapabilities("test-tts", nil, CapChat, CapTTS)

	assert.True(t, registry.HasCapability("test-tts", CapTTS))
	assert.True(t, registry.HasCapability("test-tts", CapChat))
	assert.False(t, registry.HasCapability("test-tts", CapImage))
	assert.False(t, registry.HasCapability("nonexistent", CapTTS))

	matrix := registry.CapabilityMatrix()
	assert.Contains(t, matrix, "test-tts")
	assert.Contains(t, matrix["test-tts"], CapTTS)
	assert.Contains(t, matrix["test-tts"], CapChat)
}

func TestFlexibleContentVideoTransparency(t *testing.T) {
	// Multimodal content with video_url part (used by Gemini 2.x, GPT-4o)
	rawJSON := `[{"type":"text","text":"Describe this video"},{"type":"video_url","video_url":{"url":"https://example.com/video.mp4"}}]`

	var fc FlexibleContent
	err := json.Unmarshal([]byte(rawJSON), &fc)
	require.NoError(t, err)

	// Text should be extracted
	assert.Equal(t, "Describe this video", fc.Text)

	// Raw should preserve the original JSON including the video_url part
	assert.NotNil(t, fc.Raw)

	// MarshalJSON should re-emit the original JSON (transparent forwarding)
	marshaled, err := fc.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, rawJSON, string(marshaled))
}

func TestFlexibleContentImageURLTransparency(t *testing.T) {
	// Multimodal content with image_url part (GPT-4 Vision, Gemini)
	rawJSON := `[{"type":"text","text":"What's in this image?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBOR..."}}]`

	var fc FlexibleContent
	err := json.Unmarshal([]byte(rawJSON), &fc)
	require.NoError(t, err)

	assert.Equal(t, "What's in this image?", fc.Text)

	// Raw should preserve image_url part as-is
	marshaled, err := fc.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, rawJSON, string(marshaled))
}

func TestFlexibleContentMixedMultimodal(t *testing.T) {
	// Content with text + image + video (OpenClaw scenario: video editing)
	rawJSON := `[{"type":"text","text":"Analyze this frame"},{"type":"image_url","image_url":{"url":"https://cdn.example.com/frame001.jpg"}},{"type":"video_url","video_url":{"url":"https://cdn.example.com/clip.mp4","detail":"low"}}]`

	var fc FlexibleContent
	err := json.Unmarshal([]byte(rawJSON), &fc)
	require.NoError(t, err)

	assert.Equal(t, "Analyze this frame", fc.Text)

	// Full roundtrip preserves all parts
	marshaled, err := fc.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, rawJSON, string(marshaled))

	// Verify it works in a ChatRequest context
	chatReq := ChatRequest{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: fc},
		},
	}

	chatJSON, err := json.Marshal(chatReq)
	require.NoError(t, err)
	assert.Contains(t, string(chatJSON), "video_url")
	assert.Contains(t, string(chatJSON), "image_url")
}

func TestFlexibleContentNullPreserved(t *testing.T) {
	// Tool call response with null content
	var fc FlexibleContent
	err := json.Unmarshal([]byte("null"), &fc)
	require.NoError(t, err)

	assert.Empty(t, fc.Text)

	marshaled, err := fc.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "null", string(marshaled))
}

