import { useQuery } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { useTranslation } from '@/lib/i18n';
import { SYSTEM_STATUS_QUERY } from '@/lib/graphql/operations/health';
import {
  CheckCircleIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
  ArrowPathIcon,
  ServerStackIcon,
  CpuChipIcon,
  CircleStackIcon,
  ClockIcon,
  CubeIcon,
  CommandLineIcon,
  SignalIcon,
} from '@heroicons/react/24/outline';

interface DependencyStatus {
  name: string;
  status: string;
  latencyMs: number;
  version?: string;
  details?: string;
}

interface SystemStatusData {
  systemStatus: {
    overallStatus: string;
    service: {
      version: string;
      gitCommit: string;
      buildTime: string;
      uptime: string;
      configMode: string;
    };
    runtime: {
      goroutines: number;
      heapAllocMB: number;
      heapSysMB: number;
      gcPauseMs: number;
      numGC: number;
      cpuCores: number;
    };
    dependencies: DependencyStatus[];
  };
}

function statusColor(status: string) {
  switch (status) {
    case 'healthy': return 'text-apple-green';
    case 'degraded': return 'text-apple-orange';
    case 'critical': return 'text-apple-red';
    case 'unhealthy': return 'text-apple-red';
    default: return 'text-apple-gray-400';
  }
}

function statusBg(status: string) {
  switch (status) {
    case 'healthy': return 'bg-green-500/10 border-green-500/20';
    case 'degraded': return 'bg-orange-500/10 border-orange-500/20';
    case 'critical': return 'bg-red-500/10 border-red-500/20';
    default: return 'bg-apple-gray-100 border-apple-gray-200';
  }
}

function StatusIcon({ status }: { status: string }) {
  switch (status) {
    case 'healthy':
      return <CheckCircleIcon className="w-5 h-5 text-apple-green" />;
    case 'unhealthy':
    case 'critical':
      return <XCircleIcon className="w-5 h-5 text-apple-red" />;
    case 'degraded':
      return <ExclamationTriangleIcon className="w-5 h-5 text-apple-orange" />;
    default:
      return <ExclamationTriangleIcon className="w-5 h-5 text-apple-gray-400" />;
  }
}

function depIcon(name: string) {
  switch (name) {
    case 'postgres': return <CircleStackIcon className="w-5 h-5 text-blue-500" />;
    case 'redis': return <CubeIcon className="w-5 h-5 text-red-500" />;
    default: return <ServerStackIcon className="w-5 h-5 text-apple-gray-400" />;
  }
}

function parseDetails(details?: string): Record<string, string> | null {
  if (!details) return null;
  try {
    return JSON.parse(details);
  } catch {
    return null;
  }
}

export default function SystemStatusPanel() {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<SystemStatusData>(SYSTEM_STATUS_QUERY, {
    pollInterval: 10000,
    fetchPolicy: 'network-only',
  });

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  const status = data?.systemStatus;
  if (!status) {
    return <div className="text-center text-apple-gray-500 py-8">{t('common.no_data')}</div>;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-apple-gray-900">{t('monitoring.system_status')}</h2>
          <p className="text-sm text-apple-gray-500 mt-0.5">{t('monitoring.system_status_desc')}</p>
        </div>
        <button
          onClick={() => refetch()}
          className="btn btn-secondary text-sm"
        >
          <ArrowPathIcon className="w-4 h-4 mr-1.5" />
          {t('common.refresh')}
        </button>
      </div>

      {/* Overall Status Banner */}
      <motion.div
        initial={{ opacity: 0, y: -10 }}
        animate={{ opacity: 1, y: 0 }}
        className={`flex items-center gap-3 p-4 rounded-xl border ${statusBg(status.overallStatus)}`}
      >
        <StatusIcon status={status.overallStatus} />
        <div>
          <span className={`text-lg font-semibold capitalize ${statusColor(status.overallStatus)}`}>
            {status.overallStatus === 'healthy'
              ? t('monitoring.all_operational')
              : status.overallStatus === 'degraded'
              ? t('monitoring.partial_degradation')
              : t('monitoring.system_critical')}
          </span>
        </div>
      </motion.div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Service Info Card */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.1 }}
          className="card"
        >
          <div className="flex items-center gap-2 mb-4">
            <CommandLineIcon className="w-5 h-5 text-apple-blue" />
            <h3 className="text-base font-medium text-apple-gray-900">{t('monitoring.service_info')}</h3>
          </div>
          <div className="space-y-3">
            <InfoRow label={t('monitoring.version')} value={status.service.version} />
            <InfoRow label={t('monitoring.uptime')} value={status.service.uptime} icon={<ClockIcon className="w-4 h-4 text-apple-gray-400" />} />
            <InfoRow label={t('monitoring.mode')} value={status.service.configMode} />
            <InfoRow label={t('monitoring.build_time')} value={status.service.buildTime} />
            <InfoRow label={t('monitoring.git_commit')} value={status.service.gitCommit.substring(0, 8)} mono />
          </div>
        </motion.div>

        {/* Go Runtime Card */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.2 }}
          className="card"
        >
          <div className="flex items-center gap-2 mb-4">
            <CpuChipIcon className="w-5 h-5 text-purple-500" />
            <h3 className="text-base font-medium text-apple-gray-900">{t('monitoring.go_runtime')}</h3>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <MetricBox label={t('monitoring.goroutines')} value={status.runtime.goroutines.toLocaleString()} />
            <MetricBox label={t('monitoring.heap_alloc')} value={`${status.runtime.heapAllocMB.toFixed(1)} MB`} />
            <MetricBox label={t('monitoring.heap_sys')} value={`${status.runtime.heapSysMB.toFixed(1)} MB`} />
            <MetricBox label={t('monitoring.gc_pause')} value={`${status.runtime.gcPauseMs.toFixed(2)} ms`} />
            <MetricBox label={t('monitoring.num_gc')} value={status.runtime.numGC.toLocaleString()} />
            <MetricBox label={t('monitoring.cpu_cores')} value={status.runtime.cpuCores.toString()} />
          </div>
        </motion.div>
      </div>

      {/* Dependencies */}
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.3 }}
        className="card"
      >
        <div className="flex items-center gap-2 mb-4">
          <SignalIcon className="w-5 h-5 text-apple-green" />
          <h3 className="text-base font-medium text-apple-gray-900">{t('monitoring.dependencies')}</h3>
        </div>
        <div className="space-y-3">
          {status.dependencies.map((dep: DependencyStatus) => {
            const details = parseDetails(dep.details);
            return (
              <div
                key={dep.name}
                className="flex items-center justify-between p-4 bg-apple-gray-50 dark:bg-white/5 rounded-xl"
              >
                <div className="flex items-center gap-3">
                  <StatusIcon status={dep.status} />
                  {depIcon(dep.name)}
                  <div>
                    <p className="font-medium text-apple-gray-900 capitalize">{dep.name}</p>
                    {dep.version && (
                      <p className="text-xs text-apple-gray-500 mt-0.5 truncate max-w-xs" title={dep.version}>
                        {dep.version.length > 60 ? dep.version.substring(0, 60) + '…' : dep.version}
                      </p>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-6">
                  <div className="text-right">
                    <p className="text-sm font-medium text-apple-gray-900">{dep.latencyMs.toFixed(0)} ms</p>
                    <p className="text-xs text-apple-gray-500">{t('monitoring.latency')}</p>
                  </div>
                  {details && dep.name === 'postgres' && (
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">
                        {details.inUse ?? '?'} / {details.maxOpen ?? '?'}
                      </p>
                      <p className="text-xs text-apple-gray-500">{t('monitoring.pool')}</p>
                    </div>
                  )}
                  {details && dep.name === 'redis' && details.usedMemory && (
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{details.usedMemory}</p>
                      <p className="text-xs text-apple-gray-500">{t('monitoring.memory')}</p>
                    </div>
                  )}
                  {details && dep.name === 'redis' && details.connectedClients && (
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{details.connectedClients}</p>
                      <p className="text-xs text-apple-gray-500">{t('monitoring.clients')}</p>
                    </div>
                  )}
                  <span className={`text-xs font-medium capitalize px-2 py-1 rounded-full ${
                    dep.status === 'healthy' ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400' :
                    dep.status === 'unhealthy' ? 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-400' :
                    'bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-400'
                  }`}>
                    {dep.status}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </motion.div>
    </div>
  );
}

function InfoRow({ label, value, icon, mono }: { label: string; value: string; icon?: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <div className="flex items-center gap-2 text-apple-gray-500">
        {icon}
        <span>{label}</span>
      </div>
      <span className={`text-apple-gray-900 ${mono ? 'font-mono text-xs' : 'font-medium'}`}>{value}</span>
    </div>
  );
}

function MetricBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="p-3 bg-apple-gray-50 dark:bg-white/5 rounded-lg">
      <p className="text-xs text-apple-gray-500 mb-1">{label}</p>
      <p className="text-lg font-semibold text-apple-gray-900">{value}</p>
    </div>
  );
}
