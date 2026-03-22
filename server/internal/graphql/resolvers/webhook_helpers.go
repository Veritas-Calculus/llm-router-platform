package resolvers

import (
	"llm-router-platform/internal/graphql/model"
	gdbModels "llm-router-platform/internal/models"
)

func mapWebhookEndpoint(m *gdbModels.WebhookEndpoint, includeSecret bool) *model.WebhookEndpoint {
	if m == nil {
		return nil
	}
	var secret *string
	if includeSecret {
		s := m.Secret
		secret = &s
	}
	var desc *string
	if m.Description != "" {
		s := m.Description
		desc = &s
	}
	return &model.WebhookEndpoint{
		ID:          m.ID.String(),
		ProjectID:   m.ProjectID.String(),
		URL:         m.URL,
		Secret:      secret,
		Events:      m.Events,
		IsActive:    m.IsActive,
		Description: desc,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func mapWebhookDelivery(m *gdbModels.WebhookDelivery) *model.WebhookDelivery {
	if m == nil {
		return nil
	}
	var resp *string
	if m.ResponseBody != "" {
		s := m.ResponseBody
		resp = &s
	}
	var errMsg *string
	if m.ErrorMessage != "" {
		s := m.ErrorMessage
		errMsg = &s
	}
	return &model.WebhookDelivery{
		ID:           m.ID.String(),
		EndpointID:   m.EndpointID.String(),
		EventType:    m.EventType,
		Payload:      m.Payload,
		StatusCode:   m.StatusCode,
		ResponseBody: resp,
		ErrorMessage: errMsg,
		CreatedAt:    m.CreatedAt,
	}
}
