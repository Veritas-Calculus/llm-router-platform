/* eslint-disable @typescript-eslint/no-explicit-any */

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { useQuery } from '@apollo/client/react';
import {
  ArrowTrendingUpIcon,
  CurrencyDollarIcon,
  ClockIcon,
  CheckCircleIcon,
  ExclamationCircleIcon,
  ServerStackIcon,
  KeyIcon,
  GlobeAltIcon,
  CommandLineIcon,
  UsersIcon,
  BanknotesIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';
import {
  LineChart,
  Line,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { useTranslation } from '@/lib/i18n';
import { ADMIN_DASHBOARD_QUERY } from '@/lib/graphql/operations/adminDashboard';

/* -- Helpers -- */

const fmtNum = (v: number): string => {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return new Intl.NumberFormat('en-US').format(v);
};

const fmtCurrency = (v: number): string =>
  new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(v);

const fmtTokens = (v: number): string => {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return new Intl.NumberFormat('en-US').format(v);
};

const tooltipStyle = {
  backgroundColor: '#fff',
  border: '1px solid #E8E8ED',
  borderRadius: '12px',
  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
};

const COLORS = ['#007AFF', '#34C759', '#FF9500', '#AF52DE', '#FF3B30', '#5AC8FA'];

/* -- Stat Card -- */

interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon: React.ElementType;
  color: 'blue' | 'green' | 'orange' | 'purple' | 'red' | 'indigo';
}

function StatCard({ title, value, subtitle, icon: Icon, color }: StatCardProps) {
  const cls: Record<string, string> = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    orange: 'bg-orange-50 text-orange-600',
    purple: 'bg-purple-50 text-purple-600',
    red: 'bg-red-50 text-red-600',
    indigo: 'bg-indigo-50 text-indigo-600',
  };

  return (
    <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-apple-gray-500 mb-1">{title}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{value}</p>
          {subtitle && <p className="text-sm text-apple-gray-400 mt-1">{subtitle}</p>}
        </div>
        <div className={`p-3 rounded-apple ${cls[color]}`}>
          <Icon className="w-6 h-6" />
        </div>
      </div>
    </motion.div>
  );
}

/* -- Health Badge -- */

function HealthBadge({ healthy, total, icon: Icon, label }: { healthy: number; total: number; icon: React.ElementType; label: string }) {
  const pct = total > 0 ? Math.round((healthy / total) * 100) : 0;
  const color = pct >= 80 ? 'green' : pct >= 50 ? 'orange' : 'red';
  const bg = color === 'green' ? 'bg-green-100 text-apple-green' : color === 'orange' ? 'bg-orange-100 text-apple-orange' : 'bg-red-100 text-apple-red';

  return (
    <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-apple-gray-50 rounded-apple"><Icon className="w-5 h-5 text-apple-gray-600" /></div>
          <div>
            <p className="text-sm text-apple-gray-500">{label}</p>
            <p className="text-xl font-semibold text-apple-gray-900">{healthy} / {total}</p>
          </div>
        </div>
        <div className={`px-2 py-1 rounded-full text-xs font-medium ${bg}`}>{pct}%</div>
      </div>
    </motion.div>
  );
}

/* -- Admin Dashboard Page -- */

function DashboardPage() {
  const { t } = useTranslation();
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const { data, loading, refetch } = useQuery<any>(ADMIN_DASHBOARD_QUERY, { pollInterval: 30_000 });

  useEffect(() => {
    if (data) {
      setLastUpdated(new Date());
    }
  }, [data]);

  const d = data?.adminDashboard;
  const chartData = data?.usageChart || [];
  const providerStats = data?.providerStats || [];
  const modelStats = data?.modelStats || [];

  if (loading && !d) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('admin.dashboard.title')}</h1>
          <p className="text-apple-gray-500 mt-1">{t('admin.dashboard.subtitle')}</p>
        </div>
        <div className="hidden sm:flex items-center gap-4 text-right">
          <div>
            <p className="text-sm text-apple-gray-500">{t('admin.dashboard.last_updated')}</p>
            <p className="text-sm font-medium text-apple-gray-700">
              {lastUpdated.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
            </p>
          </div>
          <button 
            onClick={() => refetch()} 
            disabled={loading}
            className="p-2 rounded-xl bg-white border border-apple-gray-200 shadow-sm text-apple-gray-600 hover:text-apple-gray-900 hover:bg-apple-gray-50 transition-colors disabled:opacity-50"
            title="Refresh Dashboard"
          >
            <ArrowPathIcon className={`w-5 h-5 ${loading ? 'animate-spin text-apple-blue' : ''}`} />
          </button>
        </div>
      </div>

      {/* Row 1: Platform KPIs */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title={t('admin.dashboard.total_users')} value={fmtNum(d?.totalUsers || 0)} subtitle={`${d?.activeUsersToday || 0} ${t('admin.dashboard.active_today')}`} icon={UsersIcon} color="indigo" />
        <StatCard title={t('admin.dashboard.active_users_month')} value={fmtNum(d?.activeUsersMonth || 0)} icon={UsersIcon} color="blue" />
        <StatCard title={t('admin.dashboard.total_revenue')} value={fmtCurrency(d?.totalRevenue || 0)} icon={BanknotesIcon} color="green" />
        <StatCard title={t('admin.dashboard.revenue_this_month')} value={fmtCurrency(d?.revenueThisMonth || 0)} icon={CurrencyDollarIcon} color="orange" />
      </div>

      {/* Row 2: Usage KPIs */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title={t('admin.dashboard.total_requests')} value={fmtNum(d?.totalRequests || 0)} subtitle={`${fmtNum(d?.requestsToday || 0)} ${t('admin.dashboard.today')}`} icon={ArrowTrendingUpIcon} color="blue" />
        <StatCard title={t('admin.dashboard.total_tokens')} value={fmtTokens(d?.totalTokens || 0)} subtitle={`${fmtTokens(d?.tokensToday || 0)} ${t('admin.dashboard.today')}`} icon={ClockIcon} color="purple" />
        <StatCard title={t('admin.dashboard.total_cost')} value={fmtCurrency(d?.totalCost || 0)} subtitle={`${fmtCurrency(d?.costToday || 0)} ${t('admin.dashboard.today')}`} icon={CurrencyDollarIcon} color="orange" />
        <StatCard
          title={t('admin.dashboard.success_rate')}
          value={`${(d?.successRate || 0).toFixed(1)}%`}
          subtitle={`${d?.errorCount || 0} ${t('admin.dashboard.errors')}`}
          icon={d?.successRate >= 95 ? CheckCircleIcon : ExclamationCircleIcon}
          color={d?.successRate >= 95 ? 'green' : 'red'}
        />
      </div>

      {/* Row 3: Infrastructure Health */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <HealthBadge healthy={d?.activeProviders || 0} total={d?.totalProviders || 0} icon={ServerStackIcon} label={t('admin.dashboard.providers')} />
        <HealthBadge healthy={d?.activeProxies || 0} total={d?.totalProxies || 0} icon={GlobeAltIcon} label={t('admin.dashboard.proxies')} />
        <HealthBadge healthy={d?.apiKeysHealthy || 0} total={d?.apiKeysTotal || 0} icon={KeyIcon} label={t('admin.dashboard.api_keys')} />
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-purple-50 rounded-apple"><CommandLineIcon className="w-5 h-5 text-purple-600" /></div>
              <div>
                <p className="text-sm text-apple-gray-500">{t('admin.dashboard.mcp_calls')}</p>
                <p className="text-xl font-semibold text-apple-gray-900">{fmtNum(d?.mcpCallCount || 0)}</p>
              </div>
            </div>
            <div className={`px-2 py-1 rounded-full text-xs font-medium ${(d?.mcpErrorCount || 0) === 0 ? 'bg-green-100 text-apple-green' : 'bg-red-100 text-apple-red'}`}>
              {d?.mcpErrorCount || 0} errors
            </div>
          </div>
        </motion.div>
      </div>

      {/* Charts Row 1: Request Trend + Cost Trend */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.dashboard.request_trend')}</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            {chartData.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
                <ArrowTrendingUpIcon className="w-10 h-10 mb-2 opacity-50" />
                <p className="text-sm font-medium">{t('admin.dashboard.no_data')}</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%" minHeight={256}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                  <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
                  <YAxis stroke="#8E8E93" fontSize={12} />
                  <Tooltip contentStyle={tooltipStyle} />
                  <Line type="monotone" dataKey="requests" stroke="#007AFF" strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.dashboard.cost_trend')}</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            {chartData.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
                <CurrencyDollarIcon className="w-10 h-10 mb-2 opacity-50" />
                <p className="text-sm font-medium">{t('admin.dashboard.no_data')}</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%" minHeight={256}>
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                  <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
                  <YAxis stroke="#8E8E93" fontSize={12} tickFormatter={(v) => `$${v.toFixed(2)}`} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(value) => [`$${Number(value).toFixed(4)}`, 'Cost']} />
                  <Bar dataKey="cost" fill="#FF9500" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </motion.div>
      </div>

      {/* Provider & Model Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.dashboard.provider_usage')}</h2>
          {providerStats.length === 0 ? (
            <div className="h-48 flex items-center justify-center text-apple-gray-400">
              <div className="text-center"><ServerStackIcon className="w-12 h-12 mx-auto mb-2 opacity-50" /><p>{t('admin.dashboard.no_data')}</p></div>
            </div>
          ) : (
            <div className="space-y-4">
              {providerStats.slice(0, 5).map((p: any, i: number) => (
                <div key={p.providerName} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-3 h-3 rounded-full" style={{ backgroundColor: COLORS[i % COLORS.length] }} />
                    <span className="font-medium text-apple-gray-900">{p.providerName}</span>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{fmtNum(p.requests)} req</p>
                      <p className="text-xs text-apple-gray-500">{fmtCurrency(p.totalCost)}</p>
                    </div>
                    <div className={`px-2 py-0.5 rounded text-xs font-medium ${p.successRate >= 95 ? 'bg-green-100 text-apple-green' : p.successRate >= 80 ? 'bg-orange-100 text-apple-orange' : 'bg-red-100 text-apple-red'}`}>
                      {p.successRate?.toFixed(0) || 0}%
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.dashboard.top_models')}</h2>
          {modelStats.length === 0 ? (
            <div className="h-48 flex items-center justify-center text-apple-gray-400">
              <div className="text-center"><ClockIcon className="w-12 h-12 mx-auto mb-2 opacity-50" /><p>{t('admin.dashboard.no_data')}</p></div>
            </div>
          ) : (
            <div className="space-y-4">
              {modelStats.slice(0, 5).map((m: any, i: number) => (
                <div key={m.modelName} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="w-6 h-6 bg-apple-gray-100 rounded-full flex items-center justify-center text-sm font-medium text-apple-gray-600">{i + 1}</span>
                    <div>
                      <span className="font-medium text-apple-gray-900">{m.modelName}</span>
                      <p className="text-xs text-apple-gray-500">{fmtTokens(m.inputTokens)} in / {fmtTokens(m.outputTokens)} out</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <p className="text-sm font-medium text-apple-gray-900">{fmtNum(m.requests)} req</p>
                    <p className="text-xs text-apple-gray-500">{fmtCurrency(m.totalCost)}</p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      </div>
    </div>
  );
}

export default DashboardPage;
