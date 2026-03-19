import { useState, useEffect, useRef, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  BellIcon,
  BellAlertIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  XMarkIcon,
  EyeIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { ALERTS_QUERY, ACKNOWLEDGE_ALERT, RESOLVE_ALERT } from '@/lib/graphql/operations';
import type { Alert } from '@/lib/types';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface NotificationCenterProps {
  /** Poll interval in ms (default: 60000 = 1 minute) */
  pollInterval?: number;
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
    return d.toLocaleDateString();
  };

  const statusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-red-100 text-red-700';
      case 'acknowledged':
        return 'bg-yellow-100 text-yellow-700';
      case 'resolved':
        return 'bg-green-100 text-green-700';
      default:
        return 'bg-gray-100 text-gray-700';
    }
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
          <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white px-1">
            {activeCount > 99 ? '99+' : activeCount}
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
            className="absolute right-0 top-full mt-2 w-80 sm:w-96 rounded-2xl overflow-hidden z-50"
            style={{
              backgroundColor: 'var(--theme-bg-card)',
              border: '1px solid var(--theme-border-light)',
              boxShadow: 'var(--theme-shadow-lg)',
            }}
          >
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid var(--theme-border-light)' }}>
              <h3 className="font-semibold" style={{ color: 'var(--theme-text)' }}>Notifications</h3>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setFilter('active')}
                  className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                    filter === 'active' ? 'bg-apple-blue text-white' : 'hover:bg-[var(--theme-bg-hover)]'
                  }`}
                  style={filter !== 'active' ? { color: 'var(--theme-text-secondary)' } : undefined}
                >
                  Active
                </button>
                <button
                  onClick={() => setFilter('all')}
                  className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                    filter === 'all' ? 'bg-apple-blue text-white' : 'hover:bg-[var(--theme-bg-hover)]'
                  }`}
                  style={filter !== 'all' ? { color: 'var(--theme-text-secondary)' } : undefined}
                >
                  All
                </button>
              </div>
            </div>

            {/* Alert List */}
            <div className="max-h-80 overflow-y-auto">
              {loading && alerts.length === 0 ? (
                <div className="flex items-center justify-center py-8">
                  <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-apple-blue" />
                </div>
              ) : alerts.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-8 text-center" style={{ color: 'var(--theme-text-muted)' }}>
                  <CheckCircleIcon className="w-10 h-10 mb-2 opacity-40" />
                  <p className="text-sm font-medium">All clear</p>
                  <p className="text-xs mt-1">No {filter === 'active' ? 'active ' : ''}alerts</p>
                </div>
              ) : (
                alerts.map((alert) => (
                  <div
                    key={alert.id}
                    className="px-4 py-3 transition-colors hover:bg-[var(--theme-bg-hover)]"
                    style={{ borderBottom: '1px solid var(--theme-border-light)' }}
                  >
                    <div className="flex items-start gap-3">
                      <div className="mt-0.5">
                        <ExclamationTriangleIcon
                          className={`w-5 h-5 ${
                            alert.status === 'active' ? 'text-red-500' : alert.status === 'acknowledged' ? 'text-yellow-500' : 'text-green-500'
                          }`}
                        />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-0.5">
                          <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase ${statusColor(alert.status)}`}>
                            {alert.status}
                          </span>
                          <span className="text-[11px]" style={{ color: 'var(--theme-text-muted)' }}>
                            {formatTime(alert.created_at)}
                          </span>
                        </div>
                        <p className="text-sm leading-snug" style={{ color: 'var(--theme-text)' }}>
                          {alert.message || `${alert.alert_type} on ${alert.target_type}`}
                        </p>
                        <p className="text-xs mt-0.5" style={{ color: 'var(--theme-text-muted)' }}>
                          {alert.target_type} · {alert.alert_type}
                        </p>

                        {/* Actions */}
                        {alert.status === 'active' && (
                          <div className="flex gap-2 mt-2">
                            <button
                              onClick={() => handleAcknowledge(alert.id)}
                              className="flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-lg bg-yellow-50 text-yellow-700 hover:bg-yellow-100 transition-colors"
                            >
                              <EyeIcon className="w-3.5 h-3.5" />
                              Acknowledge
                            </button>
                            <button
                              onClick={() => handleResolve(alert.id)}
                              className="flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-lg bg-green-50 text-green-700 hover:bg-green-100 transition-colors"
                            >
                              <CheckCircleIcon className="w-3.5 h-3.5" />
                              Resolve
                            </button>
                          </div>
                        )}
                        {alert.status === 'acknowledged' && (
                          <div className="mt-2">
                            <button
                              onClick={() => handleResolve(alert.id)}
                              className="flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-lg bg-green-50 text-green-700 hover:bg-green-100 transition-colors"
                            >
                              <CheckCircleIcon className="w-3.5 h-3.5" />
                              Resolve
                            </button>
                          </div>
                        )}
                      </div>
                      {alert.status !== 'active' && (
                        <XMarkIcon className="w-4 h-4 flex-shrink-0 opacity-30" />
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
