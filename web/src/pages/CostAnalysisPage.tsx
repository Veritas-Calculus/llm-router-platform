import { useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  CurrencyDollarIcon,
  ChartBarIcon,
  ArrowTrendingUpIcon,
  ServerStackIcon,
} from '@heroicons/react/24/outline';
import {
  MY_USAGE_SUMMARY,
  MY_DAILY_USAGE,
  MY_USAGE_BY_PROVIDER,
} from '@/lib/graphql/operations/usage';

/* eslint-disable @typescript-eslint/no-explicit-any */

function CostAnalysisPage() {
  const { data: summaryData, loading: l1 } = useQuery<any>(MY_USAGE_SUMMARY);
  const { data: dailyData, loading: l2 } = useQuery<any>(MY_DAILY_USAGE, { variables: { days: 30 } });
  const { data: providerData, loading: l3 } = useQuery<any>(MY_USAGE_BY_PROVIDER);

  const loading = l1 || l2 || l3;
  const summary = summaryData?.myUsageSummary;
  const daily = useMemo(() => dailyData?.myDailyUsage || [], [dailyData]);
  const byProvider = useMemo(() => {
    const items = providerData?.myUsageByProvider || [];
    return [...items].sort((a: any, b: any) => (b.totalCost || 0) - (a.totalCost || 0));
  }, [providerData]);

  const totalCost = summary?.totalCost || 0;
  const totalRequests = summary?.totalRequests || 0;
  const totalTokens = summary?.totalTokens || 0;
  const avgCostPerReq = totalRequests > 0 ? totalCost / totalRequests : 0;

  // Daily chart helpers
  const maxDailyCost = Math.max(...daily.map((d: any) => d.cost || 0), 0.01);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Cost Analysis</h1>
        <p className="text-apple-gray-500 mt-1">Breakdown of LLM costs by provider, model, and time period</p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: 'Total Cost', value: `$${totalCost.toFixed(2)}`, icon: CurrencyDollarIcon, color: 'bg-green-50 text-green-600' },
          { label: 'Total Requests', value: totalRequests.toLocaleString(), icon: ChartBarIcon, color: 'bg-blue-50 text-apple-blue' },
          { label: 'Total Tokens', value: totalTokens.toLocaleString(), icon: ArrowTrendingUpIcon, color: 'bg-purple-50 text-purple-600' },
          { label: 'Avg Cost/Req', value: `$${avgCostPerReq.toFixed(4)}`, icon: ServerStackIcon, color: 'bg-orange-50 text-orange-600' },
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

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Daily cost chart (bar chart) */}
        <div className="card overflow-hidden">
          <div className="px-6 py-4 border-b border-apple-gray-100">
            <h2 className="text-base font-semibold text-apple-gray-900">Daily Cost (Last 30 Days)</h2>
          </div>
          <div className="p-6">
            {daily.length === 0 ? (
              <div className="text-center py-8 text-apple-gray-400">No usage data</div>
            ) : (
              <div className="flex items-end gap-[2px] h-40">
                {daily.map((day: any, i: number) => {
                  const pct = maxDailyCost > 0 ? ((day.cost || 0) / maxDailyCost) * 100 : 0;
                  return (
                    <div key={i} className="flex-1 flex flex-col items-center justify-end group relative">
                      <div className="absolute -top-8 left-1/2 -translate-x-1/2 hidden group-hover:block bg-apple-gray-900 text-white text-[10px] px-2 py-1 rounded-lg whitespace-nowrap z-10">
                        {day.date}: ${(day.cost || 0).toFixed(3)}
                      </div>
                      <motion.div
                        initial={{ height: 0 }}
                        animate={{ height: `${Math.max(pct, 2)}%` }}
                        transition={{ duration: 0.5, delay: i * 0.02 }}
                        className="w-full bg-gradient-to-t from-apple-blue to-blue-400 rounded-t-sm hover:from-blue-600 hover:to-blue-400 transition-colors cursor-pointer"
                      />
                    </div>
                  );
                })}
              </div>
            )}
            <div className="flex justify-between mt-2 text-[10px] text-apple-gray-400">
              <span>{daily.length > 0 ? daily[0].date : ''}</span>
              <span>{daily.length > 0 ? daily[daily.length - 1].date : ''}</span>
            </div>
          </div>
        </div>

        {/* Cost by provider */}
        <div className="card overflow-hidden">
          <div className="px-6 py-4 border-b border-apple-gray-100">
            <h2 className="text-base font-semibold text-apple-gray-900">Cost by Provider</h2>
          </div>
          <div className="divide-y divide-apple-gray-100">
            {byProvider.length === 0 ? (
              <div className="p-8 text-center text-apple-gray-400">No provider data</div>
            ) : (
              byProvider.map((prov: any, i: number) => {
                const pct = totalCost > 0 ? ((prov.totalCost || 0) / totalCost) * 100 : 0;
                return (
                  <motion.div
                    key={prov.providerName}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: i * 0.05 }}
                    className="px-6 py-4"
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="font-medium text-sm text-apple-gray-900">{prov.providerName}</span>
                      <div className="flex items-center gap-3 text-sm">
                        <span className="font-semibold text-apple-gray-900">${(prov.totalCost || 0).toFixed(2)}</span>
                        <span className="text-xs text-apple-gray-400">{pct.toFixed(1)}%</span>
                      </div>
                    </div>
                    <div className="h-2 bg-apple-gray-100 rounded-full overflow-hidden">
                      <motion.div
                        initial={{ width: 0 }}
                        animate={{ width: `${pct}%` }}
                        transition={{ duration: 0.6, delay: i * 0.05 }}
                        className="h-full rounded-full bg-gradient-to-r from-green-400 to-emerald-500"
                      />
                    </div>
                    <div className="flex items-center gap-4 mt-1.5 text-[11px] text-apple-gray-400">
                      <span>{(prov.requests || 0).toLocaleString()} requests</span>
                      <span>{(prov.tokens || 0).toLocaleString()} tokens</span>
                    </div>
                  </motion.div>
                );
              })
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default CostAnalysisPage;
