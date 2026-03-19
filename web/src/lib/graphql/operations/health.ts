import { gql } from '@apollo/client';

// ── Health Operations ───────────────────────────────────────────────

export const HEALTH_OVERVIEW_QUERY = gql`
  query HealthOverview {
    healthApiKeys { apiKeyId providerId providerName isHealthy latency lastChecked error }
    healthProxies { proxyId host port isHealthy latency lastChecked error }
    healthProviders { providerId providerName isHealthy latency lastChecked activeKeyCount totalKeyCount }
    healthHistory { id targetType targetId status message createdAt }
  }
`;

export const ALERTS_QUERY = gql`
  query Alerts($status: String) {
    alerts(status: $status) {
      data { id targetType targetId alertType message status resolvedAt acknowledgedAt createdAt }
      total
    }
  }
`;

export const ALERT_CONFIG_QUERY = gql`
  query AlertConfig($targetType: String!, $targetId: ID!) {
    alertConfig(targetType: $targetType, targetId: $targetId) {
      id targetType targetId enabled threshold cooldownMinutes
    }
  }
`;

export const CHECK_API_KEY_HEALTH = gql`
  mutation CheckApiKeyHealth($id: ID!) {
    checkApiKeyHealth(id: $id) { apiKeyId isHealthy latency error }
  }
`;

export const CHECK_PROXY_HEALTH = gql`
  mutation CheckProxyHealth($id: ID!) {
    checkProxyHealth(id: $id) { proxyId isHealthy latency error }
  }
`;

export const CHECK_PROVIDER_HEALTH = gql`
  mutation CheckProviderHealth($id: ID!) {
    checkProviderHealth(id: $id) { providerId providerName isHealthy latency }
  }
`;

export const CHECK_ALL_PROVIDER_HEALTH = gql`
  mutation CheckAllProviderHealth {
    checkAllProviderHealth { providerId providerName isHealthy latency }
  }
`;

export const ACKNOWLEDGE_ALERT = gql`
  mutation AcknowledgeAlert($id: ID!) {
    acknowledgeAlert(id: $id) { id status acknowledgedAt }
  }
`;

export const RESOLVE_ALERT = gql`
  mutation ResolveAlert($id: ID!) {
    resolveAlert(id: $id) { id status resolvedAt }
  }
`;

export const UPDATE_ALERT_CONFIG = gql`
  mutation UpdateAlertConfig($input: AlertConfigInput!) {
    updateAlertConfig(input: $input) { id enabled threshold cooldownMinutes }
  }
`;
