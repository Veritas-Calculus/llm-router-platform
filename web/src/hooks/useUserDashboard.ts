import { useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { USER_DASHBOARD_QUERY } from '@/lib/graphql/operations/userDashboard';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface UseUserDashboardProps {
  days?: number;
  projectId?: string;
  channel?: string;
}

/**
 * Hook for the User Dashboard — fetches personal usage, budget, anomaly detection.
 */
export function useUserDashboard(props?: UseUserDashboardProps) {
  const { days = 7, projectId, channel } = props || {};
  const { data, loading, error } = useQuery<any>(USER_DASHBOARD_QUERY, {
    variables: {
      days,
      ...(projectId ? { projectId } : {}),
      ...(channel ? { channel } : {}),
    },
  });

  const me = data?.me || null;

  const summary = useMemo(() => {
    if (!data?.myUsageSummary) return null;
    const s = data.myUsageSummary;
    return {
      totalRequests: s.totalRequests,
      totalTokens: s.totalTokens,
      totalCost: s.totalCost,
      successRate: s.successRate,
    };
  }, [data]);

  const chartData = useMemo(() => data?.myDailyUsage || [], [data]);

  const providerUsage = useMemo(() =>
    (data?.myUsageByProvider || []).map((p: any) => ({
      providerName: p.providerName,
      requests: p.requests,
      tokens: p.tokens,
      cost: p.cost,
    })),
  [data]);

  const budgetStatus = useMemo(() => {
    if (!data?.myBudgetStatus) return null;
    const b = data.myBudgetStatus;
    return {
      currentSpend: b.currentSpend,
      percentUsed: b.percentUsed,
      isOverBudget: b.isOverBudget,
      budget: b.budget,
    };
  }, [data]);

  const anomaly = useMemo(() => data?.myAnomalyDetection || null, [data]);

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
    me,
    summary,
    chartData,
    providerUsage,
    budgetStatus,
    anomaly,
    loading,
    error,
    formatCurrency,
    formatNumber,
    formatTokens,
    COLORS,
  };
}
