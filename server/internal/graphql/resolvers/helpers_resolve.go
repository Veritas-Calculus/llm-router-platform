package resolvers

// Domain helpers: helpers_resolve

import (
	"context"
	"fmt"
	"llm-router-platform/internal/graphql/directives"

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

func (r *Resolver) resolveOrgProjectIDs(ctx context.Context, providedOrgID *string, providedProjectID *string) (uuid.UUID, *uuid.UUID, error) {
	orgID, err := r.resolveOrgID(ctx, providedOrgID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	projectID := r.resolveProjectID(providedProjectID)
	return orgID, projectID, nil
}
