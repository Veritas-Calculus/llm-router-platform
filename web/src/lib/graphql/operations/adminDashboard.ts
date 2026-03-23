import { gql } from '@apollo/client';

// -- Admin Dashboard & Analytics Operations --

export const ADMIN_DASHBOARD_QUERY = gql`
  query AdminDashboard {
    adminDashboard {
      totalUsers
      activeUsersToday
      activeUsersMonth
      totalRevenue
      revenueThisMonth
      totalRequests
      requestsToday
      totalTokens
      tokensToday
      totalCost
      costToday
      successRate
      errorCount
      avgLatencyMs
      activeProviders
      totalProviders
      activeProxies
      totalProxies
      apiKeysTotal
      apiKeysHealthy
      mcpCallCount
      mcpErrorCount
    }
    usageChart(days: 7) {
      date
      requests
      tokens
      cost
    }
    providerStats {
      providerName
      requests
      tokens
      totalCost
      successRate
      avgLatencyMs
    }
    modelStats {
      modelName
      requests
      inputTokens
      outputTokens
      totalCost
    }
  }
`;

export const ADMIN_USAGE_BY_USER_QUERY = gql`
  query AdminUsageByUser($days: Int) {
    adminUsageByUser(days: $days) {
      userId
      userName
      email
      requests
      tokens
      cost
    }
  }
`;

export const ADMIN_REVENUE_CHART_QUERY = gql`
  query AdminRevenueChart($days: Int) {
    adminRevenueChart(days: $days) {
      date
      revenue
      transactions
    }
  }
`;

export const ADMIN_USER_GROWTH_QUERY = gql`
  query AdminUserGrowth($days: Int) {
    adminUserGrowth(days: $days) {
      date
      newUsers
      totalUsers
    }
  }
`;
