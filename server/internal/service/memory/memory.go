// Package memory provides conversation memory management.
package memory

import (
	"context"
	"encoding/json"
	"time"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles conversation memory.
type Service struct {
	memoryRepo *repository.ConversationMemoryRepository
	redis      *redis.Client
	logger     *zap.Logger
	ttl        time.Duration
}

// NewService creates a new memory service.
func NewService(
	memoryRepo *repository.ConversationMemoryRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
) *Service {
	return &Service{
		memoryRepo: memoryRepo,
		redis:      redisClient,
		logger:     logger,
		ttl:        24 * time.Hour,
	}
}

// Message represents a conversation message.
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	TokenCount int    `json:"token_count"`
}

// AddMessage adds a message to conversation memory.
// L4: Content is encrypted at rest using AES-256-GCM.
func (s *Service) AddMessage(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID, role, content string, tokenCount int) error {
	sequence, err := s.getNextSequence(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return err
	}

	// L4: Encrypt content before storing
	encryptedContent := content
	if crypto.IsInitialized() {
		enc, err := crypto.Encrypt(content)
		if err != nil {
			s.logger.Warn("failed to encrypt conversation content, storing plaintext",
				zap.Error(err),
				zap.String("conversation_id", sanitize.LogValue(conversationID)),
			)
		} else {
			encryptedContent = enc
		}
	}

	memory := &models.ConversationMemory{
		ProjectID:      projectID,
		APIKeyID:       apiKeyID,
		ConversationID: conversationID,
		Role:           role,
		Content:        encryptedContent,
		TokenCount:     tokenCount,
		Sequence:       sequence,
	}

	if err := s.memoryRepo.Create(ctx, memory); err != nil {
		return err
	}

	return s.updateCache(ctx, projectID, apiKeyID, conversationID)
}

// GetConversation retrieves conversation messages.
// L4: Content is decrypted on read.
func (s *Service) GetConversation(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) ([]Message, error) {
	cached, err := s.getFromCache(ctx, projectID, apiKeyID, conversationID)
	if err == nil && cached != nil {
		return cached, nil
	}

	memories, err := s.memoryRepo.GetByConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, len(memories))
	for i, m := range memories {
		content := m.Content
		// L4: Try to decrypt; if it fails, assume plaintext (legacy data)
		if crypto.IsInitialized() {
			if decrypted, err := crypto.Decrypt(content); err == nil {
				content = decrypted
			}
		}
		messages[i] = Message{
			Role:       m.Role,
			Content:    content,
			TokenCount: m.TokenCount,
		}
	}

	_ = s.setCache(ctx, projectID, apiKeyID, conversationID, messages)

	return messages, nil
}

// GetConversationWithLimit retrieves last N messages.
func (s *Service) GetConversationWithLimit(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string, limit int) ([]Message, error) {
	messages, err := s.GetConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return nil, err
	}

	if len(messages) <= limit {
		return messages, nil
	}

	return messages[len(messages)-limit:], nil
}

// ClearConversation deletes all messages in a conversation.
func (s *Service) ClearConversation(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) error {
	if err := s.memoryRepo.DeleteByConversation(ctx, projectID, apiKeyID, conversationID); err != nil {
		return err
	}

	return s.deleteCache(ctx, projectID, apiKeyID, conversationID)
}

// GetConversationTokenCount returns total tokens in conversation.
func (s *Service) GetConversationTokenCount(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) (int, error) {
	messages, err := s.GetConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, m := range messages {
		total += m.TokenCount
	}

	return total, nil
}

// TruncateConversation removes oldest messages to fit token limit.
func (s *Service) TruncateConversation(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string, maxTokens int) error {
	messages, err := s.GetConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return err
	}

	totalTokens := 0
	for _, m := range messages {
		totalTokens += m.TokenCount
	}

	if totalTokens <= maxTokens {
		return nil
	}

	// Count how many of the oldest messages we need to remove
	tokensToRemove := totalTokens - maxTokens
	removed := 0
	messagesToDelete := 0

	for i := 0; i < len(messages) && removed < tokensToRemove; i++ {
		removed += messages[i].TokenCount
		messagesToDelete++
	}

	// Delete the oldest messages from the database
	if messagesToDelete > 0 {
		if err := s.memoryRepo.DeleteOldestByConversation(ctx, projectID, apiKeyID, conversationID, messagesToDelete); err != nil {
			return err
		}
	}

	return s.updateCache(ctx, projectID, apiKeyID, conversationID)
}

// getNextSequence returns the next sequence number.
func (s *Service) getNextSequence(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) (int, error) {
	messages, err := s.memoryRepo.GetByConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return 1, nil
	}
	return len(messages) + 1, nil
}

// cacheKey generates a cache key — includes apiKeyID when present for namespace isolation.
func (s *Service) cacheKey(projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) string {
	key := "conversation:" + projectID.String() + ":"
	if apiKeyID != nil {
		key += apiKeyID.String() + ":"
	}
	key += conversationID
	return key
}

// getFromCache retrieves messages from Redis cache.
func (s *Service) getFromCache(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) ([]Message, error) {
	if s.redis == nil {
		return nil, nil
	}

	key := s.cacheKey(projectID, apiKeyID, conversationID)
	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

// setCache stores messages in Redis cache.
func (s *Service) setCache(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string, messages []Message) error {
	if s.redis == nil {
		return nil
	}

	key := s.cacheKey(projectID, apiKeyID, conversationID)
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return s.redis.Set(ctx, key, data, s.ttl).Err()
}

// updateCache refreshes the cache from database.
func (s *Service) updateCache(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) error {
	memories, err := s.memoryRepo.GetByConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return err
	}

	messages := make([]Message, len(memories))
	for i, m := range memories {
		content := m.Content
		// L4: Decrypt content before caching
		if crypto.IsInitialized() {
			if decrypted, err := crypto.Decrypt(content); err == nil {
				content = decrypted
			}
		}
		messages[i] = Message{
			Role:       m.Role,
			Content:    content,
			TokenCount: m.TokenCount,
		}
	}

	return s.setCache(ctx, projectID, apiKeyID, conversationID, messages)
}

// deleteCache removes conversation from cache.
func (s *Service) deleteCache(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID, conversationID string) error {
	if s.redis == nil {
		return nil
	}

	key := s.cacheKey(projectID, apiKeyID, conversationID)
	return s.redis.Del(ctx, key).Err()
}

// CompressConversation replaces older messages with a summary to reduce token usage.
func (s *Service) CompressConversation(
	ctx context.Context,
	projectID uuid.UUID,
	apiKeyID *uuid.UUID,
	conversationID string,
	maxTokens int,
	keepRecent int,
	summarizer func(messages []Message) (string, int, error),
) error {
	messages, err := s.GetConversation(ctx, projectID, apiKeyID, conversationID)
	if err != nil {
		return err
	}

	totalTokens := 0
	for _, m := range messages {
		totalTokens += m.TokenCount
	}

	// No compression needed
	if totalTokens <= maxTokens || len(messages) <= keepRecent {
		return nil
	}

	// Split into old (to summarize) and recent (to keep)
	splitIdx := len(messages) - keepRecent
	oldMessages := messages[:splitIdx]

	// Generate summary of old messages
	summary, summaryTokens, err := summarizer(oldMessages)
	if err != nil {
		s.logger.Error("failed to generate conversation summary",
			zap.Error(err),
			zap.String("conversation_id", sanitize.LogValue(conversationID)),
		)
		// Fallback: just truncate
		return s.TruncateConversation(ctx, projectID, apiKeyID, conversationID, maxTokens)
	}

	// Delete old messages from DB
	if err := s.memoryRepo.DeleteOldestByConversation(ctx, projectID, apiKeyID, conversationID, splitIdx); err != nil {
		return err
	}

	// Insert summary as a system message at the beginning
	summaryMemory := &models.ConversationMemory{
		ProjectID:      projectID,
		APIKeyID:       apiKeyID,
		ConversationID: conversationID,
		Role:           "system",
		Content:        "[Conversation Summary]\n" + summary,
		TokenCount:     summaryTokens,
		Sequence:       0, // Place at the start
	}

	if err := s.memoryRepo.Create(ctx, summaryMemory); err != nil {
		return err
	}

	s.logger.Info("conversation compressed",
		zap.String("conversation_id", sanitize.LogValue(conversationID)),
		zap.Int("original_messages", len(messages)),
		zap.Int("compressed_to_messages", keepRecent+1),
		zap.Int("original_tokens", totalTokens),
		zap.Int("summary_tokens", summaryTokens),
	)

	return s.updateCache(ctx, projectID, apiKeyID, conversationID)
}

// SummarizeMessages builds a plain-text summary of a list of messages.
func SummarizeMessages(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	var summary string
	summary = "Previous conversation context:\n"
	for _, m := range messages {
		prefix := m.Role
		content := m.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		summary += "- " + prefix + ": " + content + "\n"
	}

	return summary
}

// ListConversations returns all conversation IDs for a project, scoped to API key.
func (s *Service) ListConversations(ctx context.Context, projectID uuid.UUID, apiKeyID *uuid.UUID) ([]string, error) {
	return s.memoryRepo.ListConversationIDs(ctx, projectID, apiKeyID)
}
