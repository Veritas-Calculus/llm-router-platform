import { useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  ShieldExclamationIcon,
  ServerIcon,
  KeyIcon,
  BoltIcon,
} from '@heroicons/react/24/outline';
import { PROVIDERS_QUERY } from '@/lib/graphql/operations/providers';
import { SUBSCRIPTION_QUOTA_QUERY } from '@/lib/graphql/operations/billing';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

function RateLimitDashboardPage() {
  const { t } = useTranslation();
  const { data: provData, loading } = useQuery<any>(PROVIDERS_QUERY);
  const providers = useMemo(() => provData?.providers || [], [provData]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  // Aggregate stats
  const totalProviders = providers.length;
  const activeProviders = providers.filter((p: any) => p.isActive).length;
  const avgRateLimit = totalProviders
    ? Math.round(providers.reduce((s: number, p: any) => s + (p.rateLimit || 0), 0) / totalProviders)
    : 0;
  const maxRateLimit = Math.max(...providers.map((p: any) => p.rateLimit || 0), 0);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">{t('rate_limits.title')}</h1>
        <p className="text-apple-gray-500 mt-1">
          {t('rate_limits.subtitle')}
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: t('rate_limits.active_providers'), value: `${activeProviders}/${totalProviders}`, icon: ServerIcon, color: 'bg-blue-50 text-apple-blue' },
          { label: t('rate_limits.avg_rate_limit'), value: `${avgRateLimit} ${t('rate_limits.rpm')}`, icon: ShieldExclamationIcon, color: 'bg-green-50 text-green-600' },
          { label: t('rate_limits.max_rate_limit'), value: `${maxRateLimit} ${t('rate_limits.rpm')}`, icon: ShieldExclamationIcon, color: 'bg-purple-50 text-purple-600' },
          { label: t('rate_limits.api_keys_label'), value: t('rate_limits.per_key_limits'), icon: KeyIcon, color: 'bg-orange-50 text-orange-600' },
        ].map((card, i) => (
          <motion.div
            key={card.label}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: i * 0.05 }}
            className="card p-5"
          >
            <div className="flex items-center gap-3">
              <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${card.color}`}>
                <card.icon className="w-5 h-5" />
              </div>
              <div>
                <p className="text-xs text-apple-gray-500">{card.label}</p>
                <p className="text-lg font-bold text-apple-gray-900">{card.value}</p>
              </div>
            </div>
          </motion.div>
        ))}
      </div>
      {/* Subscription Plan Quota */}
      <SubscriptionQuotaSection />

      {/* Provider rate limits */}
      <div className="card overflow-hidden">
        <div className="px-6 py-4 border-b border-apple-gray-100">
          <h2 className="text-base font-semibold text-apple-gray-900">{t('rate_limits.provider_rate_limits')}</h2>
          <p className="text-xs text-apple-gray-500 mt-0.5">{t('rate_limits.provider_rate_limits_desc')}</p>
        </div>
        <div className="divide-y divide-apple-gray-100">
          {providers
            .sort((a: any, b: any) => (b.rateLimit || 0) - (a.rateLimit || 0))
            .map((provider: any, i: number) => {
              const pct = maxRateLimit > 0 ? ((provider.rateLimit || 0) / maxRateLimit) * 100 : 0;
              return (
                <motion.div
                  key={provider.id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ delay: i * 0.03 }}
                  className="px-6 py-4"
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-3">
                      <span className={`w-2 h-2 rounded-full ${provider.isActive ? 'bg-apple-green' : 'bg-apple-gray-300'}`} />
                      <span className="font-medium text-sm text-apple-gray-900">{provider.name}</span>
                      <span className="text-xs text-apple-gray-400 font-mono">{provider.type}</span>
                    </div>
                    <div className="flex items-center gap-4 text-sm">
                      <span className="text-apple-gray-600 font-semibold">
                        {provider.rateLimit || 0} <span className="text-xs text-apple-gray-400 font-normal">{t('rate_limits.rpm')}</span>
                      </span>
                      <span className="text-apple-gray-400 text-xs">
                        {t('rate_limits.weight_label', { value: provider.weight })}
                      </span>
                    </div>
                  </div>
                  <div className="h-2 bg-apple-gray-100 rounded-full overflow-hidden">
                    <motion.div
                      initial={{ width: 0 }}
                      animate={{ width: `${pct}%` }}
                      transition={{ duration: 0.6, delay: i * 0.05 }}
                      className={`h-full rounded-full ${
                        provider.isActive
                          ? pct > 80 ? 'bg-gradient-to-r from-apple-blue to-purple-500'
                            : pct > 40 ? 'bg-apple-blue'
                            : 'bg-apple-blue/60'
                          : 'bg-apple-gray-300'
                      }`}
                    />
                  </div>
                </motion.div>
              );
            })}
        </div>
      </div>

      {/* Info card */}
      <div className="card p-5 bg-blue-50/50 border-blue-100">
        <div className="flex items-start gap-3">
          <ShieldExclamationIcon className="w-5 h-5 text-apple-blue shrink-0 mt-0.5" />
          <div className="text-sm text-apple-gray-700">
            <p className="font-medium text-apple-gray-900 mb-1">{t('rate_limits.enforcement_title')}</p>
            <ul className="space-y-1 text-apple-gray-600">
              <li>• <strong>{t('rate_limits.enforcement_subscription')}</strong> {t('rate_limits.enforcement_subscription_desc')}</li>
              <li>• <strong>{t('rate_limits.enforcement_provider')}</strong> {t('rate_limits.enforcement_provider_desc')}</li>
              <li>• <strong>{t('rate_limits.enforcement_api_key')}</strong> {t('rate_limits.enforcement_api_key_desc')}</li>
              <li>• <strong>{t('rate_limits.enforcement_graphql')}</strong> {t('rate_limits.enforcement_graphql_desc')}</li>
              <li>• <strong>{t('rate_limits.enforcement_circuit')}</strong> {t('rate_limits.enforcement_circuit_desc')}</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

export default RateLimitDashboardPage;

/* eslint-disable @typescript-eslint/no-explicit-any */
function SubscriptionQuotaSection() {
  const { t } = useTranslation();
  const { data } = useQuery<any>(SUBSCRIPTION_QUOTA_QUERY, { fetchPolicy: 'cache-and-network' });
  const sub = data?.mySubscription;
  if (!sub || sub.tokenLimit <= 0) return null;

  const pct = sub.quotaPercentage;
  const exceeded = sub.isQuotaExceeded;
  const near = pct >= 80 && !exceeded;
  const barColor = exceeded ? 'bg-red-500' : near ? 'bg-orange-400' : 'bg-apple-blue';
  const bgColor = exceeded ? 'bg-red-50 border-red-200' : near ? 'bg-orange-50 border-orange-200' : 'bg-blue-50/50 border-blue-200';
  const textColor = exceeded ? 'text-red-700' : near ? 'text-orange-700' : 'text-apple-blue';
  const fmt = (n: number) => n >= 1000000 ? `${(n / 1000000).toFixed(1)}M` : n >= 1000 ? `${(n / 1000).toFixed(1)}K` : `${n}`;

  return (
    <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className={`card p-5 border ${bgColor}`}>
      <div className="flex items-center gap-3 mb-3">
        <BoltIcon className={`w-5 h-5 ${textColor}`} />
        <h2 className={`text-base font-semibold ${textColor}`}>{t('rate_limits.subscription_quota_title')}</h2>
      </div>
      <div className="flex items-center justify-between mb-2">
        <span className={`text-sm font-medium ${textColor}`}>{sub.planName} {t('rate_limits.plan_suffix')}</span>
        <span className={`text-sm font-semibold ${textColor}`}>
          {fmt(sub.usedTokens)} / {fmt(sub.tokenLimit)}{exceeded && ` ${t('rate_limits.exceeded')}`}
        </span>
      </div>
      <div className="h-2.5 bg-white/60 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${barColor} transition-all duration-500`} style={{ width: `${Math.min(pct, 100)}%` }} />
      </div>
      {exceeded && (
        <p className="text-xs text-red-600 mt-2">{t('rate_limits.quota_exceeded_desc')}</p>
      )}
    </motion.div>
  );
}
