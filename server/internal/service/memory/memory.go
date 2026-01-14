// Package memory provides conversation memory management.
package memory

import (
	"context"
	"encoding/json"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

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
func (s *Service) AddMessage(ctx context.Context, userID uuid.UUID, conversationID, role, content string, tokenCount int) error {
	sequence, err := s.getNextSequence(ctx, userID, conversationID)
	if err != nil {
		return err
	}

	memory := &models.ConversationMemory{
		UserID:         userID,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokenCount:     tokenCount,
		Sequence:       sequence,
	}

	if err := s.memoryRepo.Create(ctx, memory); err != nil {
		return err
	}

	return s.updateCache(ctx, userID, conversationID)
}

// GetConversation retrieves conversation messages.
func (s *Service) GetConversation(ctx context.Context, userID uuid.UUID, conversationID string) ([]Message, error) {
	cached, err := s.getFromCache(ctx, userID, conversationID)
	if err == nil && cached != nil {
		return cached, nil
	}

	memories, err := s.memoryRepo.GetByConversation(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, len(memories))
	for i, m := range memories {
		messages[i] = Message{
			Role:       m.Role,
			Content:    m.Content,
			TokenCount: m.TokenCount,
		}
	}

	_ = s.setCache(ctx, userID, conversationID, messages)

	return messages, nil
}

// GetConversationWithLimit retrieves last N messages.
func (s *Service) GetConversationWithLimit(ctx context.Context, userID uuid.UUID, conversationID string, limit int) ([]Message, error) {
	messages, err := s.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}

	if len(messages) <= limit {
		return messages, nil
	}

	return messages[len(messages)-limit:], nil
}

// ClearConversation deletes all messages in a conversation.
func (s *Service) ClearConversation(ctx context.Context, userID uuid.UUID, conversationID string) error {
	if err := s.memoryRepo.DeleteByConversation(ctx, userID, conversationID); err != nil {
		return err
	}

	return s.deleteCache(ctx, userID, conversationID)
}

// GetConversationTokenCount returns total tokens in conversation.
func (s *Service) GetConversationTokenCount(ctx context.Context, userID uuid.UUID, conversationID string) (int, error) {
	messages, err := s.GetConversation(ctx, userID, conversationID)
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
func (s *Service) TruncateConversation(ctx context.Context, userID uuid.UUID, conversationID string, maxTokens int) error {
	messages, err := s.GetConversation(ctx, userID, conversationID)
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

	tokensToRemove := totalTokens - maxTokens
	removed := 0

	for i := 0; i < len(messages) && removed < tokensToRemove; i++ {
		removed += messages[i].TokenCount
	}

	return s.updateCache(ctx, userID, conversationID)
}

// getNextSequence returns the next sequence number.
func (s *Service) getNextSequence(ctx context.Context, userID uuid.UUID, conversationID string) (int, error) {
	messages, err := s.memoryRepo.GetByConversation(ctx, userID, conversationID)
	if err != nil {
		return 1, nil
	}
	return len(messages) + 1, nil
}

// cacheKey generates a cache key.
func (s *Service) cacheKey(userID uuid.UUID, conversationID string) string {
	return "conversation:" + userID.String() + ":" + conversationID
}

// getFromCache retrieves messages from Redis cache.
func (s *Service) getFromCache(ctx context.Context, userID uuid.UUID, conversationID string) ([]Message, error) {
	if s.redis == nil {
		return nil, nil
	}

	key := s.cacheKey(userID, conversationID)
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
func (s *Service) setCache(ctx context.Context, userID uuid.UUID, conversationID string, messages []Message) error {
	if s.redis == nil {
		return nil
	}

	key := s.cacheKey(userID, conversationID)
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return s.redis.Set(ctx, key, data, s.ttl).Err()
}

// updateCache refreshes the cache from database.
func (s *Service) updateCache(ctx context.Context, userID uuid.UUID, conversationID string) error {
	memories, err := s.memoryRepo.GetByConversation(ctx, userID, conversationID)
	if err != nil {
		return err
	}

	messages := make([]Message, len(memories))
	for i, m := range memories {
		messages[i] = Message{
			Role:       m.Role,
			Content:    m.Content,
			TokenCount: m.TokenCount,
		}
	}

	return s.setCache(ctx, userID, conversationID, messages)
}

// deleteCache removes conversation from cache.
func (s *Service) deleteCache(ctx context.Context, userID uuid.UUID, conversationID string) error {
	if s.redis == nil {
		return nil
	}

	key := s.cacheKey(userID, conversationID)
	return s.redis.Del(ctx, key).Err()
}

// ListConversations returns all conversation IDs for a user.
func (s *Service) ListConversations(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return s.memoryRepo.ListConversationIDs(ctx, userID)
}
