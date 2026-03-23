/* eslint-disable @typescript-eslint/no-explicit-any */

import { useState, useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { useTranslation } from '@/lib/i18n';
import {
  ArrowTrendingUpIcon,
  UsersIcon,
  ClockIcon,
  ServerStackIcon,
  BanknotesIcon,
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
import {
  ADMIN_USAGE_BY_USER_QUERY,
  ADMIN_REVENUE_CHART_QUERY,
  ADMIN_USER_GROWTH_QUERY,
} from '@/lib/graphql/operations/adminDashboard';
import { DASHBOARD_QUERY } from '@/lib/graphql/operations/dashboard';

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

const tabs = [
  { key: 'usage', labelKey: 'admin.analytics.tab_usage' },
  { key: 'revenue', labelKey: 'admin.analytics.tab_revenue' },
  { key: 'users', labelKey: 'admin.analytics.tab_users' },
] as const;

type TabKey = (typeof tabs)[number]['key'];

/* ============================================================
   Tab: Usage (system-wide)
   ============================================================ */

function UsageTab() {
  const { t } = useTranslation();
  const { data, loading } = useQuery<any>(DASHBOARD_QUERY, { variables: { days: 30 } });

  const chartData = useMemo(() => data?.usageChart || [], [data]);
  const providerStats = useMemo(() => data?.providerStats || [], [data]);
  const modelStats = useMemo(() => data?.modelStats || [], [data]);
  const d = data?.dashboard;

  if (loading && !d) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.total_requests')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{fmtNum(d?.totalRequests || 0)}</p>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.total_tokens')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{fmtTokens(d?.totalTokens || 0)}</p>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.total_cost')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{fmtCurrency(d?.totalCost || 0)}</p>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.success_rate')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{(d?.successRate || 0).toFixed(1)}%</p>
        </motion.div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.analytics.requests_30d')}</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            {chartData.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
                <ArrowTrendingUpIcon className="w-10 h-10 mb-2 opacity-50" />
                <p className="text-sm">{t('admin.dashboard.no_data')}</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
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
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.analytics.tokens_30d')}</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            {chartData.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
                <ClockIcon className="w-10 h-10 mb-2 opacity-50" />
                <p className="text-sm">{t('admin.dashboard.no_data')}</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                  <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
                  <YAxis stroke="#8E8E93" fontSize={12} tickFormatter={(v) => fmtTokens(v)} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(value) => [fmtTokens(Number(value)), 'Tokens']} />
                  <Line type="monotone" dataKey="tokens" stroke="#AF52DE" strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </motion.div>
      </div>

      {/* Provider & Model Breakdown */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.dashboard.provider_usage')}</h2>
          {providerStats.length === 0 ? (
            <div className="h-48 flex items-center justify-center text-apple-gray-400">
              <div className="text-center"><ServerStackIcon className="w-12 h-12 mx-auto mb-2 opacity-50" /><p>{t('admin.dashboard.no_data')}</p></div>
            </div>
          ) : (
            <div className="space-y-3">
              {providerStats.map((p: any, i: number) => (
                <div key={p.providerName} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-3 h-3 rounded-full" style={{ backgroundColor: COLORS[i % COLORS.length] }} />
                    <span className="font-medium text-apple-gray-900">{p.providerName}</span>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className="text-sm text-apple-gray-600">{fmtNum(p.requests)} req</span>
                    <span className="text-sm text-apple-gray-600">{fmtCurrency(p.totalCost)}</span>
                    <div className={`px-2 py-0.5 rounded text-xs font-medium ${p.successRate >= 95 ? 'bg-green-100 text-apple-green' : 'bg-orange-100 text-apple-orange'}`}>
                      {p.successRate?.toFixed(0)}%
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
            <div className="space-y-3">
              {modelStats.map((m: any, i: number) => (
                <div key={m.modelName} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="w-6 h-6 bg-apple-gray-100 rounded-full flex items-center justify-center text-xs font-medium text-apple-gray-600">{i + 1}</span>
                    <span className="font-medium text-apple-gray-900">{m.modelName}</span>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className="text-sm text-apple-gray-600">{fmtNum(m.requests)} req</span>
                    <span className="text-sm text-apple-gray-600">{fmtCurrency(m.totalCost)}</span>
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

/* ============================================================
   Tab: Revenue
   ============================================================ */

function RevenueTab() {
  const { t } = useTranslation();
  const { data, loading } = useQuery<any>(ADMIN_REVENUE_CHART_QUERY, { variables: { days: 30 } });

  const chartData = useMemo(() => data?.adminRevenueChart || [], [data]);
  const totalRevenue = useMemo(() => chartData.reduce((sum: number, p: any) => sum + (p.revenue || 0), 0), [chartData]);
  const totalTxns = useMemo(() => chartData.reduce((sum: number, p: any) => sum + (p.transactions || 0), 0), [chartData]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.revenue_30d')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{fmtCurrency(totalRevenue)}</p>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.transactions_30d')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{fmtNum(totalTxns)}</p>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <p className="text-sm text-apple-gray-500 mb-1">{t('admin.analytics.avg_per_txn')}</p>
          <p className="text-2xl font-semibold text-apple-gray-900">{fmtCurrency(totalTxns > 0 ? totalRevenue / totalTxns : 0)}</p>
        </motion.div>
      </div>

      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.analytics.revenue_trend')}</h2>
        <div className="h-72" style={{ minHeight: '288px' }}>
          {chartData.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
              <BanknotesIcon className="w-10 h-10 mb-2 opacity-50" />
              <p className="text-sm">{t('admin.analytics.no_revenue_data')}</p>
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
                <YAxis stroke="#8E8E93" fontSize={12} tickFormatter={(v) => `$${v}`} />
                <Tooltip contentStyle={tooltipStyle} formatter={(value) => [fmtCurrency(Number(value)), 'Revenue']} />
                <Bar dataKey="revenue" fill="#34C759" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </motion.div>
    </div>
  );
}

/* ============================================================
   Tab: Users
   ============================================================ */

function UsersTab() {
  const { t } = useTranslation();
  const { data: usageData, loading: l1 } = useQuery<any>(ADMIN_USAGE_BY_USER_QUERY, { variables: { days: 30 } });
  const { data: growthData, loading: l2 } = useQuery<any>(ADMIN_USER_GROWTH_QUERY, { variables: { days: 30 } });

  const users = useMemo(() => usageData?.adminUsageByUser || [], [usageData]);
  const growth = useMemo(() => growthData?.adminUserGrowth || [], [growthData]);
  const loading = l1 || l2;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* User Growth Chart */}
      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.analytics.user_growth')}</h2>
        <div className="h-64" style={{ minHeight: '256px' }}>
          {growth.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
              <UsersIcon className="w-10 h-10 mb-2 opacity-50" />
              <p className="text-sm">{t('admin.analytics.no_growth_data')}</p>
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={growth}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
                <YAxis yAxisId="left" stroke="#8E8E93" fontSize={12} />
                <YAxis yAxisId="right" orientation="right" stroke="#8E8E93" fontSize={12} />
                <Tooltip contentStyle={tooltipStyle} />
                <Line yAxisId="left" type="monotone" dataKey="totalUsers" stroke="#007AFF" strokeWidth={2} dot={false} name="Total Users" />
                <Bar yAxisId="right" dataKey="newUsers" fill="#34C759" opacity={0.6} name="New Users" />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </motion.div>

      {/* Per-User Usage Table */}
      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.analytics.usage_by_user')}</h2>
        {users.length === 0 ? (
          <div className="h-48 flex items-center justify-center text-apple-gray-400">
            <div className="text-center"><UsersIcon className="w-12 h-12 mx-auto mb-2 opacity-50" /><p>{t('admin.analytics.no_user_data')}</p></div>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-apple-gray-200">
                  <th className="text-left py-3 px-4 font-semibold text-apple-gray-600">{t('admin.analytics.user')}</th>
                  <th className="text-left py-3 px-4 font-semibold text-apple-gray-600">{t('admin.analytics.email')}</th>
                  <th className="text-right py-3 px-4 font-semibold text-apple-gray-600">{t('admin.analytics.requests')}</th>
                  <th className="text-right py-3 px-4 font-semibold text-apple-gray-600">{t('admin.analytics.tokens')}</th>
                  <th className="text-right py-3 px-4 font-semibold text-apple-gray-600">{t('admin.analytics.cost')}</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u: any) => (
                  <tr key={u.userId} className="border-b border-apple-gray-100 hover:bg-apple-gray-50 transition-colors">
                    <td className="py-3 px-4 font-medium text-apple-gray-900">{u.userName || '--'}</td>
                    <td className="py-3 px-4 text-apple-gray-600">{u.email}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-900">{fmtNum(u.requests)}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-900">{fmtTokens(u.tokens)}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-900">{fmtCurrency(u.cost)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </motion.div>
    </div>
  );
}

/* ============================================================
   Main: Admin Analytics Page
   ============================================================ */

export default function AnalyticsPage() {
  const { t } = useTranslation();
  const [active, setActive] = useState<TabKey>('usage');

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">{t('admin.analytics.title')}</h1>
        <p className="text-apple-gray-500 mt-1">{t('admin.analytics.subtitle')}</p>
      </div>

      {/* Tab bar */}
      <div className="flex items-center gap-1 bg-apple-gray-100 p-1 rounded-xl w-fit border border-apple-gray-200">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActive(tab.key)}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
              active === tab.key
                ? 'bg-white text-apple-blue shadow-sm'
                : 'text-apple-gray-500 hover:text-apple-gray-700'
            }`}
          >
            {t(tab.labelKey)}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {active === 'usage' && <UsageTab />}
      {active === 'revenue' && <RevenueTab />}
      {active === 'users' && <UsersTab />}
    </div>
  );
}
