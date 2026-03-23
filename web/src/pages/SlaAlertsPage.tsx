import { useState, useEffect } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import {
  BellAlertIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  ArrowPathIcon,
  ShieldCheckIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import {
  ALERTS_QUERY,
  ALERT_CONFIG_QUERY,
  UPDATE_ALERT_CONFIG,
  ACKNOWLEDGE_ALERT,
  RESOLVE_ALERT,
} from '@/lib/graphql/operations/health';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

// "global" target uses a zero UUID for platform-wide rules
const GLOBAL_TARGET_TYPE = 'global';
const GLOBAL_TARGET_ID = '00000000-0000-0000-0000-000000000000';

// ─── Alert Rules Configuration ──────────────────────────────────────

interface AlertRuleFormState {
  isEnabled: boolean;
  failureThreshold: string;
  errorRateThreshold: string;
  latencyThresholdMs: string;
  budgetThreshold: string;
  cooldownMinutes: string;
  webhookUrl: string;
  email: string;
}

const defaultForm: AlertRuleFormState = {
  isEnabled: true,
  failureThreshold: '3',
  errorRateThreshold: '5',
  latencyThresholdMs: '5000',
  budgetThreshold: '90',
  cooldownMinutes: '5',
  webhookUrl: '',
  email: '',
};

function AlertRulesCard() {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<any>(ALERT_CONFIG_QUERY, {
    variables: { targetType: GLOBAL_TARGET_TYPE, targetId: GLOBAL_TARGET_ID },
    fetchPolicy: 'cache-and-network',
  });

  const [updateConfig, { loading: saving }] = useMutation(UPDATE_ALERT_CONFIG);
  const [form, setForm] = useState<AlertRuleFormState>(defaultForm);

  useEffect(() => {
    const cfg = data?.alertConfig;
    if (cfg) {
      setForm({
        isEnabled: cfg.isEnabled ?? true,
        failureThreshold: String(cfg.failureThreshold ?? 3),
        errorRateThreshold: String(((cfg.errorRateThreshold ?? 0) * 100).toFixed(0)),
        latencyThresholdMs: String(cfg.latencyThresholdMs ?? 0),
        budgetThreshold: String(((cfg.budgetThreshold ?? 0) * 100).toFixed(0)),
        cooldownMinutes: String(cfg.cooldownMinutes ?? 5),
        webhookUrl: cfg.webhookUrl ?? '',
        email: cfg.email ?? '',
      });
    }
  }, [data]);

  const handleSave = async () => {
    try {
      await updateConfig({
        variables: {
          input: {
            targetType: GLOBAL_TARGET_TYPE,
            targetId: GLOBAL_TARGET_ID,
            isEnabled: form.isEnabled,
            failureThreshold: parseInt(form.failureThreshold, 10) || 3,
            errorRateThreshold: (parseFloat(form.errorRateThreshold) || 0) / 100,
            latencyThresholdMs: parseInt(form.latencyThresholdMs, 10) || 0,
            budgetThreshold: (parseFloat(form.budgetThreshold) || 0) / 100,
            cooldownMinutes: parseInt(form.cooldownMinutes, 10) || 5,
            webhookUrl: form.webhookUrl || null,
            email: form.email || null,
          },
        },
      });
      await refetch();
      toast.success('Alert rules saved');
    } catch {
      toast.error('Failed to save alert rules');
    }
  };

  if (loading && !data) {
    return (
      <div className="card animate-pulse h-48" />
    );
  }

  return (
    <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-blue-50 flex items-center justify-center">
            <ShieldCheckIcon className="w-5 h-5 text-apple-blue" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-apple-gray-900">Global Alert Rules</h2>
            <p className="text-sm text-apple-gray-500">Platform-wide SLA monitoring thresholds</p>
          </div>
        </div>
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={form.isEnabled}
            onChange={(e) => setForm({ ...form, isEnabled: e.target.checked })}
            className="sr-only peer"
          />
          <div className="w-11 h-6 bg-apple-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-apple-blue" />
          <span className="ml-3 text-sm font-medium text-apple-gray-700">
            {form.isEnabled ? 'Enabled' : 'Disabled'}
          </span>
        </label>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        {/* Health Failure Threshold */}
        <div>
          <label className="label">Consecutive Failures</label>
          <input
            type="number"
            value={form.failureThreshold}
            onChange={(e) => setForm({ ...form, failureThreshold: e.target.value })}
            className="input mt-1 w-full"
            placeholder="3"
          />
          <p className="text-xs text-apple-gray-400 mt-1">Alert after N consecutive health check failures</p>
        </div>

        {/* Error Rate */}
        <div>
          <label className="label">Error Rate Threshold (%)</label>
          <input
            type="number"
            value={form.errorRateThreshold}
            onChange={(e) => setForm({ ...form, errorRateThreshold: e.target.value })}
            className="input mt-1 w-full"
            placeholder="5"
            step="0.1"
          />
          <p className="text-xs text-apple-gray-400 mt-1">Alert when error rate exceeds this percentage</p>
        </div>

        {/* Latency */}
        <div>
          <label className="label">P95 Latency Threshold (ms)</label>
          <input
            type="number"
            value={form.latencyThresholdMs}
            onChange={(e) => setForm({ ...form, latencyThresholdMs: e.target.value })}
            className="input mt-1 w-full"
            placeholder="5000"
          />
          <p className="text-xs text-apple-gray-400 mt-1">Alert when P95 latency exceeds this value</p>
        </div>

        {/* Budget */}
        <div>
          <label className="label">Budget Alert Threshold (%)</label>
          <input
            type="number"
            value={form.budgetThreshold}
            onChange={(e) => setForm({ ...form, budgetThreshold: e.target.value })}
            className="input mt-1 w-full"
            placeholder="90"
          />
          <p className="text-xs text-apple-gray-400 mt-1">Alert when budget consumption exceeds this %</p>
        </div>

        {/* Cooldown */}
        <div>
          <label className="label">Cooldown (minutes)</label>
          <input
            type="number"
            value={form.cooldownMinutes}
            onChange={(e) => setForm({ ...form, cooldownMinutes: e.target.value })}
            className="input mt-1 w-full"
            placeholder="5"
          />
          <p className="text-xs text-apple-gray-400 mt-1">Minimum interval between duplicate alerts</p>
        </div>
      </div>

      <div className="border-t border-apple-gray-100 mt-6 pt-6">
        <h3 className="text-sm font-semibold text-apple-gray-700 mb-4">Notification Channels</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          <div>
            <label className="label">Webhook URL</label>
            <input
              type="url"
              value={form.webhookUrl}
              onChange={(e) => setForm({ ...form, webhookUrl: e.target.value })}
              className="input mt-1 w-full font-mono text-sm"
              placeholder="https://hooks.slack.com/..."
            />
          </div>
          <div>
            <label className="label">Email</label>
            <input
              type="email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              className="input mt-1 w-full"
              placeholder="ops-team@company.com"
            />
          </div>
        </div>
      </div>

      <div className="flex justify-end mt-6">
        <button onClick={handleSave} className="btn btn-primary" disabled={saving}>
          {saving ? 'Saving...' : 'Save Rules'}
        </button>
      </div>
    </motion.div>
  );
}

// ─── Active Alerts Table ────────────────────────────────────────────

const statusColors: Record<string, string> = {
  active: 'bg-red-50 text-apple-red ring-red-600/10',
  acknowledged: 'bg-orange-50 text-apple-orange ring-orange-600/10',
  resolved: 'bg-green-50 text-apple-green ring-green-600/10',
};

function formatDate(d: string): string {
  return new Date(d).toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function ActiveAlertsTable() {
  const [statusFilter, setStatusFilter] = useState<string>('active');
  const { data, loading, refetch, networkStatus } = useQuery<any>(ALERTS_QUERY, {
    variables: { status: statusFilter || null },
    fetchPolicy: 'cache-and-network',
    notifyOnNetworkStatusChange: true,
  });

  const [ackMut] = useMutation(ACKNOWLEDGE_ALERT);
  const [resolveMut] = useMutation(RESOLVE_ALERT);
  const [bulkClearing, setBulkClearing] = useState(false);
  const isRefreshing = networkStatus === 4;

  const alerts = data?.alerts?.data || [];
  const total = data?.alerts?.total || 0;
  const unresolved = alerts.filter((a: any) => a.status !== 'resolved');

  const handleAck = async (id: string) => {
    try {
      await ackMut({ variables: { id } });
      toast.success('Alert acknowledged');
      await refetch();
    } catch {
      toast.error('Failed to acknowledge alert');
    }
  };

  const handleResolve = async (id: string) => {
    try {
      await resolveMut({ variables: { id } });
      toast.success('Alert resolved');
      await refetch();
    } catch {
      toast.error('Failed to resolve alert');
    }
  };

  const handleBulkClear = async () => {
    if (unresolved.length === 0) return;
    setBulkClearing(true);
    try {
      await Promise.all(
        unresolved.map((a: any) => resolveMut({ variables: { id: a.id } }))
      );
      toast.success(`${unresolved.length} alert${unresolved.length > 1 ? 's' : ''} resolved`);
      await refetch();
    } catch {
      toast.error('Failed to resolve some alerts');
    } finally {
      setBulkClearing(false);
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.1 }}
      className="card"
    >
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-red-50 flex items-center justify-center">
            <BellAlertIcon className="w-5 h-5 text-apple-red" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-apple-gray-900">Active Alerts</h2>
            <p className="text-sm text-apple-gray-500">{total} alert{total !== 1 ? 's' : ''} total</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="input py-2 text-sm"
          >
            <option value="">{t('sla_alerts.all_statuses')}</option>
            <option value="active">{t('sla_alerts.active')}</option>
            <option value="acknowledged">{t('sla_alerts.acknowledged')}</option>
            <option value="resolved">{t('sla_alerts.resolved')}</option>
          </select>
          {unresolved.length > 0 && (
            <button
              onClick={handleBulkClear}
              className="btn btn-secondary text-sm text-apple-red hover:text-red-700"
              disabled={bulkClearing}
            >
              <CheckCircleIcon className="w-4 h-4 mr-1" />
              {bulkClearing ? 'Clearing...' : `Clear All (${unresolved.length})`}
            </button>
          )}
          <button
            onClick={() => refetch()}
            className="btn btn-secondary"
            disabled={isRefreshing}
          >
            <ArrowPathIcon className={`w-5 h-5 ${isRefreshing ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {loading && !data ? (
        <div className="flex items-center justify-center h-32">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
        </div>
      ) : alerts.length === 0 ? (
        <div className="text-center py-12">
          <CheckCircleIcon className="w-12 h-12 text-apple-green mx-auto mb-3" />
          <h3 className="text-lg font-semibold text-apple-gray-900">All Clear</h3>
          <p className="text-sm text-apple-gray-500 mt-1">
            No {statusFilter || ''} alerts at this time.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-apple-gray-200">
            <thead>
              <tr>
                <th className="table-header">Type</th>
                <th className="table-header">Target</th>
                <th className="table-header">Message</th>
                <th className="table-header">Status</th>
                <th className="table-header">Created</th>
                <th className="table-header">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {alerts.map((alert: any) => (
                <tr key={alert.id} className="hover:bg-apple-gray-50">
                  <td className="table-cell">
                    <div className="flex items-center gap-2">
                      <ExclamationTriangleIcon className="w-4 h-4 text-apple-orange" />
                      <span className="text-sm font-medium">{alert.alertType}</span>
                    </div>
                  </td>
                  <td className="table-cell">
                    <span className="text-xs font-mono bg-apple-gray-100 px-2 py-1 rounded">
                      {alert.targetType}:{alert.targetId?.substring(0, 8)}
                    </span>
                  </td>
                  <td className="table-cell text-sm text-apple-gray-700 max-w-xs truncate">
                    {alert.message}
                  </td>
                  <td className="table-cell">
                    <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset ${statusColors[alert.status] || 'bg-apple-gray-100 text-apple-gray-600'}`}>
                      {alert.status}
                    </span>
                  </td>
                  <td className="table-cell text-sm text-apple-gray-500">
                    {formatDate(alert.createdAt)}
                  </td>
                  <td className="table-cell">
                    <div className="flex items-center gap-2">
                      {alert.status === 'active' && (
                        <button
                          onClick={() => handleAck(alert.id)}
                          className="text-xs text-apple-orange hover:text-orange-700 font-medium transition-colors"
                        >
                          Acknowledge
                        </button>
                      )}
                      {(alert.status === 'active' || alert.status === 'acknowledged') && (
                        <button
                          onClick={() => handleResolve(alert.id)}
                          className="text-xs text-apple-green hover:text-green-700 font-medium transition-colors"
                        >
                          Resolve
                        </button>
                      )}
                      {alert.status === 'resolved' && (
                        <span className="text-xs text-apple-gray-400">—</span>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </motion.div>
  );
}

// ─── Main Page ──────────────────────────────────────────────────────

function SlaAlertsPage() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Alert Rules</h1>
        <p className="text-apple-gray-500 mt-1">
          Configure SLA thresholds and notification channels for platform-wide monitoring
        </p>
      </div>

      <AlertRulesCard />
      <ActiveAlertsTable />
    </div>
  );
}

export default SlaAlertsPage;
