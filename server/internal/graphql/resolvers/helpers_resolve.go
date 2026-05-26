package resolvers

// Domain helpers: helpers_resolve

import (
	"context"
	"fmt"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/models"
	"strings"

	"github.com/google/uuid"
)

func (r *Resolver) resolveOrgID(ctx context.Context, providedOrgID *string) (uuid.UUID, error) {
	uidStr, _ := directives.UserIDFromContext(ctx)
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID in context")
	}

	if providedOrgID != nil && *providedOrgID != "" {
		orgID, err := uuid.Parse(*providedOrgID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid org ID")
		}
		// Validate the user actually belongs to this org (IDOR prevention)
		if err := r.UserSvc.RequireOrgRole(ctx, uidStr, *providedOrgID, "OWNER", "ADMIN", "MEMBER", "READONLY"); err != nil {
			return uuid.Nil, fmt.Errorf("forbidden: access denied")
		}
		return orgID, nil
	}

	orgs, err := r.UserSvc.GetOrganizations(ctx, userID)
	if err != nil || len(orgs) == 0 {
		return uuid.Nil, fmt.Errorf("no organization found for user")
	}
	return orgs[0].ID, nil
}

func (r *Resolver) resolveProjectID(providedProjectID *string) *uuid.UUID {
	if providedProjectID != nil && *providedProjectID != "" {
		id, err := uuid.Parse(*providedProjectID)
		if err == nil {
			return &id
		}
	}
	return nil
}

func (r *Resolver) resolveAccessibleProjectID(ctx context.Context, providedProjectID *string, allowedRoles ...string) (uuid.UUID, error) {
	uidStr, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	if providedProjectID != nil && *providedProjectID != "" {
		projectID, err := uuid.Parse(*providedProjectID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid project ID")
		}
		if err := r.UserSvc.RequireProjectRole(ctx, uidStr, projectID.String(), allowedRoles...); err != nil {
			return uuid.Nil, fmt.Errorf("forbidden: access denied")
		}
		return projectID, nil
	}

	var project models.Project
	db := r.AdminSvc.DB().WithContext(ctx)
	err = db.
		Joins("JOIN organization_members om ON om.org_id = projects.org_id AND om.user_id = ?", uidStr).
		Order("projects.created_at DESC").
		First(&project).Error
	if err == nil {
		return project.ID, nil
	}

	var user struct{ Role string }
	if db.Table("users").Select("role").Where("id = ?", uidStr).First(&user).Error == nil && strings.EqualFold(user.Role, "admin") {
		if err := db.Order("created_at DESC").First(&project).Error; err == nil {
			return project.ID, nil
		}
	}

	return uuid.Nil, fmt.Errorf("no active project")
}

func (r *Resolver) currentUserIsPlatformAdmin(ctx context.Context) bool {
	uidStr, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return false
	}
	var user struct{ Role string }
	err = r.AdminSvc.DB().WithContext(ctx).
		Table("users").
		Select("role").
		Where("id = ?", uidStr).
		First(&user).Error
	return err == nil && strings.EqualFold(user.Role, "admin")
}

func (r *Resolver) resolveUsageScope(ctx context.Context, providedProjectID *string, allowedRoles ...string) (uuid.UUID, *uuid.UUID, bool, error) {
	if (providedProjectID == nil || *providedProjectID == "") && r.currentUserIsPlatformAdmin(ctx) {
		return uuid.Nil, nil, true, nil
	}

	projectID, err := r.resolveAccessibleProjectID(ctx, providedProjectID, allowedRoles...)
	if err != nil {
		return uuid.Nil, nil, false, err
	}

	var project struct{ OrgID uuid.UUID }
	if err := r.AdminSvc.DB().WithContext(ctx).
		Table("projects").
		Select("org_id").
		Where("id = ?", projectID).
		First(&project).Error; err != nil {
		return uuid.Nil, nil, false, fmt.Errorf("project not found")
	}

	return project.OrgID, &projectID, false, nil
}

func (r *Resolver) resolveOrgProjectIDs(ctx context.Context, providedOrgID *string, providedProjectID *string) (uuid.UUID, *uuid.UUID, error) {
	orgID, err := r.resolveOrgID(ctx, providedOrgID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	projectID := r.resolveProjectID(providedProjectID)
	return orgID, projectID, nil
}
