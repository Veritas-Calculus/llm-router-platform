import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  BatchCreateProxiesMutation,
  BatchCreateProxiesMutationVariables,
  CreateProxyMutation,
  CreateProxyMutationVariables,
  CreateProxyPoolMutation,
  CreateProxyPoolMutationVariables,
  DeleteProxyMutation,
  DeleteProxyMutationVariables,
  DeleteProxyPoolMutation,
  DeleteProxyPoolMutationVariables,
  ProxiesQuery,
  ProxiesQueryVariables,
  ProxyPoolsQuery,
  ProxyPoolsQueryVariables,
  ProxyTopologyQuery,
  ProxyTopologyQueryVariables,
  TestAllProxiesMutation,
  TestAllProxiesMutationVariables,
  TestProxyMutation,
  TestProxyMutationVariables,
  ToggleProxyStatusMutation,
  ToggleProxyStatusMutationVariables,
  UpdateProxyMutation,
  UpdateProxyMutationVariables,
  UpdateProxyPoolMutation,
  UpdateProxyPoolMutationVariables,
} from '../generated/graphql';

// ── Proxy Operations ────────────────────────────────────────────────

export const PROXIES_QUERY: TypedDocumentNode<ProxiesQuery, ProxiesQueryVariables> = gql`
  query Proxies {
    proxies {
      id poolId poolName url type region isActive weight successCount failureCount avgLatency lastChecked createdAt hasAuth upstreamProxyId
    }
  }
`;

export const PROXY_POOLS_QUERY: TypedDocumentNode<ProxyPoolsQuery, ProxyPoolsQueryVariables> = gql`
  query ProxyPools {
    proxyPools {
      id name description isActive strategy proxyCount activeProxyCount createdAt
    }
  }
`;

export const PROXY_TOPOLOGY_QUERY: TypedDocumentNode<ProxyTopologyQuery, ProxyTopologyQueryVariables> = gql`
  query ProxyTopology {
    proxyTopology {
      directAccounts
      proxiedAccounts
      providers {
        provider { id name baseUrl isActive useProxy defaultProxyId requiresApiKey }
        models
        accounts {
          label
          bindingSource
          apiKey {
            id providerId proxyId proxyPoolId alias keyPrefix isActive priority weight rateLimit usageCount lastUsedAt createdAt
          }
          proxyPool { id name description isActive strategy proxyCount activeProxyCount createdAt }
          candidateProxies {
            id poolId poolName url type region isActive weight successCount failureCount avgLatency lastChecked createdAt hasAuth upstreamProxyId
          }
          route {
            id type label status detail
          }
        }
      }
      proxyPools {
        pool { id name description isActive strategy proxyCount activeProxyCount createdAt }
        proxies {
          id poolId poolName url type region isActive weight successCount failureCount avgLatency lastChecked createdAt hasAuth upstreamProxyId
        }
      }
    }
  }
`;

export const CREATE_PROXY: TypedDocumentNode<CreateProxyMutation, CreateProxyMutationVariables> = gql`
  mutation CreateProxy($input: ProxyInput!) {
    createProxy(input: $input) {
      id poolId poolName url type region isActive weight successCount failureCount avgLatency lastChecked createdAt hasAuth upstreamProxyId
    }
  }
`;

export const CREATE_PROXY_POOL: TypedDocumentNode<CreateProxyPoolMutation, CreateProxyPoolMutationVariables> = gql`
  mutation CreateProxyPool($input: ProxyPoolInput!) {
    createProxyPool(input: $input) {
      id name description isActive strategy proxyCount activeProxyCount createdAt
    }
  }
`;

export const UPDATE_PROXY_POOL: TypedDocumentNode<UpdateProxyPoolMutation, UpdateProxyPoolMutationVariables> = gql`
  mutation UpdateProxyPool($id: ID!, $input: ProxyPoolInput!) {
    updateProxyPool(id: $id, input: $input) {
      id name description isActive strategy proxyCount activeProxyCount createdAt
    }
  }
`;

export const DELETE_PROXY_POOL: TypedDocumentNode<DeleteProxyPoolMutation, DeleteProxyPoolMutationVariables> = gql`
  mutation DeleteProxyPool($id: ID!) {
    deleteProxyPool(id: $id)
  }
`;

export const BATCH_CREATE_PROXIES: TypedDocumentNode<BatchCreateProxiesMutation, BatchCreateProxiesMutationVariables> = gql`
  mutation BatchCreateProxies($input: BatchProxyInput!) {
    batchCreateProxies(input: $input) {
      success failed errors
      proxies { id poolId poolName url type region isActive weight successCount failureCount avgLatency lastChecked createdAt hasAuth upstreamProxyId }
    }
  }
`;

export const UPDATE_PROXY: TypedDocumentNode<UpdateProxyMutation, UpdateProxyMutationVariables> = gql`
  mutation UpdateProxy($id: ID!, $input: ProxyInput!) {
    updateProxy(id: $id, input: $input) {
      id poolId poolName url type region isActive weight successCount failureCount avgLatency lastChecked createdAt hasAuth upstreamProxyId
    }
  }
`;

export const DELETE_PROXY: TypedDocumentNode<DeleteProxyMutation, DeleteProxyMutationVariables> = gql`
  mutation DeleteProxy($id: ID!) {
    deleteProxy(id: $id)
  }
`;

export const TOGGLE_PROXY_STATUS: TypedDocumentNode<ToggleProxyStatusMutation, ToggleProxyStatusMutationVariables> = gql`
  mutation ToggleProxyStatus($id: ID!) {
    toggleProxyStatus(id: $id) { id isActive }
  }
`;

export const TEST_PROXY: TypedDocumentNode<TestProxyMutation, TestProxyMutationVariables> = gql`
  mutation TestProxy($id: ID!) {
    testProxy(id: $id) { id url isHealthy latencyMs error }
  }
`;

export const TEST_ALL_PROXIES: TypedDocumentNode<TestAllProxiesMutation, TestAllProxiesMutationVariables> = gql`
  mutation TestAllProxies {
    testAllProxies { id url isHealthy latencyMs error }
  }
`;
