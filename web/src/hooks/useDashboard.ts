import { useEffect, useState, useCallback, useMemo } from 'react';
import {
  dashboardApi,
  OverviewStats,
  UsageChartData,
  ProviderStats,
  ModelStats,
} from '@/lib/api';

/**
 * Custom hook encapsulating Dashboard data fetching, state, and formatting utilities.
 */
export function useDashboard() {
  const [stats, setStats] = useState<OverviewStats | null>(null);
  const [chartData, setChartData] = useState<UsageChartData[]>([]);
  const [providerStats, setProviderStats] = useState<ProviderStats[]>([]);
  const [modelStats, setModelStats] = useState<ModelStats[]>([]);
  const [loading, setLoading] = useState(true);

  const loadDashboardData = useCallback(async () => {
    try {
      const [overviewRes, chartRes, providerRes, modelRes] = await Promise.all([
        dashboardApi.getOverview(),
        dashboardApi.getUsageChart(),
        dashboardApi.getProviderStats(),
        dashboardApi.getModelStats(),
      ]);
      setStats(overviewRes);
      setChartData(chartRes?.data || []);
      setProviderStats(providerRes?.data || []);
      setModelStats(modelRes?.data || []);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadDashboardData(); }, [loadDashboardData]);

  // ─── Formatting utilities ──────────────────────────────────────

  const formatCurrency = useCallback((value: number): string => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(value);
  }, []);

  const formatNumber = useCallback((value: number): string => {
    if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M';
    if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
    return new Intl.NumberFormat('en-US').format(value);
  }, []);

  const formatTokens = useCallback((value: number): string => {
    if (value >= 1000000) return (value / 1000000).toFixed(2) + 'M';
    if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
    return new Intl.NumberFormat('en-US').format(value);
  }, []);

  const COLORS = useMemo(() => ['#007AFF', '#34C759', '#FF9500', '#AF52DE', '#FF3B30', '#5AC8FA'], []);

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
    refresh: loadDashboardData,
  };
}
