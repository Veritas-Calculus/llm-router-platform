import { gql } from '@apollo/client';

// ── Usage Operations ────────────────────────────────────────────────

export const MY_USAGE_SUMMARY = gql`
  query MyUsageSummary {
    myUsageSummary {
      totalRequests totalTokens totalCost
      periodStart periodEnd
    }
  }
`;

export const MY_DAILY_USAGE = gql`
  query MyDailyUsage($days: Int) {
    myDailyUsage(days: $days) { date requests tokens cost }
  }
`;

export const MY_USAGE_BY_PROVIDER = gql`
  query MyUsageByProvider {
    myUsageByProvider { provider requests tokens cost }
  }
`;

export const MY_RECENT_USAGE = gql`
  query MyRecentUsage($page: Int, $pageSize: Int) {
    myRecentUsage(page: $page, pageSize: $pageSize) {
      data {
        id model provider promptTokens completionTokens totalTokens cost
        statusCode latency createdAt
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
