import { gql } from '@apollo/client';

export const IDENTITY_PROVIDERS_QUERY = gql`
  query IdentityProviders($orgId: ID!) {
    identityProviders(orgId: $orgId) {
      id orgId type name isActive domains
      oidcClientId oidcIssuerUrl
      samlEntityId samlSsoUrl
      enableJit defaultRole groupRoleMapping
      createdAt updatedAt
    }
  }
`;

export const CREATE_IDENTITY_PROVIDER = gql`
  mutation CreateIdentityProvider($input: CreateIdentityProviderInput!) {
    createIdentityProvider(input: $input) {
      id type name isActive domains createdAt
    }
  }
`;

export const UPDATE_IDENTITY_PROVIDER = gql`
  mutation UpdateIdentityProvider($id: ID!, $input: UpdateIdentityProviderInput!) {
    updateIdentityProvider(id: $id, input: $input) {
      id type name isActive domains
      oidcClientId oidcIssuerUrl
      samlEntityId samlSsoUrl
      enableJit defaultRole groupRoleMapping
    }
  }
`;

export const DELETE_IDENTITY_PROVIDER = gql`
  mutation DeleteIdentityProvider($id: ID!) {
    deleteIdentityProvider(id: $id)
  }
`;
