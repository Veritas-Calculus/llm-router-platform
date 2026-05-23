package resolvers

// This file contains identity domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"fmt"

	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
)

// CreateIdentityProvider is the resolver for the createIdentityProvider field.
func (r *mutationResolver) CreateIdentityProvider(ctx context.Context, input model.CreateIdentityProviderInput) (*model.IdentityProvider, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.UserSvc.RequireOrgRole(ctx, uid, input.OrgID, "OWNER", "ADMIN"); err != nil {
		return nil, err
	}
	orgID, err := uuid.Parse(input.OrgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org id")
	}

	idp := &models.IdentityProvider{
		OrgID:    orgID,
		Type:     input.Type,
		Name:     input.Name,
		Domains:  input.Domains,
		IsActive: true,
	}

	if input.OidcClientID != nil {
		idp.OIDCClientID = *input.OidcClientID
	}
	if input.OidcClientSecret != nil {
		idp.OIDCClientSecret = *input.OidcClientSecret
	}
	if input.OidcIssuerURL != nil {
		idp.OIDCIssuerURL = *input.OidcIssuerURL
	}
	if input.SamlEntityID != nil {
		idp.SAMLEntityID = *input.SamlEntityID
	}
	if input.SamlSsoURL != nil {
		idp.SAMLSSOURL = *input.SamlSsoURL
	}
	if input.SamlIdpCert != nil {
		idp.SAMLIdPCert = *input.SamlIdpCert
	}
	if input.EnableJit != nil {
		idp.EnableJIT = *input.EnableJit
	} else {
		idp.EnableJIT = true
	}
	if input.DefaultRole != nil {
		idp.DefaultRole = *input.DefaultRole
	} else {
		idp.DefaultRole = "MEMBER"
	}
	if input.GroupRoleMapping != nil {
		idp.GroupRoleMapping = *input.GroupRoleMapping
	}

	if err := r.UserSvc.CreateIdentityProvider(ctx, idp); err != nil {
		return nil, err
	}
	return mapIdentityProviderToGraphQL(idp), nil
}

// UpdateIdentityProvider is the resolver for the updateIdentityProvider field.
func (r *mutationResolver) UpdateIdentityProvider(ctx context.Context, id string, input model.UpdateIdentityProviderInput) (*model.IdentityProvider, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	idpID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid idp id")
	}

	idp, err := r.UserSvc.GetIdentityProvider(ctx, idpID)
	if err != nil {
		return nil, err
	}

	if err := r.UserSvc.RequireOrgRole(ctx, uid, idp.OrgID.String(), "OWNER", "ADMIN"); err != nil {
		return nil, err
	}

	applyIdentityProviderUpdates(idp, input)

	if err := r.UserSvc.UpdateIdentityProvider(ctx, idp); err != nil {
		return nil, err
	}
	return mapIdentityProviderToGraphQL(idp), nil
}

// applyIdentityProviderUpdates applies non-nil fields from the input to the identity provider model.
func applyIdentityProviderUpdates(idp *models.IdentityProvider, input model.UpdateIdentityProviderInput) {
	if input.Name != nil {
		idp.Name = *input.Name
	}
	if input.IsActive != nil {
		idp.IsActive = *input.IsActive
	}
	if input.Domains != nil {
		idp.Domains = *input.Domains
	}
	if input.OidcClientID != nil {
		idp.OIDCClientID = *input.OidcClientID
	}
	if input.OidcClientSecret != nil {
		idp.OIDCClientSecret = *input.OidcClientSecret
	}
	if input.OidcIssuerURL != nil {
		idp.OIDCIssuerURL = *input.OidcIssuerURL
	}
	if input.SamlEntityID != nil {
		idp.SAMLEntityID = *input.SamlEntityID
	}
	if input.SamlSsoURL != nil {
		idp.SAMLSSOURL = *input.SamlSsoURL
	}
	if input.SamlIdpCert != nil {
		idp.SAMLIdPCert = *input.SamlIdpCert
	}
	if input.EnableJit != nil {
		idp.EnableJIT = *input.EnableJit
	}
	if input.DefaultRole != nil {
		idp.DefaultRole = *input.DefaultRole
	}
	if input.GroupRoleMapping != nil {
		idp.GroupRoleMapping = *input.GroupRoleMapping
	}
}

// DeleteIdentityProvider is the resolver for the deleteIdentityProvider field.
func (r *mutationResolver) DeleteIdentityProvider(ctx context.Context, id string) (bool, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return false, err
	}
	idpID, err := uuid.Parse(id)
	if err != nil {
		return false, fmt.Errorf("invalid idp id")
	}

	idp, err := r.UserSvc.GetIdentityProvider(ctx, idpID)
	if err != nil {
		return false, err
	}

	if err := r.UserSvc.RequireOrgRole(ctx, uid, idp.OrgID.String(), "OWNER", "ADMIN"); err != nil {
		return false, err
	}

	if err := r.UserSvc.DeleteIdentityProvider(ctx, idpID); err != nil {
		return false, err
	}
	return true, nil
}

// IdentityProviders is the resolver for the identityProviders field.
func (r *queryResolver) IdentityProviders(ctx context.Context, orgID string) ([]*model.IdentityProvider, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.UserSvc.RequireOrgRole(ctx, uid, orgID, "OWNER", "ADMIN", "MEMBER", "READONLY"); err != nil {
		return nil, err
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org id")
	}
	list, err := r.UserSvc.ListIdentityProvidersByOrg(ctx, orgUUID)
	if err != nil {
		return nil, err
	}

	out := make([]*model.IdentityProvider, len(list))
	for i := range list {
		out[i] = mapIdentityProviderToGraphQL(&list[i])
	}
	return out, nil
}
