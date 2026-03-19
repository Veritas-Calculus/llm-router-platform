import { gql } from '@apollo/client';

// ── Proxy Operations ────────────────────────────────────────────────

export const PROXIES_QUERY = gql`
  query Proxies {
    proxies {
      id protocol host port username isActive latency lastChecked createdAt
    }
  }
`;

export const CREATE_PROXY = gql`
  mutation CreateProxy($input: ProxyInput!) {
    createProxy(input: $input) {
      id protocol host port username isActive createdAt
    }
  }
`;

export const BATCH_CREATE_PROXIES = gql`
  mutation BatchCreateProxies($input: BatchProxyInput!) {
    batchCreateProxies(input: $input) {
      total success failed errors
    }
  }
`;

export const UPDATE_PROXY = gql`
  mutation UpdateProxy($id: ID!, $input: ProxyInput!) {
    updateProxy(id: $id, input: $input) {
      id protocol host port username isActive
    }
  }
`;

export const DELETE_PROXY = gql`
  mutation DeleteProxy($id: ID!) {
    deleteProxy(id: $id)
  }
`;

export const TOGGLE_PROXY_STATUS = gql`
  mutation ToggleProxyStatus($id: ID!) {
    toggleProxyStatus(id: $id) { id isActive }
  }
`;

export const TEST_PROXY = gql`
  mutation TestProxy($id: ID!) {
    testProxy(id: $id) { proxyId isHealthy latency error }
  }
`;

export const TEST_ALL_PROXIES = gql`
  mutation TestAllProxies {
    testAllProxies { proxyId isHealthy latency error }
  }
`;
