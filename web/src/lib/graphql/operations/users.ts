import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  ToggleUserMutation,
  ToggleUserMutationVariables,
  UpdateUserQuotaMutation,
  UpdateUserQuotaMutationVariables,
  UpdateUserRoleMutation,
  UpdateUserRoleMutationVariables,
  UserDetailQuery,
  UserDetailQueryVariables,
  UsersQuery,
  UsersQueryVariables,
} from '../generated/graphql';

// ── User Operations ─────────────────────────────────────────────────

export const USERS_QUERY: TypedDocumentNode<UsersQuery, UsersQueryVariables> = gql`
  query Users($q: String, $page: Int, $pageSize: Int) {
    users(q: $q, page: $page, pageSize: $pageSize) {
      data {
        id email name role isActive apiKeyCount lastLoginAt createdAt
      }
      total
    }
  }
`;

export const USER_DETAIL_QUERY: TypedDocumentNode<UserDetailQuery, UserDetailQueryVariables> = gql`
  query UserDetail($id: ID!, $days: Int) {
    user(id: $id) {
      id email name role isActive
      createdAt apiKeys
      monthlyTokenLimit monthlyBudgetUsd
      usageMonth {
        totalRequests totalTokens totalCost avgLatency successRate errorCount
      }
    }
    userUsage(id: $id, days: $days) {
      date requests totalTokens totalCost
    }
    userApiKeys(id: $id) {
      id name keyPrefix isActive lastUsedAt createdAt expiresAt
    }
  }
`;

export const TOGGLE_USER: TypedDocumentNode<ToggleUserMutation, ToggleUserMutationVariables> = gql`
  mutation ToggleUser($id: ID!) {
    toggleUser(id: $id) { id isActive }
  }
`;

export const UPDATE_USER_ROLE: TypedDocumentNode<UpdateUserRoleMutation, UpdateUserRoleMutationVariables> = gql`
  mutation UpdateUserRole($id: ID!, $role: String!) {
    updateUserRole(id: $id, role: $role) { id role }
  }
`;

export const UPDATE_USER_QUOTA: TypedDocumentNode<UpdateUserQuotaMutation, UpdateUserQuotaMutationVariables> = gql`
  mutation UpdateUserQuota($id: ID!, $input: QuotaInput!) {
    updateUserQuota(id: $id, input: $input) {
      id monthlyTokenLimit monthlyBudgetUsd
    }
  }
`;
