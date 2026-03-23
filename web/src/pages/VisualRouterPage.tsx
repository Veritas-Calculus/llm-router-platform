import { useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  ServerIcon,
  ArrowRightIcon,
  SignalIcon,
  SignalSlashIcon,
  ArrowPathIcon,
  GlobeAltIcon,
} from '@heroicons/react/24/outline';
import { GET_ROUTING_RULES } from '@/lib/graphql/operations/routingRules';
import { PROVIDERS_QUERY } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

// ─── Helper: colour by provider index ───────────────────────────────

const PROVIDER_COLORS = [
  { bg: 'bg-blue-50', border: 'border-blue-200', text: 'text-blue-700', dot: 'bg-blue-500', line: '#3b82f6' },
  { bg: 'bg-purple-50', border: 'border-purple-200', text: 'text-purple-700', dot: 'bg-purple-500', line: '#a855f7' },
  { bg: 'bg-emerald-50', border: 'border-emerald-200', text: 'text-emerald-700', dot: 'bg-emerald-500', line: '#10b981' },
  { bg: 'bg-amber-50', border: 'border-amber-200', text: 'text-amber-700', dot: 'bg-amber-500', line: '#f59e0b' },
  { bg: 'bg-rose-50', border: 'border-rose-200', text: 'text-rose-700', dot: 'bg-rose-500', line: '#f43f5e' },
  { bg: 'bg-cyan-50', border: 'border-cyan-200', text: 'text-cyan-700', dot: 'bg-cyan-500', line: '#06b6d4' },
];

function getProviderColor(index: number) {
  const { t } = useTranslation();
  return PROVIDER_COLORS[index % PROVIDER_COLORS.length];
}

// ─── Provider Node Card ─────────────────────────────────────────────

function ProviderNode({ provider, index, ruleCount }: { provider: any; index: number; ruleCount: number }) {
  const c = getProviderColor(index);
  const weightPct = Math.min(100, Math.round((provider.weight || 1) * 100));

  return (
    <motion.div
      initial={{ opacity: 0, x: 20 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ delay: index * 0.08 }}
      className={`${c.bg} ${c.border} border rounded-xl p-4 relative`}
    >
      <div className="flex items-center gap-3 mb-3">
        <div className={`p-2 rounded-lg ${c.bg}`}>
          <ServerIcon className={`w-5 h-5 ${c.text}`} />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className={`text-sm font-semibold ${c.text} truncate`}>{provider.name}</h3>
          <p className="text-xs text-apple-gray-400 truncate">{provider.baseUrl}</p>
        </div>
        {provider.isActive ? (
          <SignalIcon className="w-4 h-4 text-apple-green flex-shrink-0" />
        ) : (
          <SignalSlashIcon className="w-4 h-4 text-apple-red flex-shrink-0" />
        )}
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs">
          <span className="text-apple-gray-500">Weight</span>
          <span className={`font-semibold ${c.text}`}>{provider.weight?.toFixed(1) ?? '1.0'}</span>
        </div>
        <div className="w-full bg-white/60 rounded-full h-1.5">
          <div className={`${c.dot} h-1.5 rounded-full transition-all`} style={{ width: `${weightPct}%` }} />
        </div>

        <div className="flex items-center justify-between text-xs mt-2">
          <span className="text-apple-gray-500">Priority</span>
          <span className={`font-medium ${c.text}`}>{provider.priority ?? 0}</span>
        </div>
        <div className="flex items-center justify-between text-xs">
          <span className="text-apple-gray-500">Routes</span>
          <span className={`font-medium ${c.text}`}>{ruleCount}</span>
        </div>
      </div>
    </motion.div>
  );
}

// ─── Rule Row ───────────────────────────────────────────────────────

function RuleRow({ rule, providerMap, providerColorMap }: { rule: any; providerMap: Map<string, any>; providerColorMap: Map<string, number> }) {
  const target = providerMap.get(rule.targetProviderId);
  const fallback = rule.fallbackProviderId ? providerMap.get(rule.fallbackProviderId) : null;
  const tc = getProviderColor(providerColorMap.get(rule.targetProviderId) ?? 0);
  const fc = fallback ? getProviderColor(providerColorMap.get(rule.fallbackProviderId) ?? 0) : null;

  return (
    <motion.div
      initial={{ opacity: 0, y: 5 }}
      animate={{ opacity: 1, y: 0 }}
      className={`flex items-center gap-3 p-3 rounded-lg border transition-all ${
        rule.isEnabled
          ? 'border-apple-gray-200 bg-white hover:shadow-sm'
          : 'border-dashed border-apple-gray-200 bg-apple-gray-50 opacity-60'
      }`}
    >
      {/* Pattern */}
      <code className="text-xs bg-apple-gray-100 text-apple-gray-800 px-2 py-1 rounded font-mono min-w-[120px]">
        {rule.modelPattern}
      </code>

      {/* Arrow */}
      <ArrowRightIcon className="w-4 h-4 text-apple-gray-300 flex-shrink-0" />

      {/* Target */}
      <span className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium ${tc.bg} ${tc.text} ${tc.border} border`}>
        <span className={`w-1.5 h-1.5 rounded-full ${tc.dot}`} />
        {target?.name || 'Unknown'}
      </span>

      {/* Fallback */}
      {fallback && fc && (
        <>
          <div className="flex items-center gap-1 text-xs text-apple-gray-400">
            <ArrowPathIcon className="w-3 h-3" />
            <span>fallback</span>
          </div>
          <span className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium ${fc.bg} ${fc.text} ${fc.border} border`}>
            <span className={`w-1.5 h-1.5 rounded-full ${fc.dot}`} />
            {fallback.name}
          </span>
        </>
      )}

      {/* Priority badge */}
      <span className="ml-auto text-xs text-apple-gray-400">
        P{rule.priority}
      </span>

      {/* Status */}
      <span
        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
          rule.isEnabled
            ? 'bg-green-50 text-apple-green ring-1 ring-green-600/20'
            : 'bg-apple-gray-100 text-apple-gray-500'
        }`}
      >
        {rule.isEnabled ? 'Active' : 'Disabled'}
      </span>
    </motion.div>
  );
}

// ─── Main Page ──────────────────────────────────────────────────────

function VisualRouterPage() {
  const { data: providersData, loading: provLoading, refetch: refetchProviders } = useQuery<any>(PROVIDERS_QUERY);
  const { data: rulesData, loading: rulesLoading, refetch: refetchRules } = useQuery<any>(GET_ROUTING_RULES, {
    variables: { page: 1, pageSize: 200 },
  });

  const providers: any[] = useMemo(() => providersData?.providers || [], [providersData]);
  const rules: any[] = useMemo(
    () => (rulesData?.routingRules?.data || []).slice().sort((a: any, b: any) => b.priority - a.priority),
    [rulesData],
  );

  const providerMap = useMemo(() => {
    const m = new Map<string, any>();
    providers.forEach((p) => m.set(p.id, p));
    return m;
  }, [providers]);

  const providerColorMap = useMemo(() => {
    const m = new Map<string, number>();
    providers.forEach((p, i) => m.set(p.id, i));
    return m;
  }, [providers]);

  // Count rules targeting each provider
  const ruleCountByProvider = useMemo(() => {
    const m = new Map<string, number>();
    rules.forEach((r: any) => {
      m.set(r.targetProviderId, (m.get(r.targetProviderId) || 0) + 1);
    });
    return m;
  }, [rules]);

  // Traffic distribution (simplified: weight-proportional for active+enabled)
  const totalWeight = useMemo(() => {
    return providers.filter((p) => p.isActive).reduce((acc, p) => acc + (p.weight || 1), 0);
  }, [providers]);

  const loading = provLoading || rulesLoading;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Visual Router</h1>
          <p className="text-apple-gray-500 mt-1">
            Real-time routing topology and load distribution overview
          </p>
        </div>
        <button
          onClick={() => { refetchProviders(); refetchRules(); }}
          className="btn btn-secondary"
        >
          <ArrowPathIcon className="w-5 h-5" />
        </button>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="card p-4">
          <div className="text-xs text-apple-gray-500 mb-1">Total Providers</div>
          <div className="text-2xl font-semibold text-apple-gray-900">{providers.length}</div>
        </div>
        <div className="card p-4">
          <div className="text-xs text-apple-gray-500 mb-1">Active Providers</div>
          <div className="text-2xl font-semibold text-apple-green">
            {providers.filter((p) => p.isActive).length}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-xs text-apple-gray-500 mb-1">Routing Rules</div>
          <div className="text-2xl font-semibold text-apple-gray-900">{rules.length}</div>
        </div>
        <div className="card p-4">
          <div className="text-xs text-apple-gray-500 mb-1">Active Rules</div>
          <div className="text-2xl font-semibold text-apple-blue">
            {rules.filter((r: any) => r.isEnabled).length}
          </div>
        </div>
      </div>

      {/* Flow Diagram: Request → Router → Providers */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-6">Traffic Flow</h2>
        <div className="grid grid-cols-1 md:grid-cols-[200px_1fr_1fr] gap-6 items-start">
          {/* Entry Node */}
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            className="card p-5 text-center border-2 border-dashed border-apple-gray-300"
          >
            <GlobeAltIcon className="w-10 h-10 text-apple-gray-400 mx-auto mb-2" />
            <div className="text-sm font-semibold text-apple-gray-900">Incoming Request</div>
            <div className="text-xs text-apple-gray-500 mt-1">/v1/chat/completions</div>
          </motion.div>

          {/* Router Engine */}
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: 0.1 }}
            className="card p-5 border-2 border-apple-blue/30 bg-blue-50/30"
          >
            <div className="text-center mb-4">
              <div className="inline-flex items-center gap-2 bg-apple-blue text-white rounded-full px-4 py-1.5 text-sm font-semibold">
                <ArrowPathIcon className="w-4 h-4" />
                Routing Engine
              </div>
              <p className="text-xs text-apple-gray-500 mt-2">
                Priority-based rule matching with weighted load balancing
              </p>
            </div>

            {/* Rule List */}
            <div className="space-y-2">
              {rules.length === 0 ? (
                <div className="text-center py-6 text-apple-gray-400 text-sm">
                  No routing rules configured. Requests use default provider selection.
                </div>
              ) : (
                rules.map((r: any) => (
                  <RuleRow
                    key={r.id}
                    rule={r}
                    providerMap={providerMap}
                    providerColorMap={providerColorMap}
                  />
                ))
              )}
            </div>
          </motion.div>

          {/* Provider Nodes */}
          <div className="space-y-3">
            <div className="text-xs font-semibold text-apple-gray-500 uppercase tracking-wider mb-2">
              Providers ({providers.length})
            </div>
            {providers.map((p: any, i: number) => (
              <ProviderNode
                key={p.id}
                provider={p}
                index={i}
                ruleCount={ruleCountByProvider.get(p.id) || 0}
              />
            ))}
          </div>
        </div>
      </div>

      {/* Weight Distribution */}
      {totalWeight > 0 && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Load Distribution</h2>
          <div className="flex rounded-xl overflow-hidden h-10 bg-apple-gray-100">
            {providers
              .filter((p) => p.isActive)
              .map((p, i) => {
                const pct = ((p.weight || 1) / totalWeight) * 100;
                const c = getProviderColor(providerColorMap.get(p.id) ?? i);
                return (
                  <motion.div
                    key={p.id}
                    initial={{ width: 0 }}
                    animate={{ width: `${pct}%` }}
                    transition={{ delay: 0.3 + i * 0.05, duration: 0.5 }}
                    className={`${c.dot} flex items-center justify-center relative group cursor-default`}
                    title={`${p.name}: ${pct.toFixed(1)}%`}
                  >
                    {pct > 8 && (
                      <span className="text-white text-xs font-semibold truncate px-1">
                        {p.name} ({pct.toFixed(0)}%)
                      </span>
                    )}
                  </motion.div>
                );
              })}
          </div>
          <div className="flex flex-wrap gap-4 mt-4">
            {providers
              .filter((p) => p.isActive)
              .map((p, i) => {
                const pct = ((p.weight || 1) / totalWeight) * 100;
                const c = getProviderColor(providerColorMap.get(p.id) ?? i);
                return (
                  <div key={p.id} className="flex items-center gap-2 text-xs">
                    <span className={`w-2.5 h-2.5 rounded-full ${c.dot}`} />
                    <span className="text-apple-gray-700 font-medium">{p.name}</span>
                    <span className="text-apple-gray-400">{pct.toFixed(1)}%</span>
                  </div>
                );
              })}
          </div>
        </div>
      )}
    </div>
  );
}

export default VisualRouterPage;
