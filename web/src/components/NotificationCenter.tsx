import { useState, useEffect, useRef, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  BellIcon,
  BellAlertIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  EyeIcon,
  ExclamationCircleIcon,
  ShieldExclamationIcon,
  CpuChipIcon,
  ServerStackIcon,
  ClockIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { ALERTS_QUERY, ACKNOWLEDGE_ALERT, RESOLVE_ALERT } from '@/lib/graphql/operations';
import type { Alert } from '@/lib/types';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface NotificationCenterProps {
  /** Poll interval in ms (default: 60000 = 1 minute) */
  pollInterval?: number;
}

/* ── Utilities ── */

const alertTypeLabels: Record<string, string> = {
  high_error_rate: 'High Error Rate',
  high_latency: 'High Latency',
  provider_down: 'Provider Down',
  rate_limit: 'Rate Limited',
  quota_exceeded: 'Quota Exceeded',
  circuit_open: 'Circuit Breaker Open',
  sla_breach: 'SLA Breach',
  health_check_failed: 'Health Check Failed',
};

const targetTypeLabels: Record<string, string> = {
  provider: 'Provider',
  model: 'Model',
  api_key: 'API Key',
  system: 'System',
  endpoint: 'Endpoint',
};

function getAlertTypeLabel(alertType: string): string {
  return alertTypeLabels[alertType] || alertType.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}

function getTargetLabel(targetType: string): string {
  return targetTypeLabels[targetType] || targetType.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}

function getAlertIcon(alertType: string) {
  if (alertType.includes('error') || alertType.includes('failed')) return ExclamationCircleIcon;
  if (alertType.includes('latency') || alertType.includes('sla')) return ClockIcon;
  if (alertType.includes('security') || alertType.includes('shield')) return ShieldExclamationIcon;
  if (alertType.includes('provider') || alertType.includes('circuit')) return ServerStackIcon;
  if (alertType.includes('rate') || alertType.includes('quota')) return CpuChipIcon;
  return ExclamationTriangleIcon;
}

function getSeverityConfig(status: string) {
  switch (status) {
    case 'active':
      return {
        dot: 'bg-red-500',
        badge: 'bg-red-500/10 text-red-600 dark:text-red-400',
        label: 'Active',
        iconColor: 'text-red-500',
      };
    case 'acknowledged':
      return {
        dot: 'bg-yellow-500',
        badge: 'bg-yellow-500/10 text-yellow-600 dark:text-yellow-400',
        label: 'Acknowledged',
        iconColor: 'text-yellow-500',
      };
    case 'resolved':
      return {
        dot: 'bg-green-500',
        badge: 'bg-green-500/10 text-green-600 dark:text-green-400',
        label: 'Resolved',
        iconColor: 'text-green-500',
      };
    default:
      return {
        dot: 'bg-gray-400',
        badge: 'bg-gray-400/10 text-gray-600 dark:text-gray-400',
        label: status,
        iconColor: 'text-gray-400',
      };
  }
}

export default function NotificationCenter({ pollInterval = 60000 }: NotificationCenterProps) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState<'active' | 'all'>('active');
  const panelRef = useRef<HTMLDivElement>(null);

  const { data, loading, refetch } = useQuery<any>(ALERTS_QUERY, {
    variables: { status: filter === 'active' ? 'active' : undefined },
    pollInterval,
  });
  const alerts: Alert[] = useMemo(() =>
    (data?.alerts?.data || []).map((a: any) => ({
      id: a.id, target_type: a.targetType, target_id: a.targetId,
      alert_type: a.alertType, message: a.message, status: a.status,
      resolved_at: a.resolvedAt, acknowledged_at: a.acknowledgedAt, created_at: a.createdAt,
    })),
  [data]);

  const [acknowledgeMut] = useMutation(ACKNOWLEDGE_ALERT);
  const [resolveMut] = useMutation(RESOLVE_ALERT);

  const activeCount = alerts.filter((a) => a.status === 'active').length;

  // Close on click outside
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open]);

  const handleAcknowledge = async (id: string) => {
    try {
      await acknowledgeMut({ variables: { id } });
      refetch();
    } catch {
      // silent
    }
  };

  const handleResolve = async (id: string) => {
    try {
      await resolveMut({ variables: { id } });
      refetch();
    } catch {
      // silent
    }
  };

  const formatTime = (ts: string) => {
    const d = new Date(ts);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    const diffDay = Math.floor(diffHr / 24);
    if (diffDay < 7) return `${diffDay}d ago`;
    return d.toLocaleDateString();
  };

  const BellComp = activeCount > 0 ? BellAlertIcon : BellIcon;

  return (
    <div ref={panelRef} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="relative p-2 rounded-apple transition-colors hover:bg-[var(--theme-bg-hover)]"
        style={{ color: 'var(--theme-text-secondary)' }}
        aria-label="Notifications"
      >
        <BellComp className="w-5 h-5" />
        {activeCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white px-1 animate-pulse">
            {activeCount > 9 ? '9+' : activeCount}
          </span>
        )}
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -8, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -8, scale: 0.95 }}
            transition={{ duration: 0.15 }}
            className="absolute right-0 top-full mt-2 w-80 sm:w-[420px] rounded-2xl overflow-hidden z-50"
            style={{
              backgroundColor: 'var(--theme-bg-card)',
              border: '1px solid var(--theme-border-light)',
              boxShadow: 'var(--theme-shadow-lg)',
            }}
          >
            {/* Header */}
            <div className="flex items-center justify-between px-5 py-3.5" style={{ borderBottom: '1px solid var(--theme-border-light)' }}>
              <div className="flex items-center gap-2.5">
                <BellIcon className="w-5 h-5" style={{ color: 'var(--theme-text-secondary)' }} />
                <h3 className="font-semibold text-[15px]" style={{ color: 'var(--theme-text)' }}>Notifications</h3>
                {activeCount > 0 && (
                  <span className="flex items-center justify-center h-5 min-w-5 px-1.5 rounded-full bg-red-500/10 text-red-600 dark:text-red-400 text-[11px] font-semibold">
                    {activeCount}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-1 p-0.5 rounded-xl" style={{ backgroundColor: 'var(--theme-bg-hover)' }}>
                <button
                  onClick={() => setFilter('active')}
                  className={`px-3 py-1 rounded-lg text-xs font-medium transition-all ${
                    filter === 'active' ? 'bg-apple-blue text-white shadow-sm' : ''
                  }`}
                  style={filter !== 'active' ? { color: 'var(--theme-text-secondary)' } : undefined}
                >
                  Active
                </button>
                <button
                  onClick={() => setFilter('all')}
                  className={`px-3 py-1 rounded-lg text-xs font-medium transition-all ${
                    filter === 'all' ? 'bg-apple-blue text-white shadow-sm' : ''
                  }`}
                  style={filter !== 'all' ? { color: 'var(--theme-text-secondary)' } : undefined}
                >
                  All
                </button>
              </div>
            </div>

            {/* Alert List */}
            <div className="max-h-96 overflow-y-auto">
              {loading && alerts.length === 0 ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-apple-blue" />
                </div>
              ) : alerts.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 text-center" style={{ color: 'var(--theme-text-muted)' }}>
                  <CheckCircleIcon className="w-12 h-12 mb-3 opacity-30" />
                  <p className="text-sm font-medium">All clear</p>
                  <p className="text-xs mt-1 opacity-70">No {filter === 'active' ? 'active ' : ''}alerts right now</p>
                </div>
              ) : (
                <div className="py-1">
                  {alerts.map((alert) => {
                    const severity = getSeverityConfig(alert.status);
                    const AlertIcon = getAlertIcon(alert.alert_type);
                    return (
                      <div
                        key={alert.id}
                        className="px-4 py-3 mx-1.5 my-1 rounded-xl transition-colors hover:bg-[var(--theme-bg-hover)] group"
                      >
                        <div className="flex items-start gap-3">
                          {/* Icon */}
                          <div className={`mt-0.5 p-1.5 rounded-lg ${
                            alert.status === 'active' ? 'bg-red-500/10' :
                            alert.status === 'acknowledged' ? 'bg-yellow-500/10' : 'bg-green-500/10'
                          }`}>
                            <AlertIcon className={`w-4 h-4 ${severity.iconColor}`} />
                          </div>

                          {/* Content */}
                          <div className="flex-1 min-w-0">
                            {/* Title row */}
                            <div className="flex items-center gap-2 mb-1">
                              <span className={`px-2 py-0.5 rounded-md text-[10px] font-semibold ${severity.badge}`}>
                                {severity.label}
                              </span>
                              <span className="text-[11px] flex items-center gap-1" style={{ color: 'var(--theme-text-muted)' }}>
                                <ClockIcon className="w-3 h-3" />
                                {formatTime(alert.created_at)}
                              </span>
                            </div>

                            {/* Alert type as human-readable title */}
                            <p className="text-sm font-medium leading-snug mb-0.5" style={{ color: 'var(--theme-text)' }}>
                              {getAlertTypeLabel(alert.alert_type)}
                            </p>

                            {/* Message */}
                            {alert.message && (
                              <p className="text-xs leading-relaxed line-clamp-2" style={{ color: 'var(--theme-text-secondary)' }}>
                                {alert.message}
                              </p>
                            )}

                            {/* Target info */}
                            <div className="flex items-center gap-1.5 mt-1.5">
                              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium"
                                style={{
                                  backgroundColor: 'var(--theme-bg-hover)',
                                  color: 'var(--theme-text-muted)',
                                }}>
                                {getTargetLabel(alert.target_type)}
                              </span>
                              {alert.target_id && (
                                <span className="text-[10px] font-mono truncate max-w-[120px]" style={{ color: 'var(--theme-text-muted)' }}>
                                  {alert.target_id.substring(0, 8)}...
                                </span>
                              )}
                            </div>

                            {/* Actions */}
                            {alert.status === 'active' && (
                              <div className="flex gap-2 mt-2.5">
                                <button
                                  onClick={() => handleAcknowledge(alert.id)}
                                  className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg transition-colors"
                                  style={{
                                    backgroundColor: 'var(--theme-bg-hover)',
                                    color: 'var(--theme-text-secondary)',
                                  }}
                                >
                                  <EyeIcon className="w-3.5 h-3.5" />
                                  Acknowledge
                                </button>
                                <button
                                  onClick={() => handleResolve(alert.id)}
                                  className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg bg-green-500/10 text-green-600 dark:text-green-400 hover:bg-green-500/20 transition-colors"
                                >
                                  <CheckCircleIcon className="w-3.5 h-3.5" />
                                  Resolve
                                </button>
                              </div>
                            )}
                            {alert.status === 'acknowledged' && (
                              <div className="mt-2.5">
                                <button
                                  onClick={() => handleResolve(alert.id)}
                                  className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg bg-green-500/10 text-green-600 dark:text-green-400 hover:bg-green-500/20 transition-colors"
                                >
                                  <CheckCircleIcon className="w-3.5 h-3.5" />
                                  Resolve
                                </button>
                              </div>
                            )}
                          </div>

                          {/* Resolved indicator */}
                          {alert.status === 'resolved' && (
                            <CheckCircleIcon className="w-4 h-4 flex-shrink-0 text-green-500 opacity-50 mt-1" />
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Footer */}
            {alerts.length > 0 && (
              <div className="px-5 py-3 text-center" style={{ borderTop: '1px solid var(--theme-border-light)' }}>
                <p className="text-[11px]" style={{ color: 'var(--theme-text-muted)' }}>
                  Showing {alerts.length} alert{alerts.length !== 1 ? 's' : ''}
                </p>
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
