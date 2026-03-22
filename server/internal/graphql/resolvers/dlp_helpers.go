package resolvers

import (
	"llm-router-platform/internal/graphql/model"
	gdbModels "llm-router-platform/internal/models"
)

func mapDlpConfig(m *gdbModels.DlpConfig) *model.DlpConfig {
	if m == nil {
		return nil
	}
	return &model.DlpConfig{
		ID:              m.ID.String(),
		ProjectID:       m.ProjectID.String(),
		IsEnabled:       m.IsEnabled,
		Strategy:        model.DlpStrategy(m.Strategy),
		MaskEmails:      m.MaskEmails,
		MaskPhones:      m.MaskPhones,
		MaskCreditCards: m.MaskCreditCards,
		MaskSsn:         m.MaskSSN,
		MaskAPIKeys:     m.MaskApiKeys,
		CustomRegex:     m.CustomRegex,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
