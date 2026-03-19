import { gql } from '@apollo/client';

// ── API Key Operations ──────────────────────────────────────────────

export const MY_API_KEYS = gql`
  query MyApiKeys {
    myApiKeys { id name keyPrefix isActive lastUsedAt createdAt expiresAt }
  }
`;

export const CREATE_API_KEY = gql`
  mutation CreateApiKey($name: String!) {
    createApiKey(name: $name) {
      apiKey { id name keyPrefix isActive createdAt expiresAt }
      secretKey
    }
  }
`;

export const REVOKE_API_KEY = gql`
  mutation RevokeApiKey($id: ID!) {
    revokeApiKey(id: $id) { id isActive }
  }
`;

export const DELETE_API_KEY = gql`
  mutation DeleteApiKey($id: ID!) {
    deleteApiKey(id: $id)
  }
`;
