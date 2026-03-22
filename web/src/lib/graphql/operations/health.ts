import { gql } from '@apollo/client';

// ── Health Operations ───────────────────────────────────────────────

export const HEALTH_OVERVIEW_QUERY = gql`
  query HealthOverview {
    healthApiKeys { id providerId providerName keyPrefix isActive isHealthy lastCheck responseTime successRate }
    healthProxies { id url type region isActive isHealthy responseTime lastCheck successRate }
    healthProviders { id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage }
    healthHistory { id targetType targetId status message createdAt }
  }
`;

export const SYSTEM_SLA_QUERY = gql`
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
      id targetType targetId isEnabled failureThreshold
      errorRateThreshold latencyThresholdMs budgetThreshold cooldownMinutes
      webhookUrl email
    }
  }
`;

export const CHECK_API_KEY_HEALTH = gql`
  mutation CheckApiKeyHealth($id: ID!) {
    checkApiKeyHealth(id: $id) { id providerId providerName isHealthy responseTime lastCheck }
  }
`;

export const CHECK_PROXY_HEALTH = gql`
  mutation CheckProxyHealth($id: ID!) {
    checkProxyHealth(id: $id) { id url isHealthy responseTime lastCheck }
  }
`;

export const CHECK_PROVIDER_HEALTH = gql`
  mutation CheckProviderHealth($id: ID!) {
    checkProviderHealth(id: $id) { id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage }
  }
`;

export const CHECK_ALL_PROVIDER_HEALTH = gql`
  mutation CheckAllProviderHealth {
    checkAllProviderHealth { id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage }
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
    updateAlertConfig(input: $input) {
      id targetType targetId isEnabled failureThreshold
      errorRateThreshold latencyThresholdMs budgetThreshold cooldownMinutes
      webhookUrl email
    }
  }
`;

export const SYSTEM_STATUS_QUERY = gql`
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

export const SYSTEM_LOAD_QUERY = gql`
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
