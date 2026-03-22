import { useQuery } from '@apollo/client/react';
import { useTranslation } from '@/lib/i18n';
import { SYSTEM_LOAD_QUERY } from '@/lib/graphql/operations/health';
import {
  BoltIcon,
  CircleStackIcon,
  ServerStackIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';

interface ServiceLoad {
  requestsInFlight: number;
  requestsPerSecond: number;
  avgLatencyMs: number;
  p95LatencyMs: number;
  errorRate: number;
}

interface DatabaseLoad {
  activeConnections: number;
  maxConnections: number;
  poolIdle: number;
  poolInUse: number;
  transactionsPerSecond: number;
  cacheHitRate: number;
  deadlocks: number;
}

interface RedisLoad {
  connectedClients: number;
  usedMemoryMB: number;
  maxMemoryMB: number;
  opsPerSecond: number;
  hitRate: number;
  keyCount: number;
}

interface SystemLoadData {
  systemLoad: {
    service: ServiceLoad;
    database: DatabaseLoad;
    redis: RedisLoad;
  };
}

function MetricCard({ label, value, unit, color }: { label: string; value: string | number; unit?: string; color?: string }) {
  return (
    <div className="text-center">
      <div className="text-xs text-apple-gray-400 mb-1">{label}</div>
      <div className={`text-xl font-semibold ${color || 'text-apple-gray-900'}`}>
        {typeof value === 'number' ? (Number.isInteger(value) ? value : value.toFixed(2)) : value}
      </div>
      {unit && <div className="text-[10px] text-apple-gray-400">{unit}</div>}
    </div>
  );
}

function ProgressBar({ value, max, color }: { value: number; max: number; color: string }) {
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  return (
    <div className="w-full bg-apple-gray-100 rounded-full h-2.5 overflow-hidden">
      <div className={`h-full rounded-full transition-all duration-500 ${color}`} style={{ width: `${pct}%` }} />
    </div>
  );
}

export default function SystemLoadPanel() {
  const { t } = useTranslation();
  const { data, loading, error, refetch } = useQuery<SystemLoadData>(SYSTEM_LOAD_QUERY, {
    pollInterval: 5000,
  });

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center py-20 text-apple-gray-400">
        <svg className="animate-spin h-6 w-6 mr-3" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        {t('common.loading')}
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-20 text-red-500">
        <p>{t('common.error')}: {error.message}</p>
      </div>
    );
  }

  const load = data?.systemLoad;
  if (!load) return null;

  const { service, database, redis } = load;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-apple-gray-900">{t('monitoring.load_monitoring')}</h2>
          <p className="text-sm text-apple-gray-400 mt-1">{t('monitoring.load_monitoring_desc')}</p>
        </div>
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-apple-gray-600 bg-white border border-apple-gray-200 rounded-xl hover:bg-apple-gray-50 transition-colors shadow-sm"
        >
          <ArrowPathIcon className="w-4 h-4" />
          {t('common.refresh')}
        </button>
      </div>

      {/* Service Load */}
      <div className="bg-white rounded-2xl border border-apple-gray-200 shadow-sm p-6">
        <h3 className="text-base font-semibold text-apple-gray-900 mb-4 flex items-center gap-2">
          <BoltIcon className="w-5 h-5 text-blue-500" />
          {t('monitoring.service_load')}
        </h3>
        <div className="grid grid-cols-5 gap-6">
          <MetricCard
            label={t('monitoring.in_flight')}
            value={service.requestsInFlight}
            color={service.requestsInFlight > 50 ? 'text-amber-600' : 'text-apple-gray-900'}
          />
          <MetricCard
            label={t('monitoring.rps')}
            value={service.requestsPerSecond}
            unit="req/s"
          />
          <MetricCard
            label={t('monitoring.avg_latency')}
            value={service.avgLatencyMs}
            unit="ms"
          />
          <MetricCard
            label={t('monitoring.p95_latency')}
            value={service.p95LatencyMs}
            unit="ms"
            color={service.p95LatencyMs > 1000 ? 'text-red-600' : 'text-apple-gray-900'}
          />
          <MetricCard
            label={t('monitoring.error_rate')}
            value={service.errorRate}
            unit="%"
            color={service.errorRate > 5 ? 'text-red-600' : service.errorRate > 1 ? 'text-amber-600' : 'text-green-600'}
          />
        </div>
      </div>

      {/* Database & Redis side by side */}
      <div className="grid grid-cols-2 gap-6">
        {/* Database Load */}
        <div className="bg-white rounded-2xl border border-apple-gray-200 shadow-sm p-6">
          <h3 className="text-base font-semibold text-apple-gray-900 mb-4 flex items-center gap-2">
            <CircleStackIcon className="w-5 h-5 text-green-500" />
            {t('monitoring.database_load')}
          </h3>

          {/* Connection Pool */}
          <div className="mb-4">
            <div className="flex justify-between text-xs text-apple-gray-500 mb-1.5">
              <span>{t('monitoring.connection_pool')}</span>
              <span>{database.poolInUse} / {database.maxConnections}</span>
            </div>
            <ProgressBar
              value={database.poolInUse}
              max={database.maxConnections}
              color={database.poolInUse / Math.max(database.maxConnections, 1) > 0.8 ? 'bg-red-500' : 'bg-blue-500'}
            />
          </div>

          <div className="grid grid-cols-2 gap-4 mt-4">
            <MetricCard label={t('monitoring.active_conn')} value={database.activeConnections} />
            <MetricCard label={t('monitoring.idle_conn')} value={database.poolIdle} />
            <MetricCard label={t('monitoring.tps')} value={database.transactionsPerSecond} unit="tx/s" />
            <MetricCard
              label={t('monitoring.cache_hit')}
              value={database.cacheHitRate}
              unit="%"
              color={database.cacheHitRate > 95 ? 'text-green-600' : database.cacheHitRate > 80 ? 'text-amber-600' : 'text-red-600'}
            />
            <MetricCard
              label={t('monitoring.deadlocks')}
              value={database.deadlocks}
              color={database.deadlocks > 0 ? 'text-red-600' : 'text-green-600'}
            />
          </div>
        </div>

        {/* Redis Load */}
        <div className="bg-white rounded-2xl border border-apple-gray-200 shadow-sm p-6">
          <h3 className="text-base font-semibold text-apple-gray-900 mb-4 flex items-center gap-2">
            <ServerStackIcon className="w-5 h-5 text-purple-500" />
            {t('monitoring.redis_load')}
          </h3>

          {/* Memory Bar */}
          <div className="mb-4">
            <div className="flex justify-between text-xs text-apple-gray-500 mb-1.5">
              <span>{t('monitoring.memory_usage')}</span>
              <span>
                {redis.usedMemoryMB.toFixed(1)} MB
                {redis.maxMemoryMB > 0 ? ` / ${redis.maxMemoryMB.toFixed(0)} MB` : ''}
              </span>
            </div>
            <ProgressBar
              value={redis.usedMemoryMB}
              max={redis.maxMemoryMB > 0 ? redis.maxMemoryMB : redis.usedMemoryMB * 2}
              color={redis.maxMemoryMB > 0 && redis.usedMemoryMB / redis.maxMemoryMB > 0.8 ? 'bg-red-500' : 'bg-purple-500'}
            />
          </div>

          <div className="grid grid-cols-2 gap-4 mt-4">
            <MetricCard label={t('monitoring.clients')} value={redis.connectedClients} />
            <MetricCard label={t('monitoring.ops_sec')} value={redis.opsPerSecond} unit="ops/s" />
            <MetricCard
              label={t('monitoring.hit_rate')}
              value={redis.hitRate}
              unit="%"
              color={redis.hitRate > 90 ? 'text-green-600' : redis.hitRate > 50 ? 'text-amber-600' : 'text-red-600'}
            />
            <MetricCard label={t('monitoring.key_count')} value={redis.keyCount} />
          </div>
        </div>
      </div>
    </div>
  );
}
