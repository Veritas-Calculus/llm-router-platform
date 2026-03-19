import { gql } from '@apollo/client';

// ── User Operations ─────────────────────────────────────────────────

export const USERS_QUERY = gql`
  query Users($q: String, $page: Int, $pageSize: Int) {
    users(q: $q, page: $page, pageSize: $pageSize) {
      data {
        id email name role isActive apiKeyCount lastLoginAt createdAt
      }
      total
    }
  }
`;

export const USER_DETAIL_QUERY = gql`
  query UserDetail($id: ID!, $days: Int) {
    user(id: $id) {
      id email name role isActive
      requestQuota usedQuota
      monthlyTokenLimit usedMonthlyTokens
      createdAt lastLoginAt
    }
    userUsage(id: $id, days: $days) {
      date requests tokens cost
    }
    userApiKeys(id: $id) {
      id name keyPrefix isActive lastUsedAt createdAt expiresAt
    }
  }
`;

export const TOGGLE_USER = gql`
  mutation ToggleUser($id: ID!) {
    toggleUser(id: $id) { id isActive }
  }
`;

export const UPDATE_USER_ROLE = gql`
  mutation UpdateUserRole($id: ID!, $role: String!) {
    updateUserRole(id: $id, role: $role) { id role }
  }
`;

export const UPDATE_USER_QUOTA = gql`
  mutation UpdateUserQuota($id: ID!, $input: QuotaInput!) {
    updateUserQuota(id: $id, input: $input) {
      id requestQuota monthlyTokenLimit
    }
  }
`;
