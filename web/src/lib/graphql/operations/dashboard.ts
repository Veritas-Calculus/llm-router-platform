import { gql } from '@apollo/client';

// ── Dashboard Operations ────────────────────────────────────────────

export const DASHBOARD_QUERY = gql`
  query Dashboard($days: Int) {
    dashboard {
      totalRequests
      totalTokens
      totalCost
      activeUsers
      activeProviders
      activeModels
      errorRate
      avgLatency
    }
    usageChart(days: $days) {
      date
      requests
      tokens
      cost
    }
    providerStats {
      provider
      requests
      tokens
      cost
      errorRate
      avgLatency
    }
    modelStats {
      model
      provider
      requests
      tokens
      cost
      avgLatency
    }
  }
`;
