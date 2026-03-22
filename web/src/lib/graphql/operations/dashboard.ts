import { gql } from '@apollo/client';

// ── Dashboard Operations ────────────────────────────────────────────

export const DASHBOARD_QUERY = gql`
  query Dashboard($days: Int, $projectId: ID, $channel: String) {
    dashboard(projectId: $projectId, channel: $channel) {
      totalRequests
      successRate
      totalTokens
      totalCost
      activeUsers
      activeProviders
      activeProxies
      requestsToday
      costToday
      tokensToday
      errorCount
      mcpCallCount
      mcpErrorCount
      apiKeys {
        total
        healthy
      }
      proxies {
        total
        healthy
      }
    }
    usageChart(days: $days, projectId: $projectId, channel: $channel) {
      date
      requests
      tokens
      cost
    }
    providerStats(projectId: $projectId, channel: $channel) {
      providerName
      requests
      tokens
      totalCost
      successRate
      avgLatencyMs
    }
    modelStats(projectId: $projectId, channel: $channel) {
      modelName
      requests
      inputTokens
      outputTokens
      totalCost
    }
  }
`;
