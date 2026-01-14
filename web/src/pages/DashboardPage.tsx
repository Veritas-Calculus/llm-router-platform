import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  ArrowTrendingUpIcon,
  CurrencyDollarIcon,
  BoltIcon,
  CheckCircleIcon,
} from '@heroicons/react/24/outline';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar,
} from 'recharts';
import {
  dashboardApi,
  OverviewStats,
  UsageChartData,
  ProviderStats,
  ModelStats,
} from '@/lib/api';

interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon: React.ElementType;
  color: 'blue' | 'green' | 'orange' | 'purple';
}

function StatCard({ title, value, subtitle, icon: Icon, color }: StatCardProps) {
  const colorClasses = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    orange: 'bg-orange-50 text-orange-600',
    purple: 'bg-purple-50 text-purple-600',
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      className="card"
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-apple-gray-500 mb-1">{title}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{value}</p>
          {subtitle && <p className="text-sm text-apple-gray-400 mt-1">{subtitle}</p>}
        </div>
        <div className={`p-3 rounded-apple ${colorClasses[color]}`}>
          <Icon className="w-6 h-6" />
        </div>
      </div>
    </motion.div>
  );
}

function DashboardPage() {
  const [stats, setStats] = useState<OverviewStats | null>(null);
  const [chartData, setChartData] = useState<UsageChartData[]>([]);
  const [providerStats, setProviderStats] = useState<ProviderStats[]>([]);
  const [modelStats, setModelStats] = useState<ModelStats[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadDashboardData();
  }, []);

  const loadDashboardData = async () => {
    try {
      const [overviewRes, chartRes, providerRes, modelRes] = await Promise.all([
        dashboardApi.getOverview(),
        dashboardApi.getUsageChart(),
        dashboardApi.getProviderStats(),
        dashboardApi.getModelStats(),
      ]);
      setStats(overviewRes);
      setChartData(chartRes.data);
      setProviderStats(providerRes.data);
      setModelStats(modelRes.data);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  };

  const formatCurrency = (value: number): string => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(value);
  };

  const formatNumber = (value: number): string => {
    return new Intl.NumberFormat('en-US').format(value);
  };

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
        <h1 className="text-2xl font-semibold text-apple-gray-900">Dashboard</h1>
        <p className="text-apple-gray-500 mt-1">Overview of your LLM usage and performance</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Total Requests"
          value={formatNumber(stats?.total_requests || 0)}
          subtitle="This month"
          icon={ArrowTrendingUpIcon}
          color="blue"
        />
        <StatCard
          title="Success Rate"
          value={`${(stats?.success_rate || 0).toFixed(1)}%`}
          subtitle="Request success"
          icon={CheckCircleIcon}
          color="green"
        />
        <StatCard
          title="Total Cost"
          value={formatCurrency(stats?.total_cost || 0)}
          subtitle="This month"
          icon={CurrencyDollarIcon}
          color="orange"
        />
        <StatCard
          title="Avg Latency"
          value={`${stats?.average_latency_ms || 0}ms`}
          subtitle="Response time"
          icon={BoltIcon}
          color="purple"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="card"
        >
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Usage Trend</h2>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis
                  dataKey="date"
                  stroke="#8E8E93"
                  fontSize={12}
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
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
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="card"
        >
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Cost Trend</h2>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis
                  dataKey="date"
                  stroke="#8E8E93"
                  fontSize={12}
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis
                  stroke="#8E8E93"
                  fontSize={12}
                  tickFormatter={(value) => `$${value.toFixed(2)}`}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#fff',
                    border: '1px solid #E8E8ED',
                    borderRadius: '12px',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
                  }}
                  formatter={(value) => [`$${Number(value).toFixed(4)}`, 'Cost']}
                />
                <Line
                  type="monotone"
                  dataKey="cost"
                  stroke="#FF9500"
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </motion.div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
          className="card"
        >
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Provider Usage</h2>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={providerStats} layout="vertical">
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis type="number" stroke="#8E8E93" fontSize={12} />
                <YAxis
                  type="category"
                  dataKey="provider_name"
                  stroke="#8E8E93"
                  fontSize={12}
                  width={80}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#fff',
                    border: '1px solid #E8E8ED',
                    borderRadius: '12px',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
                  }}
                />
                <Bar dataKey="requests" fill="#007AFF" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
          className="card"
        >
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Top Models</h2>
          <div className="space-y-4">
            {modelStats.slice(0, 5).map((model, index) => (
              <div key={model.model_id} className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="w-6 h-6 bg-apple-gray-100 rounded-full flex items-center justify-center text-sm font-medium text-apple-gray-600">
                    {index + 1}
                  </span>
                  <span className="font-medium text-apple-gray-900">{model.model_name}</span>
                </div>
                <div className="text-right">
                  <p className="text-sm font-medium text-apple-gray-900">
                    {formatNumber(model.requests)} requests
                  </p>
                  <p className="text-xs text-apple-gray-500">{formatCurrency(model.total_cost)}</p>
                </div>
              </div>
            ))}
          </div>
        </motion.div>
      </div>
    </div>
  );
}

export default DashboardPage;
