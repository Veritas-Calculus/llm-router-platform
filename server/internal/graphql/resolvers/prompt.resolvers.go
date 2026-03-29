package resolvers

// This file contains prompt domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"encoding/json"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
)

// CreatePromptTemplate is the resolver for the createPromptTemplate field.
func (r *mutationResolver) CreatePromptTemplate(ctx context.Context, input model.PromptTemplateInput) (*model.PromptTemplate, error) {
	t := &models.PromptTemplate{
		Name: input.Name, IsActive: true,
	}
	if input.Description != nil {
		t.Description = *input.Description
	}
	if input.ProjectID != nil {
		pid, _ := uuid.Parse(*input.ProjectID)
		t.ProjectID = &pid
	}
	if input.IsActive != nil {
		t.IsActive = *input.IsActive
	}
	if err := r.AdminSvc.DB().WithContext(ctx).Create(t).Error; err != nil {
		return nil, err
	}
	return promptTemplateToGQL(t, 0), nil
}

// UpdatePromptTemplate is the resolver for the updatePromptTemplate field.
func (r *mutationResolver) UpdatePromptTemplate(ctx context.Context, id string, input model.PromptTemplateInput) (*model.PromptTemplate, error) {
	tid, _ := uuid.Parse(id)
	var t models.PromptTemplate
	if err := r.AdminSvc.DB().WithContext(ctx).First(&t, "id = ?", tid).Error; err != nil {
		return nil, err
	}
	t.Name = input.Name
	if input.Description != nil {
		t.Description = *input.Description
	}
	if input.ProjectID != nil {
		pid, _ := uuid.Parse(*input.ProjectID)
		t.ProjectID = &pid
	}
	if input.IsActive != nil {
		t.IsActive = *input.IsActive
	}
	if err := r.AdminSvc.DB().WithContext(ctx).Save(&t).Error; err != nil {
		return nil, err
	}
	var count int64
	r.AdminSvc.DB().WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", tid).Count(&count)
	return promptTemplateToGQL(&t, int(count)), nil
}

// DeletePromptTemplate is the resolver for the deletePromptTemplate field.
func (r *mutationResolver) DeletePromptTemplate(ctx context.Context, id string) (bool, error) {
	tid, _ := uuid.Parse(id)
	if err := r.AdminSvc.DB().WithContext(ctx).Delete(&models.PromptTemplate{}, "id = ?", tid).Error; err != nil {
		return false, err
	}
	return true, nil
}

// CreatePromptVersion is the resolver for the createPromptVersion field.
func (r *mutationResolver) CreatePromptVersion(ctx context.Context, input model.PromptVersionInput) (*model.PromptVersion, error) {
	tmplID, _ := uuid.Parse(input.TemplateID)
	// Get max version number
	var maxVersion int
	r.AdminSvc.DB().WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", tmplID).
		Select("COALESCE(MAX(version), 0)").Scan(&maxVersion)

	v := &models.PromptVersion{
		TemplateID: tmplID,
		Version:    maxVersion + 1,
		Content:    input.Content,
	}
	if input.Model != nil {
		v.Model = *input.Model
	}
	if input.Parameters != nil {
		v.Parameters = json.RawMessage(*input.Parameters)
	}
	if input.ChangeLog != nil {
		v.ChangeLog = *input.ChangeLog
	}
	if err := r.AdminSvc.DB().WithContext(ctx).Create(v).Error; err != nil {
		return nil, err
	}
	return promptVersionToGQL(v), nil
}

// SetActivePromptVersion is the resolver for the setActivePromptVersion field.
func (r *mutationResolver) SetActivePromptVersion(ctx context.Context, templateID string, versionID string) (*model.PromptTemplate, error) {
	tid, _ := uuid.Parse(templateID)
	vid, _ := uuid.Parse(versionID)
	if err := r.AdminSvc.DB().WithContext(ctx).Model(&models.PromptTemplate{}).Where("id = ?", tid).
		Update("active_version_id", vid).Error; err != nil {
		return nil, err
	}
	var t models.PromptTemplate
	if err := r.AdminSvc.DB().WithContext(ctx).First(&t, "id = ?", tid).Error; err != nil {
		return nil, err
	}
	var count int64
	r.AdminSvc.DB().WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", tid).Count(&count)
	result := promptTemplateToGQL(&t, int(count))
	// Attach active version details
	if t.ActiveVersionID != nil {
		var av models.PromptVersion
		if err := r.AdminSvc.DB().WithContext(ctx).First(&av, "id = ?", *t.ActiveVersionID).Error; err == nil {
			gqlV := promptVersionToGQL(&av)
			result.ActiveVersion = gqlV
		}
	}
	return result, nil
}

// PromptTemplates is the resolver for the promptTemplates field.
func (r *queryResolver) PromptTemplates(ctx context.Context) (*model.PromptTemplateConnection, error) {
	var templates []models.PromptTemplate
	if err := r.AdminSvc.DB().WithContext(ctx).Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	out := make([]*model.PromptTemplate, len(templates))
	for i, t := range templates {
		var count int64
		r.AdminSvc.DB().WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", t.ID).Count(&count)
		out[i] = promptTemplateToGQL(&t, int(count))
	}
	return &model.PromptTemplateConnection{Data: out, Total: len(out)}, nil
}

// PromptTemplate is the resolver for the promptTemplate field.
func (r *queryResolver) PromptTemplate(ctx context.Context, id string) (*model.PromptTemplate, error) {
	tid, _ := uuid.Parse(id)
	var t models.PromptTemplate
	if err := r.AdminSvc.DB().WithContext(ctx).First(&t, "id = ?", tid).Error; err != nil {
		return nil, err
	}
	var count int64
	r.AdminSvc.DB().WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", tid).Count(&count)
	result := promptTemplateToGQL(&t, int(count))
	if t.ActiveVersionID != nil {
		var av models.PromptVersion
		if err := r.AdminSvc.DB().WithContext(ctx).First(&av, "id = ?", *t.ActiveVersionID).Error; err == nil {
			result.ActiveVersion = promptVersionToGQL(&av)
		}
	}
	return result, nil
}

// PromptVersions is the resolver for the promptVersions field.
func (r *queryResolver) PromptVersions(ctx context.Context, templateID string) ([]*model.PromptVersion, error) {
	tid, _ := uuid.Parse(templateID)
	var versions []models.PromptVersion
	if err := r.AdminSvc.DB().WithContext(ctx).Where("template_id = ?", tid).Order("version DESC").Find(&versions).Error; err != nil {
		return nil, err
	}
	out := make([]*model.PromptVersion, len(versions))
	for i, v := range versions {
		out[i] = promptVersionToGQL(&v)
	}
	return out, nil
}
