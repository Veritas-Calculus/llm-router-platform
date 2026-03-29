package resolvers

// This file contains content domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"fmt"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
)

// CreateAnnouncement is the resolver for the createAnnouncement field.
func (r *mutationResolver) CreateAnnouncement(ctx context.Context, input model.AnnouncementInput) (*model.Announcement, error) {
	a := &models.Announcement{Title: input.Title, Content: input.Content}
	if input.Type != nil {
		a.Type = *input.Type
	} else {
		a.Type = "info"
	}
	if input.Priority != nil {
		a.Priority = *input.Priority
	}
	if input.IsActive != nil {
		a.IsActive = *input.IsActive
	}
	a.StartsAt = input.StartsAt
	a.EndsAt = input.EndsAt
	if err := r.AnnouncementSvc.Create(ctx, a); err != nil {
		return nil, err
	}
	return announcementToGQL(a), nil
}

// UpdateAnnouncement is the resolver for the updateAnnouncement field.
func (r *mutationResolver) UpdateAnnouncement(ctx context.Context, id string, input model.AnnouncementInput) (*model.Announcement, error) {
	aid, _ := uuid.Parse(id)
	a, err := r.AnnouncementSvc.GetByID(ctx, aid)
	if err != nil {
		return nil, fmt.Errorf("announcement not found")
	}
	a.Title = input.Title
	a.Content = input.Content
	if input.Type != nil {
		a.Type = *input.Type
	}
	if input.Priority != nil {
		a.Priority = *input.Priority
	}
	if input.IsActive != nil {
		a.IsActive = *input.IsActive
	}
	a.StartsAt = input.StartsAt
	a.EndsAt = input.EndsAt
	if err := r.AnnouncementSvc.Update(ctx, a); err != nil {
		return nil, err
	}
	return announcementToGQL(a), nil
}

// DeleteAnnouncement is the resolver for the deleteAnnouncement field.
func (r *mutationResolver) DeleteAnnouncement(ctx context.Context, id string) (bool, error) {
	aid, _ := uuid.Parse(id)
	return true, r.AnnouncementSvc.Delete(ctx, aid)
}

// CreateCoupon is the resolver for the createCoupon field.
func (r *mutationResolver) CreateCoupon(ctx context.Context, input model.CouponInput) (*model.Coupon, error) {
	c := &models.Coupon{
		Code: input.Code, Name: input.Name, Type: input.Type,
		DiscountValue: input.DiscountValue, IsActive: true,
	}
	if input.MinAmount != nil {
		c.MinAmount = *input.MinAmount
	}
	if input.MaxUses != nil {
		c.MaxUses = *input.MaxUses
	}
	if input.MaxUsesPerUser != nil {
		c.MaxUsesPerUser = *input.MaxUsesPerUser
	} else {
		c.MaxUsesPerUser = 1
	}
	if input.IsActive != nil {
		c.IsActive = *input.IsActive
	}
	c.ExpiresAt = input.ExpiresAt
	if err := r.CouponSvc.Create(ctx, c); err != nil {
		return nil, err
	}
	return couponToGQL(c), nil
}

// UpdateCoupon is the resolver for the updateCoupon field.
func (r *mutationResolver) UpdateCoupon(ctx context.Context, id string, input model.CouponInput) (*model.Coupon, error) {
	cid, _ := uuid.Parse(id)
	c, err := r.CouponSvc.GetByID(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("coupon not found")
	}
	c.Code = input.Code
	c.Name = input.Name
	c.Type = input.Type
	c.DiscountValue = input.DiscountValue
	if input.MinAmount != nil {
		c.MinAmount = *input.MinAmount
	}
	if input.MaxUses != nil {
		c.MaxUses = *input.MaxUses
	}
	if input.MaxUsesPerUser != nil {
		c.MaxUsesPerUser = *input.MaxUsesPerUser
	}
	if input.IsActive != nil {
		c.IsActive = *input.IsActive
	}
	c.ExpiresAt = input.ExpiresAt
	if err := r.CouponSvc.Update(ctx, c); err != nil {
		return nil, err
	}
	return couponToGQL(c), nil
}

// DeleteCoupon is the resolver for the deleteCoupon field.
func (r *mutationResolver) DeleteCoupon(ctx context.Context, id string) (bool, error) {
	cid, _ := uuid.Parse(id)
	return true, r.CouponSvc.Delete(ctx, cid)
}

// CreateDocument is the resolver for the createDocument field.
func (r *mutationResolver) CreateDocument(ctx context.Context, input model.DocumentInput) (*model.Document, error) {
	d := &models.Document{Title: input.Title, Slug: input.Slug, Content: input.Content}
	if input.Category != nil {
		d.Category = *input.Category
	} else {
		d.Category = "general"
	}
	if input.SortOrder != nil {
		d.SortOrder = *input.SortOrder
	}
	if input.IsPublished != nil {
		d.IsPublished = *input.IsPublished
	}
	if err := r.DocumentSvc.Create(ctx, d); err != nil {
		return nil, err
	}
	return documentToGQL(d), nil
}

// UpdateDocument is the resolver for the updateDocument field.
func (r *mutationResolver) UpdateDocument(ctx context.Context, id string, input model.DocumentInput) (*model.Document, error) {
	did, _ := uuid.Parse(id)
	d, err := r.DocumentSvc.GetByID(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("document not found")
	}
	d.Title = input.Title
	d.Slug = input.Slug
	d.Content = input.Content
	if input.Category != nil {
		d.Category = *input.Category
	}
	if input.SortOrder != nil {
		d.SortOrder = *input.SortOrder
	}
	if input.IsPublished != nil {
		d.IsPublished = *input.IsPublished
	}
	if err := r.DocumentSvc.Update(ctx, d); err != nil {
		return nil, err
	}
	return documentToGQL(d), nil
}

// DeleteDocument is the resolver for the deleteDocument field.
func (r *mutationResolver) DeleteDocument(ctx context.Context, id string) (bool, error) {
	did, _ := uuid.Parse(id)
	return true, r.DocumentSvc.Delete(ctx, did)
}

// CreateRoutingRule is the resolver for the createRoutingRule field.
func (r *mutationResolver) CreateRoutingRule(ctx context.Context, input model.CreateRoutingRuleInput) (*model.RoutingRule, error) {
	tid, err := uuid.Parse(input.TargetProviderID)
	if err != nil {
		return nil, fmt.Errorf("invalid target provider id")
	}

	var fid *uuid.UUID
	if input.FallbackProviderID != nil {
		id, err := uuid.Parse(*input.FallbackProviderID)
		if err == nil {
			fid = &id
		}
	}

	rule := &models.RoutingRule{
		Name:               input.Name,
		Description:        derefStr(input.Description),
		ModelPattern:       input.ModelPattern,
		TargetProviderID:   tid,
		FallbackProviderID: fid,
		Priority:           input.Priority,
		IsEnabled:          input.IsEnabled,
	}

	repo := repository.NewRoutingRuleRepository(r.AdminSvc.DB())
	if err := repo.Create(ctx, rule); err != nil {
		return nil, err
	}

	return routingRuleToGQL(rule), nil
}

// UpdateRoutingRule is the resolver for the updateRoutingRule field.
func (r *mutationResolver) UpdateRoutingRule(ctx context.Context, id string, input model.UpdateRoutingRuleInput) (*model.RoutingRule, error) {
	ruleID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid routing rule id")
	}

	repo := repository.NewRoutingRuleRepository(r.AdminSvc.DB())
	rule, err := repo.GetByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		rule.Name = *input.Name
	}
	if input.Description != nil {
		rule.Description = *input.Description
	}
	if input.ModelPattern != nil {
		rule.ModelPattern = *input.ModelPattern
	}
	if input.TargetProviderID != nil {
		if tid, err := uuid.Parse(*input.TargetProviderID); err == nil {
			rule.TargetProviderID = tid
		}
	}
	if input.FallbackProviderID != nil {
		if *input.FallbackProviderID == "" {
			rule.FallbackProviderID = nil
		} else {
			if fid, err := uuid.Parse(*input.FallbackProviderID); err == nil {
				rule.FallbackProviderID = &fid
			}
		}
	}
	if input.Priority != nil {
		rule.Priority = *input.Priority
	}
	if input.IsEnabled != nil {
		rule.IsEnabled = *input.IsEnabled
	}

	if err := repo.Update(ctx, rule); err != nil {
		return nil, err
	}

	return routingRuleToGQL(rule), nil
}

// DeleteRoutingRule is the resolver for the deleteRoutingRule field.
func (r *mutationResolver) DeleteRoutingRule(ctx context.Context, id string) (bool, error) {
	ruleID, err := uuid.Parse(id)
	if err != nil {
		return false, fmt.Errorf("invalid routing rule id")
	}

	repo := repository.NewRoutingRuleRepository(r.AdminSvc.DB())
	if err := repo.Delete(ctx, ruleID); err != nil {
		return false, err
	}
	return true, nil
}

// RoutingRules is the resolver for the routingRules field.
func (r *queryResolver) RoutingRules(ctx context.Context, page *int, pageSize *int) (*model.RoutingRuleList, error) {
	limit := valInt(pageSize, 20)
	offset := (valInt(page, 1) - 1) * limit

	var rules []models.RoutingRule
	var total int64

	q := r.AdminSvc.DB().WithContext(ctx).Model(&models.RoutingRule{})
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	if err := q.Preload("TargetProvider").Preload("FallbackProvider").
		Order("priority DESC, created_at DESC").
		Limit(limit).Offset(offset).Find(&rules).Error; err != nil {
		return nil, err
	}

	out := make([]*model.RoutingRule, len(rules))
	for i := range rules {
		out[i] = routingRuleToGQL(&rules[i])
	}

	return &model.RoutingRuleList{
		Data:     out,
		Total:    int(total),
		Page:     valInt(page, 1),
		PageSize: limit,
	}, nil
}

// ActiveAnnouncements is the resolver for the activeAnnouncements field.
func (r *queryResolver) ActiveAnnouncements(ctx context.Context) ([]*model.Announcement, error) {
	list, err := r.AnnouncementSvc.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Announcement, len(list))
	for i := range list {
		out[i] = announcementToGQL(&list[i])
	}
	return out, nil
}

// Announcements is the resolver for the announcements field.
func (r *queryResolver) Announcements(ctx context.Context) ([]*model.Announcement, error) {
	list, err := r.AnnouncementSvc.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Announcement, len(list))
	for i := range list {
		out[i] = announcementToGQL(&list[i])
	}
	return out, nil
}

// Coupons is the resolver for the coupons field.
func (r *queryResolver) Coupons(ctx context.Context) ([]*model.Coupon, error) {
	list, err := r.CouponSvc.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Coupon, len(list))
	for i := range list {
		out[i] = couponToGQL(&list[i])
	}
	return out, nil
}

// Coupon is the resolver for the coupon field.
func (r *queryResolver) Coupon(ctx context.Context, id string) (*model.Coupon, error) {
	cid, _ := uuid.Parse(id)
	c, err := r.CouponSvc.GetByID(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("coupon not found")
	}
	return couponToGQL(c), nil
}

// Documents is the resolver for the documents field.
func (r *queryResolver) Documents(ctx context.Context) ([]*model.Document, error) {
	list, err := r.DocumentSvc.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Document, len(list))
	for i := range list {
		out[i] = documentToGQL(&list[i])
	}
	return out, nil
}

// PublishedDocuments is the resolver for the publishedDocuments field.
func (r *queryResolver) PublishedDocuments(ctx context.Context) ([]*model.Document, error) {
	list, err := r.DocumentSvc.GetPublished(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Document, len(list))
	for i := range list {
		out[i] = documentToGQL(&list[i])
	}
	return out, nil
}

// Document is the resolver for the document field.
func (r *queryResolver) Document(ctx context.Context, id string) (*model.Document, error) {
	did, _ := uuid.Parse(id)
	d, err := r.DocumentSvc.GetByID(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("document not found")
	}
	return documentToGQL(d), nil
}
