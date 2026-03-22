import { useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  ShieldExclamationIcon,
  ServerIcon,
  KeyIcon,
} from '@heroicons/react/24/outline';
import { PROVIDERS_QUERY } from '@/lib/graphql/operations/providers';

/* eslint-disable @typescript-eslint/no-explicit-any */

function RateLimitDashboardPage() {
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
        <h1 className="text-2xl font-semibold text-apple-gray-900">Rate Limit Overview</h1>
        <p className="text-apple-gray-500 mt-1">
          Monitor rate limits across providers and API keys
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: 'Active Providers', value: `${activeProviders}/${totalProviders}`, icon: ServerIcon, color: 'bg-blue-50 text-apple-blue' },
          { label: 'Avg Rate Limit', value: `${avgRateLimit} RPM`, icon: ShieldExclamationIcon, color: 'bg-green-50 text-green-600' },
          { label: 'Max Rate Limit', value: `${maxRateLimit} RPM`, icon: ShieldExclamationIcon, color: 'bg-purple-50 text-purple-600' },
          { label: 'API Keys', value: 'Per-key limits', icon: KeyIcon, color: 'bg-orange-50 text-orange-600' },
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

      {/* Provider rate limits */}
      <div className="card overflow-hidden">
        <div className="px-6 py-4 border-b border-apple-gray-100">
          <h2 className="text-base font-semibold text-apple-gray-900">Provider Rate Limits</h2>
          <p className="text-xs text-apple-gray-500 mt-0.5">Requests per minute allowed per provider</p>
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
                        {provider.rateLimit || 0} <span className="text-xs text-apple-gray-400 font-normal">RPM</span>
                      </span>
                      <span className="text-apple-gray-400 text-xs">
                        Weight: {provider.weight}
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
            <p className="font-medium text-apple-gray-900 mb-1">Rate Limit Enforcement</p>
            <ul className="space-y-1 text-apple-gray-600">
              <li>• <strong>Provider-level:</strong> Controls requests-per-minute to each LLM provider</li>
              <li>• <strong>API Key-level:</strong> Per-key rate limits configurable in API Keys page</li>
              <li>• <strong>GraphQL:</strong> Login/register limited to 5 req/min, password reset to 3 req/min</li>
              <li>• <strong>Circuit Breaker:</strong> Providers auto-disabled after 5 consecutive errors</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

export default RateLimitDashboardPage;
