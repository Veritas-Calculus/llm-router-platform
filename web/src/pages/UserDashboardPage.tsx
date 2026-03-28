/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useQuery } from '@apollo/client/react';
import {
  ArrowTrendingUpIcon,
  CurrencyDollarIcon,
  ClockIcon,
  CheckCircleIcon,
  ExclamationCircleIcon,
  ExclamationTriangleIcon,
  XMarkIcon,
  ServerStackIcon,

  InformationCircleIcon,
  WrenchScrewdriverIcon,
} from '@heroicons/react/24/outline';
import { ACTIVE_ANNOUNCEMENTS_QUERY } from '@/lib/graphql/operations/announcements';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { useUserDashboard } from '@/hooks/useUserDashboard';
import { useTranslation } from '@/lib/i18n';

import QuickStartGuide from '@/components/QuickStartGuide';

/* ── Announcement Banner ── */

interface AnnouncementItem {
  id: string;
  title: string;
  content: string;
  type: string;
  priority: number;
  startsAt?: string;
  endsAt?: string;
  createdAt: string;
}

function AnnouncementBanner() {
  const { data } = useQuery<{ activeAnnouncements: AnnouncementItem[] }>(ACTIVE_ANNOUNCEMENTS_QUERY, {
    pollInterval: 60000,
  });
  const [dismissed, setDismissed] = useState<Set<string>>(() => {
    try {
      const stored = localStorage.getItem('dismissedAnnouncements');
      return stored ? new Set(JSON.parse(stored)) : new Set();
    } catch { return new Set(); }
  });

  const announcements = (data?.activeAnnouncements || []).filter(a => !dismissed.has(a.id));

  const dismiss = useCallback((id: string) => {
    setDismissed(prev => {
      const next = new Set(prev);
      next.add(id);
      localStorage.setItem('dismissedAnnouncements', JSON.stringify([...next]));
      return next;
    });
  }, []);

  if (announcements.length === 0) return null;

  const typeConfig: Record<string, { icon: React.ElementType; bg: string; border: string; title: string; text: string }> = {
    info: {
      icon: InformationCircleIcon,
      bg: 'bg-blue-50/80 dark:bg-blue-900/20',
      border: 'border-l-blue-500',
      title: 'text-blue-800 dark:text-blue-300',
      text: 'text-blue-700 dark:text-blue-400',
    },
    warning: {
      icon: ExclamationTriangleIcon,
      bg: 'bg-amber-50/80 dark:bg-amber-900/20',
      border: 'border-l-amber-500',
      title: 'text-amber-800 dark:text-amber-300',
      text: 'text-amber-700 dark:text-amber-400',
    },
    maintenance: {
      icon: WrenchScrewdriverIcon,
      bg: 'bg-orange-50/80 dark:bg-orange-900/20',
      border: 'border-l-orange-500',
      title: 'text-orange-800 dark:text-orange-300',
      text: 'text-orange-700 dark:text-orange-400',
    },
  };

  return (
    <AnimatePresence>
      {announcements.map(a => {
        const cfg = typeConfig[a.type] || typeConfig.info;
        const Icon = cfg.icon;
        return (
          <motion.div
            key={a.id}
            initial={{ opacity: 0, y: -8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, height: 0, marginBottom: 0 }}
            className={`card border-l-4 ${cfg.border} ${cfg.bg} relative`}
          >
            <button
              onClick={() => dismiss(a.id)}
              className="absolute top-3 right-3 p-1 rounded-lg hover:bg-black/5 dark:hover:bg-white/10 transition-colors"
            >
              <XMarkIcon className="w-4 h-4 text-apple-gray-400" />
            </button>
            <div className="flex items-start gap-3 pr-8">
              <div className="flex-shrink-0 mt-0.5">
                <Icon className={`w-5 h-5 ${cfg.text}`} />
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <h3 className={`font-semibold text-sm ${cfg.title}`}>{a.title}</h3>
                  <span className={`text-xs px-1.5 py-0.5 rounded-full font-medium ${cfg.bg} ${cfg.text} border border-current/10`}>
                    {a.type}
                  </span>
                </div>
                <p className={`text-sm ${cfg.text} leading-relaxed`}>{a.content}</p>
                {(a.startsAt || a.endsAt) && (
                  <p className={`text-xs ${cfg.text} opacity-70 mt-1.5`}>
                    {a.startsAt && `From: ${new Date(a.startsAt).toLocaleString()}`}
                    {a.startsAt && a.endsAt && ' - '}
                    {a.endsAt && `Until: ${new Date(a.endsAt).toLocaleString()}`}
                  </p>
                )}
              </div>
            </div>
          </motion.div>
        );
      })}
    </AnimatePresence>
  );
}

/* ── Stat Card ── */

interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon: React.ElementType;
  color: 'blue' | 'green' | 'orange' | 'purple' | 'red';
}

function StatCard({ title, value, subtitle, icon: Icon, color }: StatCardProps) {
  const colorClasses = {
    blue: 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400',
    green: 'bg-green-50 text-green-600 dark:bg-green-900/30 dark:text-green-400',
    orange: 'bg-orange-50 text-orange-600 dark:bg-orange-900/30 dark:text-orange-400',
    purple: 'bg-purple-50 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400',
    red: 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-400',
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

/* ── Tooltip style ── */

const tooltipStyle = {
  backgroundColor: '#fff',
  border: '1px solid #E8E8ED',
  borderRadius: '12px',
  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
};

/* ── Main Page ── */

function UserDashboardPage() {
  const { t } = useTranslation();
  const [channelFilter, setChannelFilter] = useState('');
  const [debouncedChannel, setDebouncedChannel] = useState('');
  const [quickStartDismissed, setQuickStartDismissed] = useState(() => {
    return localStorage.getItem('quickStartDismissed') === 'true';
  });

  const dismissQuickStart = useCallback(() => {
    setQuickStartDismissed(true);
    localStorage.setItem('quickStartDismissed', 'true');
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedChannel(channelFilter);
    }, 500);
    return () => clearTimeout(timer);
  }, [channelFilter]);

  const {
    me,
    summary,
    chartData,
    providerUsage,
    budgetStatus,
    anomaly,
    loading,
    formatCurrency,
    formatNumber,
    formatTokens,
    COLORS,
  } = useUserDashboard({
    channel: debouncedChannel || undefined,
  });

  if (loading && !summary) {
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
          <h1 className="text-2xl font-semibold" style={{ color: 'var(--theme-text)' }}>
            {me?.name ? t('user_dashboard.welcome', { name: me.name.split(' ')[0] }) : t('user_dashboard.my_dashboard')}
          </h1>
          <p className="mt-1" style={{ color: 'var(--theme-text-secondary)' }}>{t('user_dashboard.subtitle')}</p>
        </div>
        <div className="flex items-center gap-4">
          <div className="relative">
            <input
              type="text"
              placeholder={t('user_dashboard.filter_channel')}
              value={channelFilter}
              onChange={(e) => setChannelFilter(e.target.value)}
              className="pl-3 pr-4 py-2 text-sm rounded-apple-lg focus:outline-none focus:ring-2 focus:ring-apple-blue/50 focus:border-apple-blue transition-shadow w-48"
              style={{
                backgroundColor: 'var(--theme-bg-card)',
                border: '1px solid var(--theme-border)',
                color: 'var(--theme-text)',
              }}
            />
          </div>
          <div className="text-right whitespace-nowrap hidden sm:block">
            <p className="text-sm" style={{ color: 'var(--theme-text-muted)' }}>{t('user_dashboard.last_updated')}</p>
            <p className="text-sm font-medium" style={{ color: 'var(--theme-text-secondary)' }}>
              {new Date().toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
            </p>
          </div>
        </div>
      </div>

      {/* Quick Start Guide */}
      <AnimatePresence>
        {!quickStartDismissed && (summary?.totalRequests || 0) === 0 && (
          <QuickStartGuide onDismiss={dismissQuickStart} />
        )}
      </AnimatePresence>

      {/* System Announcements */}
      <AnnouncementBanner />

      {/* Anomaly Alert */}
      {anomaly?.hasAnomaly && (
        <motion.div
          initial={{ opacity: 0, y: -5 }}
          animate={{ opacity: 1, y: 0 }}
          className="card border-l-4 border-l-amber-500 bg-amber-50/50 dark:bg-amber-900/10"
        >
          <div className="flex items-center gap-3">
            <ExclamationTriangleIcon className="w-5 h-5 text-amber-600 flex-shrink-0" />
            <div>
              <p className="text-sm font-semibold text-amber-800 dark:text-amber-400">{t('user_dashboard.anomaly_detected')}</p>
              <p className="text-xs text-amber-600 dark:text-amber-500 mt-0.5">{anomaly.message}</p>
            </div>
          </div>
        </motion.div>
      )}

      {/* Budget & Balance Row */}
      {me && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          {/* Balance */}
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card bg-gradient-to-br from-blue-50/60 to-indigo-50/40 dark:from-blue-900/10 dark:to-indigo-900/10 border border-blue-100 dark:border-blue-800/30">
            <div className="flex justify-between items-center">
              <div>
                <p className="text-sm text-apple-gray-500">{t('user_dashboard.account_balance')}</p>
                <p className="text-2xl font-bold text-apple-blue mt-1">{formatCurrency(me.balance || 0)}</p>
              </div>
              <div className="p-3 bg-blue-100 dark:bg-blue-900/30 rounded-apple">
                <CurrencyDollarIcon className="w-6 h-6 text-blue-600 dark:text-blue-400" />
              </div>
            </div>
            <p className="text-xs text-apple-gray-400 mt-2">{t('user_dashboard.available_payg')}</p>
          </motion.div>

          {/* Monthly Spend / Budget */}
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="card border border-apple-gray-200 dark:border-[var(--theme-border)]">
            <div className="flex justify-between text-sm mb-2">
              <span className="font-semibold text-apple-gray-900">{t('user_dashboard.monthly_spend')}</span>
              <span className="text-apple-gray-600 font-medium">
                {formatCurrency(summary?.totalCost || 0)}
                {budgetStatus?.budget?.monthlyLimitUsd ? ` / ${formatCurrency(budgetStatus.budget.monthlyLimitUsd)}` : ''}
              </span>
            </div>
            {budgetStatus?.budget?.monthlyLimitUsd ? (
              <div className="w-full bg-[var(--theme-bg-input)] rounded-full h-2.5 overflow-hidden border border-apple-gray-200 dark:border-[var(--theme-border)]">
                <div
                  className={`h-2.5 rounded-full transition-all ${(budgetStatus.percentUsed || 0) > 90 ? 'bg-apple-red' : (budgetStatus.percentUsed || 0) > 75 ? 'bg-apple-orange' : 'bg-apple-blue'}`}
                  style={{ width: `${Math.min(100, budgetStatus.percentUsed || 0)}%` }}
                />
              </div>
            ) : (
              <p className="text-xs text-apple-gray-400">{t('user_dashboard.no_budget_limit')}</p>
            )}
            {budgetStatus?.isOverBudget && (
              <p className="text-xs text-apple-red mt-1 font-medium">{t('user_dashboard.over_budget')}</p>
            )}
          </motion.div>

          {/* Token Limit */}
          {me.monthlyTokenLimit > 0 ? (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card border border-apple-gray-200 dark:border-[var(--theme-border)]">
              <div className="flex justify-between text-sm mb-2">
                <span className="font-semibold text-apple-gray-900">{t('user_dashboard.token_limit')}</span>
                <span className="text-apple-gray-600 font-medium">
                  {formatTokens(summary?.totalTokens || 0)} / {formatTokens(me.monthlyTokenLimit)}
                </span>
              </div>
              <div className="w-full bg-[var(--theme-bg-input)] rounded-full h-2.5 overflow-hidden border border-apple-gray-200 dark:border-[var(--theme-border)]">
                <div
                  className={`h-2.5 rounded-full ${((summary?.totalTokens || 0) / me.monthlyTokenLimit) > 0.9 ? 'bg-apple-red' : ((summary?.totalTokens || 0) / me.monthlyTokenLimit) > 0.75 ? 'bg-apple-orange' : 'bg-apple-purple'}`}
                  style={{ width: `${Math.min(100, ((summary?.totalTokens || 0) / me.monthlyTokenLimit) * 100)}%` }}
                />
              </div>
            </motion.div>
          ) : (
            <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card border border-apple-gray-200 dark:border-[var(--theme-border)]">
              <div className="flex justify-between items-center">
                <div>
                  <p className="text-sm text-apple-gray-500">{t('user_dashboard.tokens_used')}</p>
                  <p className="text-2xl font-semibold text-apple-gray-900 mt-1">{formatTokens(summary?.totalTokens || 0)}</p>
                </div>
                <div className="p-3 bg-purple-50 dark:bg-purple-900/30 rounded-apple">
                  <ClockIcon className="w-6 h-6 text-purple-600 dark:text-purple-400" />
                </div>
              </div>
              <p className="text-xs text-apple-gray-400 mt-2">{t('user_dashboard.no_token_limit')}</p>
            </motion.div>
          )}
        </div>
      )}

      {/* Main Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title={t('user_dashboard.my_requests')} value={formatNumber(summary?.totalRequests || 0)} subtitle={t('user_dashboard.this_month')} icon={ArrowTrendingUpIcon} color="blue" />
        <StatCard title={t('user_dashboard.my_tokens')} value={formatTokens(summary?.totalTokens || 0)} subtitle={t('user_dashboard.this_month')} icon={ClockIcon} color="purple" />
        <StatCard title={t('user_dashboard.my_spend')} value={formatCurrency(summary?.totalCost || 0)} subtitle={t('user_dashboard.this_month')} icon={CurrencyDollarIcon} color="orange" />
        <StatCard
          title={t('user_dashboard.success_rate')}
          value={`${(summary?.successRate || 0).toFixed(1)}%`}
          subtitle={t('common.of_all_requests')}
          icon={summary?.successRate && summary.successRate >= 95 ? CheckCircleIcon : ExclamationCircleIcon}
          color={summary?.successRate && summary.successRate >= 95 ? 'green' : 'red'}
        />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('user_dashboard.request_trend')}</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            {(!chartData || chartData.length === 0) ? (
              <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
                <ArrowTrendingUpIcon className="w-10 h-10 mb-2 opacity-50" />
                <p className="text-sm font-medium">{t('user_dashboard.no_request_data')}</p>
                <p className="text-xs mt-1">{t('user_dashboard.request_data_hint')}</p>
              </div>
            ) : (
            <ResponsiveContainer width="100%" height="100%" minHeight={256}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} />
                <YAxis stroke="#8E8E93" fontSize={12} />
                <Tooltip contentStyle={tooltipStyle} />
                <Line type="monotone" dataKey="requests" stroke="#007AFF" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
            )}
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('user_dashboard.spend_trend')}</h2>
          <div className="h-64" style={{ minHeight: '256px' }}>
            {(!chartData || chartData.length === 0) ? (
              <div className="flex flex-col items-center justify-center h-full text-apple-gray-400">
                <CurrencyDollarIcon className="w-10 h-10 mb-2 opacity-50" />
                <p className="text-sm font-medium">{t('user_dashboard.no_cost_data')}</p>
                <p className="text-xs mt-1">{t('user_dashboard.cost_data_hint')}</p>
              </div>
            ) : (
            <ResponsiveContainer width="100%" height="100%" minHeight={256}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} />
                <YAxis stroke="#8E8E93" fontSize={12} tickFormatter={(v) => `$${v.toFixed(2)}`} />
                <Tooltip contentStyle={tooltipStyle} formatter={(value) => [`$${Number(value).toFixed(4)}`, t('usage.cost')]} />
                <Line type="monotone" dataKey="totalCost" stroke="#FF9500" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
            )}
          </div>
        </motion.div>
      </div>

      {/* Provider Usage Breakdown */}
      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3 }} className="card">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('user_dashboard.usage_by_provider')}</h2>
        {providerUsage.length === 0 ? (
          <div className="h-40 flex items-center justify-center text-apple-gray-400">
            <div className="text-center">
              <ServerStackIcon className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p className="text-sm">{t('user_dashboard.no_provider_data')}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {providerUsage.slice(0, 8).map((provider: any, index: number) => {
              const maxReqs = Math.max(...providerUsage.map((p: any) => p.requests));
              return (
                <div key={provider.providerName} className="flex items-center gap-4">
                  <div className="flex items-center gap-3 w-40 flex-shrink-0">
                    <div className="w-3 h-3 rounded-full flex-shrink-0" style={{ backgroundColor: COLORS[index % COLORS.length] }} />
                    <span className="font-medium text-apple-gray-900 text-sm truncate">{provider.providerName}</span>
                  </div>
                  <div className="flex-1 h-6 bg-apple-gray-100 dark:bg-[var(--theme-bg-input)] rounded-full overflow-hidden">
                    <div
                      className="h-full rounded-full transition-all"
                      style={{ width: `${(provider.requests / maxReqs) * 100}%`, backgroundColor: COLORS[index % COLORS.length] }}
                    />
                  </div>
                  <div className="text-right w-32 flex-shrink-0">
                    <p className="text-sm font-medium text-apple-gray-900">{formatNumber(provider.requests)} {t('user_dashboard.req')}</p>
                    <p className="text-xs text-apple-gray-500">{formatCurrency(provider.cost)}</p>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </motion.div>
    </div>
  );
}

export default UserDashboardPage;
