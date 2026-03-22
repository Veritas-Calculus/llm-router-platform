import { gql } from '@apollo/client';

// ── Usage Operations ────────────────────────────────────────────────

export const MY_USAGE_SUMMARY = gql`
  query MyUsageSummary($projectId: ID, $channel: String) {
    myUsageSummary(projectId: $projectId, channel: $channel) {
      totalRequests totalTokens totalCost successRate
    }
  }
`;

export const MY_DAILY_USAGE = gql`
  query MyDailyUsage($days: Int, $projectId: ID, $channel: String) {
    myDailyUsage(days: $days, projectId: $projectId, channel: $channel) { date requests totalTokens totalCost }
  }
`;

export const MY_USAGE_BY_PROVIDER = gql`
  query MyUsageByProvider($projectId: ID, $channel: String) {
    myUsageByProvider(projectId: $projectId, channel: $channel) { providerName requests tokens totalCost }
  }
`;

export const MY_RECENT_USAGE = gql`
  query MyRecentUsage($page: Int, $pageSize: Int) {
    myRecentUsage(page: $page, pageSize: $pageSize) {
      data {
        id modelName inputTokens outputTokens cost
        latencyMs isSuccess createdAt
      }
      total
    }
  }
`;

export const EXPORT_USAGE_CSV = gql`
  mutation ExportUsageCsv {
    exportUsageCsv
  }
`;
