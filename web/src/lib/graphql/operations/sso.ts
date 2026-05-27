import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateIdentityProviderMutation,
  CreateIdentityProviderMutationVariables,
  DeleteIdentityProviderMutation,
  DeleteIdentityProviderMutationVariables,
  IdentityProvidersQuery,
  IdentityProvidersQueryVariables,
  UpdateIdentityProviderMutation,
  UpdateIdentityProviderMutationVariables,
} from '../generated/graphql';

export const IDENTITY_PROVIDERS_QUERY: TypedDocumentNode<IdentityProvidersQuery, IdentityProvidersQueryVariables> = gql`
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

export const CREATE_IDENTITY_PROVIDER: TypedDocumentNode<CreateIdentityProviderMutation, CreateIdentityProviderMutationVariables> = gql`
  mutation CreateIdentityProvider($input: CreateIdentityProviderInput!) {
    createIdentityProvider(input: $input) {
      id type name isActive domains createdAt
    }
  }
`;

export const UPDATE_IDENTITY_PROVIDER: TypedDocumentNode<UpdateIdentityProviderMutation, UpdateIdentityProviderMutationVariables> = gql`
  mutation UpdateIdentityProvider($id: ID!, $input: UpdateIdentityProviderInput!) {
    updateIdentityProvider(id: $id, input: $input) {
      id type name isActive domains
      oidcClientId oidcIssuerUrl
      samlEntityId samlSsoUrl
      enableJit defaultRole groupRoleMapping
    }
  }
`;

export const DELETE_IDENTITY_PROVIDER: TypedDocumentNode<DeleteIdentityProviderMutation, DeleteIdentityProviderMutationVariables> = gql`
  mutation DeleteIdentityProvider($id: ID!) {
    deleteIdentityProvider(id: $id)
  }
`;
