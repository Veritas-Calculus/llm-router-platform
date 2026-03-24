package directives

import (
	"context"
	"fmt"
)

// ValidateOwnership checks that the authenticated user has an active membership
// in the organization or project they are trying to access.
//
// resourceType must be "org" or "project".
// resourceID is the string UUID of the org or project.
// requiredRoles are the roles that grant access (e.g. "OWNER", "ADMIN", "MEMBER").
//
// Usage in resolvers:
//
//	if err := directives.ValidateOwnership(ctx, r.UserSvc, "project", projectID, "admin", "member"); err != nil {
//	    return nil, err
//	}
type OwnershipValidator interface {
	RequireOrgRole(ctx context.Context, userID, orgID string, roles ...string) error
	RequireProjectRole(ctx context.Context, userID, projectID string, roles ...string) error
}

// ValidateOwnership extracts the user from context and validates they have
// an appropriate role on the specified resource. Returns a "forbidden: access denied"
// error that passes through the GraphQL error masking whitelist.
func ValidateOwnership(ctx context.Context, svc OwnershipValidator, resourceType string, resourceID string, roles ...string) error {
	userID, err := UserIDFromContext(ctx)
	if err != nil {
		return fmt.Errorf("unauthorized")
	}

	switch resourceType {
	case "org":
		if err := svc.RequireOrgRole(ctx, userID, resourceID, roles...); err != nil {
			return fmt.Errorf("forbidden: access denied")
		}
	case "project":
		if err := svc.RequireProjectRole(ctx, userID, resourceID, roles...); err != nil {
			return fmt.Errorf("forbidden: access denied")
		}
	default:
		return fmt.Errorf("forbidden: access denied")
	}
	return nil
}
