/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  ClockIcon,
  ChartBarIcon,
  ExclamationTriangleIcon,
  ServerIcon,
  ArrowPathIcon
} from '@heroicons/react/24/outline';
import { SYSTEM_SLA_QUERY } from '@/lib/graphql/operations/health';

function SlaDashboardPage() {
  const [timeWindow, setTimeWindow] = useState<number>(24);
  const { data, loading, error, refetch, networkStatus } = useQuery<any>(SYSTEM_SLA_QUERY, {
    variables: { hours: timeWindow },
    notifyOnNetworkStatusChange: true,
    fetchPolicy: 'cache-and-network',
  });

  const isRefreshing = networkStatus === 4;
  const sla = data?.systemSla;

  if (loading && !sla) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4 bg-red-50 text-apple-red rounded-apple border border-red-100">
        Failed to load SLA metrics: {error.message}
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">SLA Dashboard</h1>
          <p className="text-apple-gray-500 mt-1">Platform telemetry and service-level agreements</p>
        </div>
        <div className="flex items-center gap-4">
          <select
            value={timeWindow}
            onChange={(e) => setTimeWindow(Number(e.target.value))}
            className="input py-2 text-sm"
          >
            <option value={1}>Last 1 Hour</option>
            <option value={24}>Last 24 Hours</option>
            <option value={168}>Last 7 Days</option>
            <option value={720}>Last 30 Days</option>
          </select>
          <button
            onClick={() => refetch()}
            className="btn btn-secondary"
            disabled={isRefreshing}
          >
            <ArrowPathIcon className={`w-5 h-5 mr-2 ${isRefreshing ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {/* Total Requests */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <div className="flex items-center gap-3 mb-2">
            <div className="w-10 h-10 rounded-full bg-blue-50 flex items-center justify-center">
              <ChartBarIcon className="w-5 h-5 text-apple-blue" />
            </div>
            <p className="text-sm font-medium text-apple-gray-500">Platform Throughput</p>
          </div>
          <p className="text-3xl font-semibold text-apple-gray-900 mt-4">
            {sla?.totalRequests.toLocaleString() ?? 0}
          </p>
          <p className="text-xs text-apple-gray-400 mt-1">requests in window</p>
        </motion.div>

        {/* Failure Rate */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card">
          <div className="flex items-center gap-3 mb-2">
            <div className={`w-10 h-10 rounded-full flex items-center justify-center ${(sla?.failureRate ?? 0) > 0.05 ? 'bg-red-50' : 'bg-green-50'}`}>
              <ExclamationTriangleIcon className={`w-5 h-5 ${(sla?.failureRate ?? 0) > 0.05 ? 'text-apple-red' : 'text-apple-green'}`} />
            </div>
            <p className="text-sm font-medium text-apple-gray-500">Error Rate</p>
          </div>
          <p className="text-3xl font-semibold text-apple-gray-900 mt-4">
            {((sla?.failureRate ?? 0) * 100).toFixed(2)}%
          </p>
          <p className="text-xs text-apple-gray-400 mt-1">
            failed / total requests
          </p>
        </motion.div>

        {/* Latency */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2 }} className="card md:col-span-2 lg:col-span-2">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-full bg-purple-50 flex items-center justify-center">
              <ClockIcon className="w-5 h-5 text-purple-500" />
            </div>
            <p className="text-sm font-medium text-apple-gray-500">Request Latency (ms)</p>
          </div>
          <div className="grid grid-cols-3 gap-4 mt-2 border-t border-apple-gray-100 pt-4">
            <div>
              <p className="text-xs text-apple-gray-400 uppercase tracking-wider mb-1">Average</p>
              <p className="text-xl font-medium text-apple-gray-900">{sla?.avgLatencyMs.toFixed(0) ?? 0}<span className="text-sm text-apple-gray-400 ml-1">ms</span></p>
            </div>
            <div>
              <p className="text-xs text-apple-gray-400 uppercase tracking-wider mb-1">P95</p>
              <p className="text-xl font-medium text-apple-gray-900">{sla?.p95LatencyMs.toFixed(0) ?? 0}<span className="text-sm text-apple-gray-400 ml-1">ms</span></p>
            </div>
            <div>
              <p className="text-xs text-apple-gray-400 uppercase tracking-wider mb-1">P99</p>
              <p className="text-xl font-medium text-apple-orange">{sla?.p99LatencyMs.toFixed(0) ?? 0}<span className="text-sm text-apple-gray-400 ml-1">ms</span></p>
            </div>
          </div>
        </motion.div>

        {/* Provider Availability */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3 }} className="card md:col-span-2 lg:col-span-4">
           <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-green-50 flex items-center justify-center">
                <ServerIcon className="w-5 h-5 text-apple-green" />
              </div>
              <p className="text-sm font-medium text-apple-gray-500">Upstream Health Availability</p>
            </div>
          </div>
          <div className="bg-apple-gray-50 p-4 rounded-apple border border-apple-gray-100 flex items-center justify-between">
            <div>
              <p className="text-2xl font-semibold text-apple-gray-900">
                {sla?.healthyProviders ?? 0} <span className="text-lg text-apple-gray-500 font-normal">/ {sla?.activeProviders ?? 0}</span>
              </p>
              <p className="text-sm text-apple-gray-500 mt-1">Active providers passing health checks</p>
            </div>
            <div className="text-right">
              {sla?.activeProviders === 0 ? (
                <span className="text-sm text-apple-gray-400">No active providers</span>
              ) : sla?.healthyProviders === sla?.activeProviders ? (
                <span className="inline-flex items-center rounded-full bg-green-50 px-2.5 py-1 text-sm font-medium text-apple-green ring-1 ring-inset ring-green-600/20">All Healthy</span>
              ) : (
                <span className="inline-flex items-center rounded-full bg-red-50 px-2.5 py-1 text-sm font-medium text-apple-red ring-1 ring-inset ring-red-600/10">Degraded</span>
              )}
            </div>
          </div>
        </motion.div>
      </div>
    </div>
  );
}

export default SlaDashboardPage;
