/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState as useReactState } from 'react';
import { motion } from 'framer-motion';
import { ArrowPathIcon, CheckCircleIcon, XCircleIcon, ExclamationTriangleIcon, ServerIcon, Cog6ToothIcon, CpuChipIcon, KeyIcon, GlobeAltIcon, BellSlashIcon } from '@heroicons/react/24/outline';
import { useHealth } from '@/hooks/useHealth';
import type { AlertConfig } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';

function getStatusIcon(status: string | boolean) {
  const { t } = useTranslation();
  if (typeof status === 'boolean') {
    return status
      ? <CheckCircleIcon className="w-5 h-5 text-apple-green" />
      : <XCircleIcon className="w-5 h-5 text-apple-red" />;
  }
  switch (status) {
    case 'healthy':
    case 'active':
      return <CheckCircleIcon className="w-5 h-5 text-apple-green" />;
    case 'unhealthy':
    case 'failed':
      return <XCircleIcon className="w-5 h-5 text-apple-red" />;
    default:
      return <ExclamationTriangleIcon className="w-5 h-5 text-apple-orange" />;
  }
}

function HealthPage() {
  const {
    apiKeyHealth,
    proxyHealth,
    providerHealth,
    alerts,
    alertConfigs,
    loading,
    refreshing,
    activeTab,
    setActiveTab,
    activeAlerts,
    refreshAll,
    checkApiKey,
    checkProxy,
    checkProvider,
    checkAllProviders,
    acknowledgeAlert,
    resolveAlert,
    saveAlertConfig,
    formatDate,
  } = useHealth();

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Health Monitor</h1>
          <p className="text-apple-gray-500 mt-1">Monitor the health of your API keys and proxies</p>
        </div>
        <button onClick={refreshAll} className="btn btn-secondary" disabled={refreshing}>
          <ArrowPathIcon className={`w-5 h-5 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      <div className="segmented-control">
        {(['providers', 'api-keys', 'proxies', 'alerts', 'config'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`segmented-control-item ${
              activeTab === tab ? 'segmented-control-item--active' : ''
            }`}
          >
            {tab === 'api-keys' ? 'API Keys' : tab === 'config' ? 'Alert Config' : tab.charAt(0).toUpperCase() + tab.slice(1)}
            {tab === 'config' && <Cog6ToothIcon className="w-4 h-4" />}
            {tab === 'alerts' && activeAlerts > 0 && (
              <span className="bg-apple-red text-white text-2xs px-1.5 py-0.5 rounded-full leading-none">{activeAlerts}</span>
            )}
          </button>
        ))}
      </div>

      {activeTab === 'providers' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-apple-gray-900">Provider Health</h3>
            <button onClick={checkAllProviders} className="btn btn-secondary text-sm">
              <ArrowPathIcon className="w-4 h-4 mr-1" /> Check All
            </button>
          </div>
          {providerHealth.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16">
              <CpuChipIcon className="w-12 h-12 text-apple-gray-300 mb-3" />
              <p className="text-lg font-medium text-apple-gray-700 mb-1">No active providers</p>
              <p className="text-sm text-apple-gray-400 mb-4">Configure a provider to start monitoring health</p>
              <a href="/admin/providers" className="btn btn-primary text-sm px-4 py-2">Add Provider</a>
            </div>
          ) : (
            <div className="space-y-4">
              {providerHealth.map((provider: any) => (
                <div key={provider.id} className="flex items-center justify-between p-4 bg-apple-gray-50 rounded-apple">
                  <div className="flex items-center gap-4">
                    {getStatusIcon(provider.is_healthy)}
                    <div>
                      <p className="font-medium text-apple-gray-900">{provider.name}</p>
                      <p className="text-sm text-apple-gray-500">{provider.base_url}</p>
                      {provider.error_message && <p className="text-xs text-apple-red mt-1">{provider.error_message}</p>}
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    {provider.use_proxy && (
                      <div className="flex items-center gap-1">
                        <ServerIcon className="w-4 h-4 text-apple-blue" />
                        <span className="text-xs text-apple-blue">Via Proxy</span>
                      </div>
                    )}
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{provider.response_time > 0 ? `${provider.response_time}ms` : '-'}</p>
                      <p className="text-xs text-apple-gray-500">Latency</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{(provider.success_rate * 100).toFixed(1)}%</p>
                      <p className="text-xs text-apple-gray-500">Success rate</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm text-apple-gray-500">{provider.last_check ? formatDate(provider.last_check) : 'Never'}</p>
                      <p className="text-xs text-apple-gray-400">Last checked</p>
                    </div>
                    <button onClick={() => checkProvider(provider.id)} className="btn btn-secondary text-sm px-3 py-1.5" title={t('health.check_now')}>
                      <ArrowPathIcon className="w-4 h-4 mr-1" />
                      Test
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}

      {activeTab === 'api-keys' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          {apiKeyHealth.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16">
              <KeyIcon className="w-12 h-12 text-apple-gray-300 mb-3" />
              <p className="text-lg font-medium text-apple-gray-700 mb-1">No API keys found</p>
              <p className="text-sm text-apple-gray-400">API keys will appear here once providers are configured</p>
            </div>
          ) : (
            <div className="space-y-4">
              {apiKeyHealth.map((key: any) => (
                <div key={key.id} className="flex items-center justify-between p-4 bg-apple-gray-50 rounded-apple">
                  <div className="flex items-center gap-4">
                    <ServerIcon className="w-5 h-5 text-apple-gray-400" />
                    <div>
                      <p className="font-medium text-apple-gray-900">{key.key_prefix}...</p>
                      <p className="text-sm text-apple-gray-500">{key.provider_name}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <p className={`text-sm font-medium ${key.is_active ? 'text-apple-green' : 'text-apple-gray-400'}`}>{key.is_active ? 'Active' : 'Inactive'}</p>
                      <p className="text-xs text-apple-gray-500">Status</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{key.response_time}ms</p>
                      <p className="text-xs text-apple-gray-500">Latency</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm text-apple-gray-500">{key.last_check ? formatDate(key.last_check) : 'Never'}</p>
                      <p className="text-xs text-apple-gray-400">Last checked</p>
                    </div>
                    <button onClick={() => checkApiKey(key.id)} className="btn btn-secondary text-sm px-3 py-1.5" title="Check now">
                      <ArrowPathIcon className="w-4 h-4 mr-1" />
                      Test
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}

      {activeTab === 'proxies' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          {proxyHealth.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16">
              <GlobeAltIcon className="w-12 h-12 text-apple-gray-300 mb-3" />
              <p className="text-lg font-medium text-apple-gray-700 mb-1">No proxies configured</p>
              <p className="text-sm text-apple-gray-400 mb-4">Add proxy servers to route traffic through them</p>
              <a href="/admin/proxies" className="btn btn-primary text-sm px-4 py-2">Add Proxy</a>
            </div>
          ) : (
            <div className="space-y-4">
              {proxyHealth.map((proxy: any) => (
                <div key={proxy.id} className="flex items-center justify-between p-4 bg-apple-gray-50 rounded-apple">
                  <div className="flex items-center gap-4">
                    {getStatusIcon(proxy.is_healthy)}
                    <div>
                      <p className="font-medium text-apple-gray-900">{proxy.url}</p>
                      <p className="text-sm text-apple-gray-500">{proxy.type} {proxy.region ? `• ${proxy.region}` : ''}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <p className={`text-sm font-medium ${proxy.is_active ? 'text-apple-green' : 'text-apple-gray-400'}`}>{proxy.is_active ? 'Active' : 'Inactive'}</p>
                      <p className="text-xs text-apple-gray-500">Status</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{proxy.response_time}ms</p>
                      <p className="text-xs text-apple-gray-500">Latency</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">{(proxy.success_rate * 100).toFixed(1)}%</p>
                      <p className="text-xs text-apple-gray-500">Success rate</p>
                    </div>
                    <button onClick={() => checkProxy(proxy.id)} className="btn btn-secondary text-sm px-3 py-1.5" title="Check now">
                      <ArrowPathIcon className="w-4 h-4 mr-1" />
                      Test
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}

      {activeTab === 'alerts' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          {alerts.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16">
              <BellSlashIcon className="w-12 h-12 text-apple-gray-300 mb-3" />
              <p className="text-lg font-medium text-apple-gray-700 mb-1">No alerts</p>
              <p className="text-sm text-apple-gray-400">Everything is running smoothly</p>
            </div>
          ) : (
            <div className="space-y-4">
              {alerts.map((alert: any) => (
                <div
                  key={alert.id}
                  className={`p-4 rounded-apple border-l-4 ${
                    alert.status === 'active' ? 'bg-red-50 border-apple-red'
                    : alert.status === 'acknowledged' ? 'bg-yellow-50 border-apple-orange'
                    : 'bg-green-50 border-apple-green'
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <p className="font-medium text-apple-gray-900">{alert.message}</p>
                      <p className="text-sm text-apple-gray-500 mt-1">{alert.target_type} - {formatDate(alert.created_at)}</p>
                    </div>
                    {alert.status === 'active' && (
                      <div className="flex gap-2">
                        <button onClick={() => acknowledgeAlert(alert.id)} className="btn-secondary text-sm px-3 py-1">Acknowledge</button>
                        <button onClick={() => resolveAlert(alert.id)} className="btn-primary text-sm px-3 py-1">Resolve</button>
                      </div>
                    )}
                    {alert.status === 'acknowledged' && (
                      <button onClick={() => resolveAlert(alert.id)} className="btn-primary text-sm px-3 py-1">Resolve</button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}
      {activeTab === 'config' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-4">
          <div className="card">
            <h3 className="text-lg font-medium text-apple-gray-900 mb-2">Alert Channel Configuration</h3>
            <p className="text-sm text-apple-gray-500 mb-4">Configure notification channels for each provider. Alerts will be sent when health checks fail.</p>
          </div>
          {providerHealth.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No providers to configure</p>
          ) : (
            providerHealth.map((provider: any) => {
              const configKey = `provider:${provider.id}`;
              const config = alertConfigs.get(configKey);
              return (
                <AlertConfigCard
                  key={provider.id}
                  providerName={provider.name}
                  providerId={provider.id}
                  config={config}
                  onSave={saveAlertConfig}
                />
              );
            })
          )}
        </motion.div>
      )}
    </div>
  );
}

/** Inline alert config editor for a single provider. */
function AlertConfigCard({
  providerName, providerId, config, onSave,
}: {
  providerName: string;
  providerId: string;
  config?: AlertConfig;
  onSave: (c: Omit<AlertConfig, 'id'>) => Promise<void>;
}) {
  const [isEnabled, setIsEnabled] = useReactState(config?.is_enabled ?? false);
  const [threshold, setThreshold] = useReactState(config?.failure_threshold ?? 3);
  const [webhookUrl, setWebhookUrl] = useReactState(config?.webhook_url ?? '');
  const [email, setEmail] = useReactState(config?.email ?? '');
  const [saving, setSaving] = useReactState(false);

  const handleSave = async () => {
    setSaving(true);
    await onSave({
      target_type: 'provider',
      target_id: providerId,
      is_enabled: isEnabled,
      failure_threshold: threshold,
      webhook_url: webhookUrl,
      email,
    });
    setSaving(false);
  };

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <ServerIcon className="w-5 h-5 text-apple-gray-400" />
          <h4 className="font-medium text-apple-gray-900">{providerName}</h4>
        </div>
        <label className="flex items-center gap-2 cursor-pointer">
          <span className="text-sm text-apple-gray-500">{isEnabled ? 'Enabled' : 'Disabled'}</span>
          <div
            className={`relative w-10 h-6 rounded-full transition-colors ${isEnabled ? 'bg-apple-blue' : 'bg-apple-gray-300'}`}
            onClick={() => setIsEnabled(!isEnabled)}
          >
            <div className={`absolute top-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform ${isEnabled ? 'translate-x-4' : 'translate-x-0.5'}`} />
          </div>
        </label>
      </div>

      <div className={`space-y-4 ${!isEnabled ? 'opacity-50 pointer-events-none' : ''}`}>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-apple-gray-700 mb-1">Webhook URL</label>
            <input
              type="url"
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              placeholder="https://hooks.slack.com/... or DingTalk webhook"
              className="input w-full"
            />
            <p className="text-xs text-apple-gray-400 mt-1">Supports Slack, DingTalk, Feishu, or any HTTP endpoint</p>
          </div>
          <div>
            <label className="block text-sm font-medium text-apple-gray-700 mb-1">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="ops-team@company.com"
              className="input w-full"
            />
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-apple-gray-700 mb-1">
            Failure Threshold: <span className="text-apple-blue">{threshold}</span> consecutive failures
          </label>
          <input
            type="range"
            min={1}
            max={10}
            value={threshold}
            onChange={(e) => setThreshold(Number(e.target.value))}
            className="w-full accent-apple-blue"
          />
          <div className="flex justify-between text-xs text-apple-gray-400">
            <span>1 (sensitive)</span>
            <span>10 (tolerant)</span>
          </div>
        </div>
      </div>

      <div className="flex justify-end mt-4 pt-4 border-t border-apple-gray-100">
        <button onClick={handleSave} disabled={saving} className="btn btn-primary text-sm">
          {saving ? 'Saving...' : 'Save Configuration'}
        </button>
      </div>
    </div>
  );
}

export default HealthPage;
