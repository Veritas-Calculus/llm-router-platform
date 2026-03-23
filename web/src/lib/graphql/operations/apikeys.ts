import { gql } from '@apollo/client';

// ── API Key Operations ──────────────────────────────────────────────

export const MY_ORGANIZATIONS = gql`
  query MyOrganizations {
    myOrganizations { id name billingLimit createdAt }
  }
`;

export const MY_PROJECTS = gql`
  query MyProjects($orgId: ID!) {
    myProjects(orgId: $orgId) { id orgId name description quotaLimit whiteListedIps createdAt }
  }
`;

export const MY_API_KEYS = gql`
  query MyApiKeys($projectId: ID!) {
    myApiKeys(projectId: $projectId) {
      id projectId channel name keyPrefix isActive scopes rateLimit tokenLimit dailyLimit lastUsedAt createdAt expiresAt
    }
  }
`;

export const CREATE_API_KEY = gql`
  mutation CreateApiKey($projectId: ID!, $name: String!, $scopes: String, $rateLimit: Int, $tokenLimit: Int) {
    createApiKey(projectId: $projectId, name: $name, scopes: $scopes, rateLimit: $rateLimit, tokenLimit: $tokenLimit) {
      id projectId channel name key keyPrefix isActive scopes rateLimit tokenLimit dailyLimit createdAt expiresAt
    }
  }
`;

export const UPDATE_API_KEY = gql`
  mutation UpdateApiKey($id: ID!, $name: String, $scopes: String, $rateLimit: Int, $tokenLimit: Int, $isActive: Boolean) {
    updateApiKey(id: $id, name: $name, scopes: $scopes, rateLimit: $rateLimit, tokenLimit: $tokenLimit, isActive: $isActive) {
      id projectId channel name keyPrefix isActive scopes rateLimit tokenLimit dailyLimit createdAt expiresAt
    }
  }
`;

export const REVOKE_API_KEY = gql`
  mutation RevokeApiKey($projectId: ID!, $id: ID!) {
    revokeApiKey(projectId: $projectId, id: $id) { id isActive }
  }
`;

export const DELETE_API_KEY = gql`
  mutation DeleteApiKey($projectId: ID!, $id: ID!) {
    deleteApiKey(projectId: $projectId, id: $id)
  }
`;

export const UPDATE_PROJECT = gql`
  mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
    updateProject(id: $id, input: $input) {
      id orgId name description quotaLimit whiteListedIps createdAt
    }
  }
`;

export const API_KEY_RATE_LIMIT_STATUS = gql`
  query ApiKeyRateLimitStatus($keyId: ID!) {
    apiKeyRateLimitStatus(keyId: $keyId) {
      keyId rpmCurrent rpmLimit rpmExceeded
      tpmCurrent tpmLimit tpmExceeded
      dailyCurrent dailyLimit dailyExceeded
      status
    }
  }
`;
