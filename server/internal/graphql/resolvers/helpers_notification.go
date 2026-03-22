package resolvers

import (
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
)

// notifChannelToGQL converts a DB notification channel to a GraphQL model.
func notifChannelToGQL(ch *models.NotificationChannel) *model.NotificationChannel {
	return &model.NotificationChannel{
		ID:        ch.ID.String(),
		Name:      ch.Name,
		Type:      ch.Type,
		IsEnabled: ch.IsEnabled,
		Config:    ch.Config,
		CreatedAt: ch.CreatedAt,
		UpdatedAt: ch.UpdatedAt,
	}
}
