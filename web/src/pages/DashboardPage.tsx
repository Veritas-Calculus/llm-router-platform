import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  ArrowTrendingUpIcon,
  CurrencyDollarIcon,
  BoltIcon,
  CheckCircleIcon,
  ClockIcon,
  ExclamationCircleIcon,
  ServerStackIcon,
  KeyIcon,
  GlobeAltIcon,
} from '@heroicons/react/24/outline';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
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
  color: 'blue' | 'green' | 'orange' | 'purple' | 'red';
  trend?: { value: number; label: string };
}

function StatCard({ title, value, subtitle, icon: Icon, color, trend }: StatCardProps) {
  const colorClasses = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    orange: 'bg-orange-50 text-orange-600',
    purple: 'bg-purple-50 text-purple-600',
    red: 'bg-red-50 text-red-600',
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
          {trend && (
            <p className={`text-xs mt-1 ${trend.value >= 0 ? 'text-apple-green' : 'text-apple-red'}`}>
              {trend.value >= 0 ? '↑' : '↓'} {Math.abs(trend.value)}% {trend.label}
            </p>
          )}
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
      setChartData(chartRes?.data || []);
      setProviderStats(providerRes?.data || []);
      setModelStats(modelRes?.data || []);
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
    if (value >= 1000000) {
      return (value / 1000000).toFixed(1) + 'M';
    }
    if (value >= 1000) {
      return (value / 1000).toFixed(1) + 'K';
    }
    return new Intl.NumberFormat('en-US').format(value);
  };

  const formatTokens = (value: number): string => {
    if (value >= 1000000) {
      return (value / 1000000).toFixed(2) + 'M';
    }
    if (value >= 1000) {
      return (value / 1000).toFixed(1) + 'K';
    }
    return new Intl.NumberFormat('en-US').format(value);
  };

  const COLORS = ['#007AFF', '#34C759', '#FF9500', '#AF52DE', '#FF3B30', '#5AC8FA'];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Dashboard</h1>
          <p className="text-apple-gray-500 mt-1">Overview of your LLM usage and performance</p>
        </div>
        <div className="text-right">
          <p className="text-sm text-apple-gray-500">Last updated</p>
          <p className="text-sm font-medium text-apple-gray-700">
            {new Date().toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}
          </p>
        </div>
      </div>

      {/* Main Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Total Requests"
          value={formatNumber(stats?.total_requests || 0)}
          subtitle={`${formatNumber(stats?.requests_today || 0)} today`}
          icon={ArrowTrendingUpIcon}
          color="blue"
        />
        <StatCard
          title="Total Tokens"
          value={formatTokens(stats?.total_tokens || 0)}
          subtitle={`${formatTokens(stats?.tokens_today || 0)} today`}
          icon={ClockIcon}
          color="purple"
        />
        <StatCard
          title="Total Cost"
          value={formatCurrency(stats?.total_cost || 0)}
          subtitle={`${formatCurrency(stats?.cost_today || 0)} today`}
          icon={CurrencyDollarIcon}
          color="orange"
        />
        <StatCard
          title="Success Rate"
          value={`${(stats?.success_rate || 0).toFixed(1)}%`}
          subtitle={`${stats?.error_count || 0} errors`}
          icon={stats?.success_rate && stats.success_rate >= 95 ? CheckCircleIcon : ExclamationCircleIcon}
          color={stats?.success_rate && stats.success_rate >= 95 ? 'green' : 'red'}
        />
      </div>

      {/* System Health Row */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="card"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-50 rounded-apple">
                <ServerStackIcon className="w-5 h-5 text-blue-600" />
              </div>
              <div>
                <p className="text-sm text-apple-gray-500">Active Providers</p>
                <p className="text-xl font-semibold text-apple-gray-900">{stats?.active_providers || 0}</p>
              </div>
            </div>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 }}
          className="card"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-green-50 rounded-apple">
                <KeyIcon className="w-5 h-5 text-green-600" />
              </div>
              <div>
                <p className="text-sm text-apple-gray-500">API Keys Health</p>
                <p className="text-xl font-semibold text-apple-gray-900">
                  {stats?.api_keys?.healthy || 0} / {stats?.api_keys?.total || 0}
                </p>
              </div>
            </div>
            <div className={`px-2 py-1 rounded-full text-xs font-medium ${
              stats?.api_keys?.healthy === stats?.api_keys?.total 
                ? 'bg-green-100 text-apple-green' 
                : 'bg-orange-100 text-apple-orange'
            }`}>
              {stats?.api_keys?.total ? Math.round((stats?.api_keys?.healthy || 0) / stats.api_keys.total * 100) : 0}% healthy
            </div>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="card"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-purple-50 rounded-apple">
                <GlobeAltIcon className="w-5 h-5 text-purple-600" />
              </div>
              <div>
                <p className="text-sm text-apple-gray-500">Proxies Health</p>
                <p className="text-xl font-semibold text-apple-gray-900">
                  {stats?.proxies?.healthy || 0} / {stats?.proxies?.total || 0}
                </p>
              </div>
            </div>
            <div className={`px-2 py-1 rounded-full text-xs font-medium ${
              stats?.proxies?.healthy === stats?.proxies?.total 
                ? 'bg-green-100 text-apple-green' 
                : 'bg-orange-100 text-apple-orange'
            }`}>
              {stats?.proxies?.total ? Math.round((stats?.proxies?.healthy || 0) / stats.proxies.total * 100) : 0}% healthy
            </div>
          </div>
        </motion.div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="card"
        >
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Request Trend (7 Days)</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            <ResponsiveContainer width="100%" height="100%" minHeight={256}>
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
          <div className="h-64" style={{ minHeight: '256px' }}>
            <ResponsiveContainer width="100%" height="100%" minHeight={256}>
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
          {providerStats.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-apple-gray-400">
              <div className="text-center">
                <ServerStackIcon className="w-12 h-12 mx-auto mb-2 opacity-50" />
                <p>No provider usage data yet</p>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              {providerStats.slice(0, 5).map((provider, index) => (
                <div key={provider.provider_id} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div
                      className="w-3 h-3 rounded-full"
                      style={{ backgroundColor: COLORS[index % COLORS.length] }}
                    />
                    <span className="font-medium text-apple-gray-900">{provider.provider_name}</span>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">
                        {formatNumber(provider.requests)} req
                      </p>
                      <p className="text-xs text-apple-gray-500">{formatCurrency(provider.total_cost)}</p>
                    </div>
                    <div className={`px-2 py-0.5 rounded text-xs font-medium ${
                      provider.success_rate >= 95
                        ? 'bg-green-100 text-apple-green'
                        : provider.success_rate >= 80
                        ? 'bg-orange-100 text-apple-orange'
                        : 'bg-red-100 text-apple-red'
                    }`}>
                      {provider.success_rate?.toFixed(0) || 0}%
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
          className="card"
        >
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Top Models</h2>
          {modelStats.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-apple-gray-400">
              <div className="text-center">
                <BoltIcon className="w-12 h-12 mx-auto mb-2 opacity-50" />
                <p>No model usage data yet</p>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              {modelStats.slice(0, 5).map((model, index) => (
                <div key={model.model_id} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="w-6 h-6 bg-apple-gray-100 rounded-full flex items-center justify-center text-sm font-medium text-apple-gray-600">
                      {index + 1}
                    </span>
                    <div>
                      <span className="font-medium text-apple-gray-900">{model.model_name}</span>
                      <p className="text-xs text-apple-gray-500">
                        {formatTokens(model.input_tokens)} in / {formatTokens(model.output_tokens)} out
                      </p>
                    </div>
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
          )}
        </motion.div>
      </div>

      {/* Token Usage Chart */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.5 }}
        className="card"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Token Usage Trend</h2>
        <div className="h-64" style={{ minHeight: '256px' }}>
          <ResponsiveContainer width="100%" height="100%" minHeight={256}>
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
                tickFormatter={(value) => formatTokens(value)}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#fff',
                  border: '1px solid #E8E8ED',
                  borderRadius: '12px',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
                }}
                formatter={(value) => [formatTokens(Number(value)), 'Tokens']}
              />
              <Line
                type="monotone"
                dataKey="tokens"
                stroke="#AF52DE"
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </motion.div>
    </div>
  );
}

export default DashboardPage;
