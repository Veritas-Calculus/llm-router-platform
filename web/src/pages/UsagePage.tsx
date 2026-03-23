import { useState, useMemo, useEffect } from 'react';
import { ChartBarIcon, TableCellsIcon } from '@heroicons/react/24/outline';
import { motion } from 'framer-motion';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { useQuery } from '@apollo/client/react';
import { MY_USAGE_SUMMARY, MY_DAILY_USAGE, MY_RECENT_USAGE } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

function UsagePage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const pageSize = 20;
  
  const [channelFilter, setChannelFilter] = useState('');
  const [debouncedChannel, setDebouncedChannel] = useState('');

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedChannel(channelFilter);
    }, 500);
    return () => clearTimeout(timer);
  }, [channelFilter]);

  const queryVars = {
    channel: debouncedChannel || undefined
  };

  const { data: summaryData, loading: sumLoading } = useQuery<any>(MY_USAGE_SUMMARY, { variables: queryVars });
  const { data: dailyData, loading: dailyLoading } = useQuery<any>(MY_DAILY_USAGE, { variables: { days: 30, ...queryVars } });
  const { data: recentData, loading: recentLoading } = useQuery<any>(MY_RECENT_USAGE, { variables: { page, pageSize } });
  const loading = sumLoading || dailyLoading || recentLoading;

  const monthlyUsage = useMemo(() => {
    const s = summaryData?.myUsageSummary;
    if (!s) return null;
    return { total_requests: s.totalRequests, total_tokens: s.totalTokens, total_cost: s.totalCost, success_rate: s.successRate };
  }, [summaryData]);

  const dailyStats = useMemo(() =>
    (dailyData?.myDailyUsage || []).map((d: any) => ({ date: d.date, requests: d.requests, tokens: d.totalTokens, cost: d.totalCost })),
  [dailyData]);

  const records = useMemo(() =>
    (recentData?.myRecentUsage?.data || []).map((r: any) => ({
      id: r.id, model_name: r.modelName, input_tokens: r.inputTokens, output_tokens: r.outputTokens,
      cost: r.cost, latency_ms: r.latencyMs, is_success: r.isSuccess, created_at: r.createdAt,
    })),
  [recentData]);
  const total = recentData?.myRecentUsage?.total || 0;

  const formatCurrency = (value: number): string => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 4,
    }).format(value);
  };

  const formatNumber = (value: number): string => {
    return new Intl.NumberFormat('en-US').format(value);
  };

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Usage</h1>
          <p className="text-apple-gray-500 mt-1">Monitor your API usage and costs</p>
        </div>
        <div className="flex items-center gap-4">
          <div className="relative">
            <input
              type="text"
              placeholder="Filter by channel..."
              value={channelFilter}
              onChange={(e) => setChannelFilter(e.target.value)}
              className="pl-3 pr-4 py-2 text-sm border border-apple-gray-200 rounded-apple-lg focus:outline-none focus:ring-2 focus:ring-apple-blue/50 focus:border-apple-blue transition-shadow bg-white w-48"
            />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="card"
        >
          <p className="text-sm text-apple-gray-500 mb-1">Monthly Requests</p>
          <p className="text-2xl font-semibold text-apple-gray-900">
            {formatNumber(monthlyUsage?.total_requests || 0)}
          </p>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="card"
        >
          <p className="text-sm text-apple-gray-500 mb-1">Total Tokens</p>
          <p className="text-2xl font-semibold text-apple-gray-900">
            {formatNumber(monthlyUsage?.total_tokens || 0)}
          </p>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="card"
        >
          <p className="text-sm text-apple-gray-500 mb-1">Total Cost</p>
          <p className="text-2xl font-semibold text-apple-gray-900">
            {formatCurrency(monthlyUsage?.total_cost || 0)}
          </p>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
          className="card"
        >
          <p className="text-sm text-apple-gray-500 mb-1">Success Rate</p>
          <p className="text-2xl font-semibold text-apple-gray-900">
            {(monthlyUsage?.success_rate || 0).toFixed(1)}%
          </p>
        </motion.div>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.4 }}
        className="card"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Daily Usage</h2>
        {dailyStats.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16">
            <div className="w-14 h-14 bg-blue-50 rounded-2xl flex items-center justify-center mb-3">
              <ChartBarIcon className="w-7 h-7 text-apple-blue" />
            </div>
            <p className="text-apple-gray-900 font-medium">No usage data yet</p>
            <p className="text-apple-gray-500 text-sm mt-1">Usage will appear here once you start making API requests.</p>
          </div>
        ) : (
          <div className="h-64" style={{ minHeight: '256px' }}>
            <ResponsiveContainer width="100%" height="100%" minHeight={256}>
              <LineChart data={dailyStats}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis
                  dataKey="date"
                  stroke="#8E8E93"
                  fontSize={12}
                  tickFormatter={(value) =>
                    new Date(value).toLocaleDateString('en-US', {
                      month: 'short',
                      day: 'numeric',
                    })
                  }
                />
                <YAxis stroke="#8E8E93" fontSize={12} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#fff',
                    border: '1px solid #E8E8ED',
                    borderRadius: '12px',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="requests"
                  stroke="#007AFF"
                  strokeWidth={2}
                  dot={false}
                  name={t('usage.requests')}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.5 }}
        className="card"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Recent Requests</h2>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-apple-gray-200">
            <thead>
              <tr>
                <th className="table-header">Model</th>
                <th className="table-header">Input Tokens</th>
                <th className="table-header">Output Tokens</th>
                <th className="table-header">Cost</th>
                <th className="table-header">Latency</th>
                <th className="table-header">Status</th>
                <th className="table-header">Time</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {records.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-16 text-center">
                    <div className="flex flex-col items-center">
                      <div className="w-12 h-12 bg-apple-gray-50 rounded-2xl flex items-center justify-center mb-3">
                        <TableCellsIcon className="w-6 h-6 text-apple-gray-400" />
                      </div>
                      <p className="text-apple-gray-500 font-medium">No requests recorded</p>
                      <p className="text-apple-gray-400 text-sm mt-1">Recent API requests will show up here.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                records.map((record: any) => (
                  <tr key={record.id} className="hover:bg-apple-gray-50">
                    <td className="table-cell font-medium">{record.model_name}</td>
                    <td className="table-cell">{formatNumber(record.input_tokens)}</td>
                    <td className="table-cell">{formatNumber(record.output_tokens)}</td>
                    <td className="table-cell">{formatCurrency(record.cost)}</td>
                    <td className="table-cell">{record.latency_ms}ms</td>
                    <td className="table-cell">
                      <span
                        className={
                          record.is_success ? 'badge-success' : 'badge-error'
                        }
                      >
                        {record.is_success ? 'Success' : 'Failed'}
                      </span>
                    </td>
                    <td className="table-cell text-apple-gray-500">
                      {formatDate(record.created_at)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {totalPages > 1 && (
          <div className="flex items-center justify-between mt-4 pt-4 border-t border-apple-gray-200">
            <p className="text-sm text-apple-gray-500">
              Showing {(page - 1) * pageSize + 1} to{' '}
              {Math.min(page * pageSize, total)} of {total} results
            </p>
            <div className="flex gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="btn-secondary px-3 py-1.5 text-sm"
              >
                Previous
              </button>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="btn-secondary px-3 py-1.5 text-sm"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </motion.div>
    </div>
  );
}

export default UsagePage;
