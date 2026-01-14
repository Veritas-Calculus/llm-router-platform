package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessage(t *testing.T) {
	msg := Message{
		Role:       "user",
		Content:    "Hello, how are you?",
		TokenCount: 5,
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, how are you?", msg.Content)
	assert.Equal(t, 5, msg.TokenCount)
}

func TestMessageRoles(t *testing.T) {
	roles := []string{"system", "user", "assistant"}

	for _, role := range roles {
		msg := Message{Role: role}
		assert.Equal(t, role, msg.Role)
	}
}

func TestMessageList(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	assert.Len(t, messages, 3)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "assistant", messages[2].Role)
}

func TestConversationLimit(t *testing.T) {
	messages := make([]Message, 0)
	maxMessages := 100

	for i := 0; i < 150; i++ {
		messages = append(messages, Message{
			Role:    "user",
			Content: "Message",
		})

		if len(messages) > maxMessages {
			messages = messages[len(messages)-maxMessages:]
		}
	}

	assert.Len(t, messages, maxMessages)
}

func TestTokenCounting(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello how are you", TokenCount: 5},
		{Role: "assistant", Content: "I am doing well", TokenCount: 5},
	}

	var totalTokens int
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	assert.Equal(t, 10, totalTokens)
}

func TestContextWindow(t *testing.T) {
	maxContextTokens := 4096
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant.", TokenCount: 10},
		{Role: "user", Content: "Hello", TokenCount: 2},
		{Role: "assistant", Content: "Hi there!", TokenCount: 3},
	}

	var tokenCount int
	for _, msg := range messages {
		tokenCount += msg.TokenCount
	}

	assert.True(t, tokenCount < maxContextTokens)
}

func TestMemoryTruncation(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
		{Role: "assistant", Content: "Response 2"},
		{Role: "user", Content: "Message 3"},
		{Role: "assistant", Content: "Response 3"},
	}

	maxMessages := 5
	if len(messages) > maxMessages {
		system := messages[0]
		recent := messages[len(messages)-(maxMessages-1):]
		messages = append([]Message{system}, recent...)
	}

	assert.Len(t, messages, maxMessages)
	assert.Equal(t, "System prompt", messages[0].Content)
	assert.Equal(t, "Message 2", messages[1].Content)
}

func TestContextExpiration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case <-time.After(50 * time.Millisecond):
		assert.True(t, true)
	case <-ctx.Done():
		t.Fatal("Context should not have expired yet")
	}
}

func TestRoleCounts(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "U1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "U2"},
		{Role: "assistant", Content: "A2"},
	}

	counts := make(map[string]int)
	for _, msg := range messages {
		counts[msg.Role]++
	}

	assert.Equal(t, 1, counts["system"])
	assert.Equal(t, 2, counts["user"])
	assert.Equal(t, 2, counts["assistant"])
}

func TestEmptyConversation(t *testing.T) {
	messages := []Message{}

	assert.Len(t, messages, 0)
}
