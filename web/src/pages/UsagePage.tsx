import { useEffect, useState } from 'react';
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
import { usageApi, DailyStats, UsageRecord, MonthlyUsage } from '@/lib/api';

function UsagePage() {
  const [dailyStats, setDailyStats] = useState<DailyStats[]>([]);
  const [records, setRecords] = useState<UsageRecord[]>([]);
  const [monthlyUsage, setMonthlyUsage] = useState<MonthlyUsage | null>(null);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const pageSize = 20;

  useEffect(() => {
    loadUsageData();
  }, [page]);

  const loadUsageData = async () => {
    try {
      const [dailyRes, recordsRes, monthlyRes] = await Promise.all([
        usageApi.getDailyStats(30),
        usageApi.getRecords(page, pageSize),
        usageApi.getMonthlyUsage(),
      ]);
      setDailyStats(dailyRes?.data || []);
      setRecords(recordsRes?.data || []);
      setTotal(recordsRes?.total || 0);
      setMonthlyUsage(monthlyRes);
    } catch (error) {
      console.error('Failed to load usage data:', error);
      // Set default empty values on error
      setDailyStats([]);
      setRecords([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

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
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Usage</h1>
        <p className="text-apple-gray-500 mt-1">Monitor your API usage and costs</p>
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
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
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
                name="Requests"
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
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
              {records.map((record) => (
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
              ))}
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
