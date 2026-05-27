/* eslint-disable @typescript-eslint/no-explicit-any */

import { useMemo, useState } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  ArrowTrendingUpIcon,
  BanknotesIcon,
  ChartBarIcon,
  CreditCardIcon,
  ReceiptRefundIcon,
  ServerStackIcon,
  WalletIcon,
} from '@heroicons/react/24/outline';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { useTranslation } from '@/lib/i18n';
import { ADMIN_FINANCIAL_DASHBOARD_QUERY } from '@/lib/graphql/operations/finance';
import { useVisibilityAwarePolling } from '@/hooks/useVisibilityAwarePolling';
import { moneyNumber, type MoneyValue } from '@/lib/format';

const dayOptions = [7, 30, 90] as const;

const tooltipStyle = {
  backgroundColor: '#fff',
  border: '1px solid #E8E8ED',
  borderRadius: '12px',
  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
};

const fmtCurrency = (v: MoneyValue): string =>
  new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(moneyNumber(v));

const fmtNum = (v: number): string =>
  new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(v || 0);

const fmtPct = (v: number): string =>
  `${new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(v || 0)}%`;

const fmtCompact = (v: number): string => {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return fmtNum(v);
};

function MetricCard({
  title,
  value,
  subtitle,
  icon: Icon,
  tone,
}: {
  title: string;
  value: string;
  subtitle?: string;
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
  tone: 'blue' | 'green' | 'orange' | 'purple' | 'red' | 'slate';
}) {
  const toneMap = {
    blue: 'bg-blue-50 text-apple-blue',
    green: 'bg-green-50 text-apple-green',
    orange: 'bg-orange-50 text-apple-orange',
    purple: 'bg-purple-50 text-purple-600',
    red: 'bg-red-50 text-apple-red',
    slate: 'bg-apple-gray-100 text-apple-gray-600',
  };

  return (
    <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="card">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm text-apple-gray-500">{title}</p>
          <p className="mt-2 text-2xl font-semibold text-apple-gray-900 truncate">{value}</p>
          {subtitle && <p className="mt-1 text-xs text-apple-gray-400">{subtitle}</p>}
        </div>
        <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${toneMap[tone]}`}>
          <Icon className="w-5 h-5" />
        </div>
      </div>
    </motion.div>
  );
}

function FinancePage() {
  const { t } = useTranslation();
  const [days, setDays] = useState<(typeof dayOptions)[number]>(30);
  const pollMs = 60_000;
  const queryResult = useQuery<any>(ADMIN_FINANCIAL_DASHBOARD_QUERY, {
    variables: { days },
    pollInterval: pollMs,
  });
  useVisibilityAwarePolling(queryResult, pollMs);
  const { data, loading, refetch } = queryResult;

  const finance = data?.adminFinancialDashboard;
  const daily = useMemo(() =>
    (finance?.daily || []).map((point: any) => ({
      ...point,
      cashRevenue: moneyNumber(point.cashRevenue),
      usageRevenue: moneyNumber(point.usageRevenue),
      providerCost: moneyNumber(point.providerCost),
      grossProfit: moneyNumber(point.grossProfit),
    })),
  [finance]);
  const payments = useMemo(() =>
    (finance?.paymentBreakdown || []).map((item: any) => ({ ...item, amount: moneyNumber(item.amount) })),
  [finance]);
  const providers = useMemo(() =>
    (finance?.providerBreakdown || []).map((provider: any) => ({
      ...provider,
      usageRevenue: moneyNumber(provider.usageRevenue),
      providerCost: moneyNumber(provider.providerCost),
      grossProfit: moneyNumber(provider.grossProfit),
    })),
  [finance]);

  if (loading && !finance) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('admin.finance.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('admin.finance.subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-xl border border-apple-gray-200 bg-apple-gray-100 p-1">
            {dayOptions.map((option) => (
              <button
                key={option}
                type="button"
                onClick={() => setDays(option)}
                className={`px-3 py-1.5 text-sm font-medium rounded-lg transition-colors ${
                  days === option ? 'bg-white text-apple-blue shadow-sm' : 'text-apple-gray-500 hover:text-apple-gray-900'
                }`}
              >
                {option}d
              </button>
            ))}
          </div>
          <button type="button" onClick={() => refetch()} className="btn-secondary">
            {t('common.refresh')}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
        <MetricCard
          title={t('admin.finance.net_cash_revenue')}
          value={fmtCurrency(finance?.netCashRevenue)}
          subtitle={`${t('admin.finance.cash_revenue')}: ${fmtCurrency(finance?.cashRevenue)}`}
          icon={BanknotesIcon}
          tone="green"
        />
        <MetricCard
          title={t('admin.finance.usage_revenue')}
          value={fmtCurrency(finance?.usageRevenue)}
          subtitle={`${fmtNum(finance?.paidOrders)} ${t('admin.finance.paid_orders')}`}
          icon={ArrowTrendingUpIcon}
          tone="blue"
        />
        <MetricCard
          title={t('admin.finance.provider_cost')}
          value={fmtCurrency(finance?.providerCost)}
          subtitle={`${t('admin.finance.gross_margin')}: ${fmtPct(finance?.grossMargin)}`}
          icon={ServerStackIcon}
          tone="orange"
        />
        <MetricCard
          title={t('admin.finance.gross_profit')}
          value={fmtCurrency(finance?.grossProfit)}
          subtitle={`${fmtNum(finance?.payingCustomers)} ${t('admin.finance.paying_customers')}`}
          icon={ChartBarIcon}
          tone={moneyNumber(finance?.grossProfit) >= 0 ? 'purple' : 'red'}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
        <MetricCard
          title={t('admin.finance.balance_liability')}
          value={fmtCurrency(finance?.outstandingBalance)}
          subtitle={t('admin.finance.balance_liability_subtitle')}
          icon={WalletIcon}
          tone="slate"
        />
        <MetricCard
          title={t('admin.finance.subscription_revenue')}
          value={fmtCurrency(finance?.subscriptionRevenue)}
          subtitle={`${fmtNum(finance?.activeSubscriptions)} ${t('admin.finance.active_subscriptions')}`}
          icon={CreditCardIcon}
          tone="green"
        />
        <MetricCard
          title={t('admin.finance.top_up_revenue')}
          value={fmtCurrency(finance?.topUpRevenue)}
          subtitle={`${t('admin.finance.arpu')}: ${fmtCurrency(finance?.arpu)}`}
          icon={WalletIcon}
          tone="blue"
        />
        <MetricCard
          title={t('admin.finance.adjustments')}
          value={fmtCurrency(moneyNumber(finance?.refundAmount) + moneyNumber(finance?.creditGrants))}
          subtitle={`${t('admin.finance.refunds')}: ${fmtCurrency(finance?.refundAmount)}`}
          icon={ReceiptRefundIcon}
          tone="red"
        />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="card xl:col-span-2">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.finance.daily_trend')}</h2>
          <div className="h-80" style={{ minHeight: '320px' }}>
            {daily.length === 0 ? (
              <div className="flex h-full items-center justify-center text-apple-gray-400">
                <p className="text-sm">{t('admin.finance.no_finance_data')}</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%" minHeight={320}>
                <LineChart data={daily}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                  <XAxis dataKey="date" stroke="#8E8E93" fontSize={12} tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
                  <YAxis stroke="#8E8E93" fontSize={12} tickFormatter={(v) => `$${fmtCompact(Number(v))}`} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(value, name) => [fmtCurrency(Number(value)), t(`admin.finance.${String(name)}`)]} />
                  <Line type="monotone" dataKey="cashRevenue" stroke="#34C759" strokeWidth={2} dot={false} />
                  <Line type="monotone" dataKey="usageRevenue" stroke="#007AFF" strokeWidth={2} dot={false} />
                  <Line type="monotone" dataKey="providerCost" stroke="#FF9500" strokeWidth={2} dot={false} />
                  <Line type="monotone" dataKey="grossProfit" stroke="#AF52DE" strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="card">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.finance.payment_breakdown')}</h2>
          <div className="h-80" style={{ minHeight: '320px' }}>
            {payments.length === 0 ? (
              <div className="flex h-full items-center justify-center text-apple-gray-400">
                <p className="text-sm">{t('admin.finance.no_finance_data')}</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%" minHeight={320}>
                <BarChart data={payments}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#E8E8ED" />
                  <XAxis dataKey="name" stroke="#8E8E93" fontSize={12} />
                  <YAxis stroke="#8E8E93" fontSize={12} tickFormatter={(v) => `$${fmtCompact(Number(v))}`} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(value) => [fmtCurrency(Number(value)), t('admin.finance.amount')]} />
                  <Bar dataKey="amount" fill="#34C759" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </motion.div>
      </div>

      <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="card">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('admin.finance.provider_unit_economics')}</h2>
        {providers.length === 0 ? (
          <div className="h-48 flex items-center justify-center text-apple-gray-400">
            <p className="text-sm">{t('admin.finance.no_finance_data')}</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-apple-gray-200">
                  <th className="py-3 pr-4 text-left font-semibold text-apple-gray-600">{t('admin.finance.provider')}</th>
                  <th className="py-3 px-4 text-right font-semibold text-apple-gray-600">{t('admin.finance.requests')}</th>
                  <th className="py-3 px-4 text-right font-semibold text-apple-gray-600">{t('admin.finance.usage_revenue')}</th>
                  <th className="py-3 px-4 text-right font-semibold text-apple-gray-600">{t('admin.finance.provider_cost')}</th>
                  <th className="py-3 px-4 text-right font-semibold text-apple-gray-600">{t('admin.finance.gross_profit')}</th>
                  <th className="py-3 pl-4 text-right font-semibold text-apple-gray-600">{t('admin.finance.gross_margin')}</th>
                </tr>
              </thead>
              <tbody>
                {providers.map((provider: any) => (
                  <tr key={provider.providerName} className="border-b border-apple-gray-100 hover:bg-apple-gray-50">
                    <td className="py-3 pr-4 font-medium text-apple-gray-900">{provider.providerName}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-700">{fmtNum(provider.requests)}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-700">{fmtCurrency(provider.usageRevenue)}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-700">{fmtCurrency(provider.providerCost)}</td>
                    <td className="py-3 px-4 text-right text-apple-gray-900">{fmtCurrency(provider.grossProfit)}</td>
                    <td className="py-3 pl-4 text-right text-apple-gray-900">{fmtPct(provider.grossMargin)}</td>
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

export default FinancePage;
