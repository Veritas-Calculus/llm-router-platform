import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  ExportUsageCsvMutation,
  ExportUsageCsvMutationVariables,
  MyDailyUsageQuery,
  MyDailyUsageQueryVariables,
  MyRecentUsageQuery,
  MyRecentUsageQueryVariables,
  MyUsageByProviderQuery,
  MyUsageByProviderQueryVariables,
  MyUsageSummaryQuery,
  MyUsageSummaryQueryVariables,
} from '../generated/graphql';

// ── Usage Operations ────────────────────────────────────────────────

export const MY_USAGE_SUMMARY: TypedDocumentNode<MyUsageSummaryQuery, MyUsageSummaryQueryVariables> = gql`
  query MyUsageSummary($projectId: ID, $channel: String) {
    myUsageSummary(projectId: $projectId, channel: $channel) {
      totalRequests totalTokens totalCost successRate
    }
  }
`;

export const MY_DAILY_USAGE: TypedDocumentNode<MyDailyUsageQuery, MyDailyUsageQueryVariables> = gql`
  query MyDailyUsage($days: Int, $projectId: ID, $channel: String) {
    myDailyUsage(days: $days, projectId: $projectId, channel: $channel) { date requests totalTokens totalCost }
  }
`;

export const MY_USAGE_BY_PROVIDER: TypedDocumentNode<MyUsageByProviderQuery, MyUsageByProviderQueryVariables> = gql`
  query MyUsageByProvider($projectId: ID, $channel: String) {
    myUsageByProvider(projectId: $projectId, channel: $channel) { providerName requests tokens cost }
  }
`;

export const MY_RECENT_USAGE: TypedDocumentNode<MyRecentUsageQuery, MyRecentUsageQueryVariables> = gql`
  query MyRecentUsage($page: Int, $pageSize: Int, $channel: String) {
    myRecentUsage(page: $page, pageSize: $pageSize, channel: $channel) {
      data {
        id modelName inputTokens outputTokens cost
        latencyMs isSuccess createdAt
      }
      total
    }
  }
`;

export const EXPORT_USAGE_CSV: TypedDocumentNode<ExportUsageCsvMutation, ExportUsageCsvMutationVariables> = gql`
  mutation ExportUsageCsv {
    exportUsageCsv
  }
`;
