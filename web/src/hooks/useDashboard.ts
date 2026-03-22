import { useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { DASHBOARD_QUERY } from '@/lib/graphql/operations';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface UseDashboardProps {
  days?: number;
  projectId?: string;
  channel?: string;
}

/**
 * Custom hook encapsulating Dashboard data fetching via GraphQL.
 * Returns the same interface as before so DashboardPage needs minimal changes.
 */
export function useDashboard(props?: UseDashboardProps) {
  const { days = 30, projectId, channel } = props || {};
  const { data, loading } = useQuery<any>(DASHBOARD_QUERY, { 
    variables: { 
      days, 
      ...(projectId ? { projectId } : {}),
      ...(channel ? { channel } : {})
    } 
  });

  // Map GraphQL camelCase → REST-compatible shape for backward compat
  const stats = useMemo(() => {
    if (!data?.dashboard) return null;
    const d = data.dashboard;
    return {
      total_requests: d.totalRequests,
      total_tokens: d.totalTokens,
      total_cost: d.totalCost,
      success_rate: d.successRate,
      active_users: d.activeUsers,
      active_providers: d.activeProviders,
      active_proxies: d.activeProxies,
      requests_today: d.requestsToday,
      cost_today: d.costToday,
      tokens_today: d.tokensToday,
      error_count: d.errorCount,
      mcp_call_count: d.mcpCallCount,
      mcp_error_count: d.mcpErrorCount,
      api_keys: d.apiKeys ? { total: d.apiKeys.total, healthy: d.apiKeys.healthy } : { total: 0, healthy: 0 },
      proxies: d.proxies ? { total: d.proxies.total, healthy: d.proxies.healthy } : { total: 0, healthy: 0 },
    };
  }, [data]);

  const chartData = useMemo(() => data?.usageChart || [], [data]);
  const providerStats = useMemo(() =>
    (data?.providerStats || []).map((p: any) => ({
      provider_id: p.providerId,
      provider_name: p.providerName,
      requests: p.requests,
      tokens: p.tokens,
      success_rate: p.successRate,
      avg_latency_ms: p.avgLatencyMs,
      total_cost: p.totalCost,
    })),
  [data]);
  const modelStats = useMemo(() =>
    (data?.modelStats || []).map((m: any) => ({
      model_id: m.modelId,
      model_name: m.modelName,
      requests: m.requests,
      input_tokens: m.inputTokens,
      output_tokens: m.outputTokens,
      total_cost: m.totalCost,
    })),
  [data]);

  const formatCurrency = (value: number): string =>
    new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);

  const formatNumber = (value: number): string => {
    if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M';
    if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
    return new Intl.NumberFormat('en-US').format(value);
  };

  const formatTokens = (value: number): string => {
    if (value >= 1000000) return (value / 1000000).toFixed(2) + 'M';
    if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
    return new Intl.NumberFormat('en-US').format(value);
  };

  const COLORS = ['#007AFF', '#34C759', '#FF9500', '#AF52DE', '#FF3B30', '#5AC8FA'];

  return {
    stats,
    chartData,
    providerStats,
    modelStats,
    loading,
    formatCurrency,
    formatNumber,
    formatTokens,
    COLORS,
  };
}
