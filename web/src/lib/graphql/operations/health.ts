import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  HealthOverviewQuery,
  HealthOverviewQueryVariables,
  SystemSlaQuery,
  SystemSlaQueryVariables,
  AlertsQuery,
  AlertsQueryVariables,
  AlertConfigQuery,
  AlertConfigQueryVariables,
  CheckApiKeyHealthMutation,
  CheckApiKeyHealthMutationVariables,
  CheckProxyHealthMutation,
  CheckProxyHealthMutationVariables,
  CheckProviderHealthMutation,
  CheckProviderHealthMutationVariables,
  CheckAllProviderHealthMutation,
  CheckAllProviderHealthMutationVariables,
  AcknowledgeAlertMutation,
  AcknowledgeAlertMutationVariables,
  ResolveAlertMutation,
  ResolveAlertMutationVariables,
  UpdateAlertConfigMutation,
  UpdateAlertConfigMutationVariables,
  SystemStatusQuery,
  SystemStatusQueryVariables,
  SystemLoadQuery,
  SystemLoadQueryVariables,
} from '../generated/graphql';

// ── Health Operations ───────────────────────────────────────────────
//
// Each operation is exported as a TypedDocumentNode so call sites
// (useQuery/useMutation) infer TData/TVariables automatically — required
// by Apollo 4.2's modern signature, and removes the historical
// `useQuery(...)` pattern across ~80 components.

export const HEALTH_OVERVIEW_QUERY: TypedDocumentNode<HealthOverviewQuery, HealthOverviewQueryVariables> = gql`
  query HealthOverview {
    healthApiKeys { id providerId providerName keyPrefix isActive isHealthy lastCheck responseTime successRate }
    healthProxies { id url type region isActive isHealthy responseTime lastCheck successRate }
    healthProviders { id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage }
    healthHistory { id targetType targetId status message createdAt }
  }
`;

export const SYSTEM_SLA_QUERY: TypedDocumentNode<SystemSlaQuery, SystemSlaQueryVariables> = gql`
  query SystemSla($hours: Int) {
    systemSla(hours: $hours) {
      totalRequests
      failureRate
      avgLatencyMs
      p95LatencyMs
      p99LatencyMs
      activeProviders
      healthyProviders
    }
  }
`;

export const ALERTS_QUERY: TypedDocumentNode<AlertsQuery, AlertsQueryVariables> = gql`
  query Alerts($status: String) {
    alerts(status: $status) {
      data { id targetType targetId alertType message status resolvedAt acknowledgedAt createdAt }
      total
    }
  }
`;

export const ALERT_CONFIG_QUERY: TypedDocumentNode<AlertConfigQuery, AlertConfigQueryVariables> = gql`
  query AlertConfig($targetType: String!, $targetId: ID!) {
    alertConfig(targetType: $targetType, targetId: $targetId) {
      id targetType targetId isEnabled failureThreshold
      errorRateThreshold latencyThresholdMs budgetThreshold cooldownMinutes
      webhookUrl email
    }
  }
`;

export const CHECK_API_KEY_HEALTH: TypedDocumentNode<CheckApiKeyHealthMutation, CheckApiKeyHealthMutationVariables> = gql`
  mutation CheckApiKeyHealth($id: ID!) {
    checkApiKeyHealth(id: $id) { id providerId providerName isHealthy responseTime lastCheck }
  }
`;

export const CHECK_PROXY_HEALTH: TypedDocumentNode<CheckProxyHealthMutation, CheckProxyHealthMutationVariables> = gql`
  mutation CheckProxyHealth($id: ID!) {
    checkProxyHealth(id: $id) { id url isHealthy responseTime lastCheck }
  }
`;

export const CHECK_PROVIDER_HEALTH: TypedDocumentNode<CheckProviderHealthMutation, CheckProviderHealthMutationVariables> = gql`
  mutation CheckProviderHealth($id: ID!) {
    checkProviderHealth(id: $id) { id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage }
  }
`;

export const CHECK_ALL_PROVIDER_HEALTH: TypedDocumentNode<CheckAllProviderHealthMutation, CheckAllProviderHealthMutationVariables> = gql`
  mutation CheckAllProviderHealth {
    checkAllProviderHealth { id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage }
  }
`;

export const ACKNOWLEDGE_ALERT: TypedDocumentNode<AcknowledgeAlertMutation, AcknowledgeAlertMutationVariables> = gql`
  mutation AcknowledgeAlert($id: ID!) {
    acknowledgeAlert(id: $id) { id status acknowledgedAt }
  }
`;

export const RESOLVE_ALERT: TypedDocumentNode<ResolveAlertMutation, ResolveAlertMutationVariables> = gql`
  mutation ResolveAlert($id: ID!) {
    resolveAlert(id: $id) { id status resolvedAt }
  }
`;

export const UPDATE_ALERT_CONFIG: TypedDocumentNode<UpdateAlertConfigMutation, UpdateAlertConfigMutationVariables> = gql`
  mutation UpdateAlertConfig($input: AlertConfigInput!) {
    updateAlertConfig(input: $input) {
      id targetType targetId isEnabled failureThreshold
      errorRateThreshold latencyThresholdMs budgetThreshold cooldownMinutes
      webhookUrl email
    }
  }
`;

export const SYSTEM_STATUS_QUERY: TypedDocumentNode<SystemStatusQuery, SystemStatusQueryVariables> = gql`
  query SystemStatus {
    systemStatus {
      overallStatus
      service {
        version
        gitCommit
        buildTime
        uptime
        configMode
      }
      runtime {
        goroutines
        heapAllocMB
        heapSysMB
        gcPauseMs
        numGC
        cpuCores
      }
      dependencies {
        name
        status
        latencyMs
        version
        details
      }
    }
  }
`;

export const SYSTEM_LOAD_QUERY: TypedDocumentNode<SystemLoadQuery, SystemLoadQueryVariables> = gql`
  query SystemLoad {
    systemLoad {
      service {
        requestsInFlight
        requestsPerSecond
        avgLatencyMs
        p95LatencyMs
        errorRate
      }
      database {
        activeConnections
        maxConnections
        poolIdle
        poolInUse
        transactionsPerSecond
        cacheHitRate
        deadlocks
      }
      redis {
        connectedClients
        usedMemoryMB
        maxMemoryMB
        opsPerSecond
        hitRate
        keyCount
      }
    }
  }
`;
