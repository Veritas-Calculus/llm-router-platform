import { gql } from '@apollo/client';

// ── User Dashboard Composite Query ─────────────────────────────────
// Combines personal usage, daily trends, provider breakdown, budget, and anomaly detection
// into a single query call for the User Dashboard page.

export const USER_DASHBOARD_QUERY = gql`
  query UserDashboard($days: Int, $projectId: ID, $channel: String) {
    me {
      id
      email
      name
      balance
      monthlyBudgetUsd
      monthlyTokenLimit
    }
    myUsageSummary(projectId: $projectId, channel: $channel) {
      totalRequests
      successRate
      totalTokens
      totalCost
    }
    myDailyUsage(days: $days, projectId: $projectId, channel: $channel) {
      date
      requests
      totalTokens
      totalCost
    }
    myUsageByProvider(projectId: $projectId, channel: $channel) {
      providerName
      requests
      tokens
      cost
    }
    myBudgetStatus {
      budget {
        id
        monthlyLimitUsd
        alertThreshold
        enforceHardLimit
        isActive
      }
      currentSpend
      remainingBudget
      percentUsed
      isOverBudget
    }
    myAnomalyDetection {
      hasAnomaly
      message
    }
  }
`;
