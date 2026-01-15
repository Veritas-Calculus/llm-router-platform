import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { ArrowPathIcon, CheckCircleIcon, XCircleIcon, ExclamationTriangleIcon, ServerIcon } from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { healthApi, alertsApi, ApiKeyHealth, ProxyHealth, ProviderHealth, Alert } from '@/lib/api';

function HealthPage() {
  const [apiKeyHealth, setApiKeyHealth] = useState<ApiKeyHealth[]>([]);
  const [proxyHealth, setProxyHealth] = useState<ProxyHealth[]>([]);
  const [providerHealth, setProviderHealth] = useState<ProviderHealth[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [activeTab, setActiveTab] = useState<'providers' | 'api-keys' | 'proxies' | 'alerts'>('providers');

  useEffect(() => {
    loadHealthData();
  }, []);

  const loadHealthData = async () => {
    try {
      const [apiKeysRes, proxiesRes, providersRes, alertsRes] = await Promise.all([
        healthApi.getApiKeysHealth(),
        healthApi.getProxiesHealth(),
        healthApi.getProvidersHealth(),
        alertsApi.list(),
      ]);
      setApiKeyHealth(apiKeysRes?.data || []);
      setProxyHealth(proxiesRes?.data || []);
      setProviderHealth(providersRes || []);
      setAlerts(alertsRes?.data || []);
    } catch (error) {
      toast.error('Failed to load health data');
      setApiKeyHealth([]);
      setProxyHealth([]);
      setProviderHealth([]);
      setAlerts([]);
    } finally {
      setLoading(false);
    }
  };

  const refreshAll = async () => {
    setRefreshing(true);
    await loadHealthData();
    setRefreshing(false);
    toast.success('Health data refreshed');
  };

  const checkApiKey = async (id: string) => {
    try {
      const result = await healthApi.checkApiKey(id);
      setApiKeyHealth((prev) =>
        prev.map((key) => (key.id === id ? result : key))
      );
      toast.success('API key checked');
    } catch (error) {
      toast.error('Health check failed');
    }
  };

  const checkProxy = async (id: string) => {
    try {
      const result = await healthApi.checkProxy(id);
      setProxyHealth((prev) =>
        prev.map((p) => (p.id === id ? result : p))
      );
      toast.success('Proxy checked');
    } catch (error) {
      toast.error('Health check failed');
    }
  };

  const checkProvider = async (id: string) => {
    try {
      const result = await healthApi.checkProvider(id);
      setProviderHealth((prev) =>
        prev.map((p) => (p.id === id ? result : p))
      );
      toast.success('Provider checked');
    } catch (error) {
      toast.error('Health check failed');
    }
  };

  const checkAllProviders = async () => {
    try {
      await healthApi.checkAllProviders();
      await loadHealthData();
      toast.success('All providers checked');
    } catch (error) {
      toast.error('Health check failed');
    }
  };

  const acknowledgeAlert = async (id: string) => {
    try {
      await alertsApi.acknowledge(id);
      setAlerts((prev) =>
        prev.map((a) =>
          a.id === id ? { ...a, status: 'acknowledged', acknowledged_at: new Date().toISOString() } : a
        )
      );
      toast.success('Alert acknowledged');
    } catch (error) {
      toast.error('Failed to acknowledge alert');
    }
  };

  const resolveAlert = async (id: string) => {
    try {
      await alertsApi.resolve(id);
      setAlerts((prev) =>
        prev.map((a) =>
          a.id === id ? { ...a, status: 'resolved', resolved_at: new Date().toISOString() } : a
        )
      );
      toast.success('Alert resolved');
    } catch (error) {
      toast.error('Failed to resolve alert');
    }
  };

  const getStatusIcon = (status: string | boolean) => {
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
  };

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  const activeAlerts = alerts.filter((a) => a.status === 'active').length;

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Health Monitor</h1>
          <p className="text-apple-gray-500 mt-1">Monitor the health of your API keys and proxies</p>
        </div>
        <button
          onClick={refreshAll}
          className="btn btn-secondary"
          disabled={refreshing}
        >
          <ArrowPathIcon className={`w-5 h-5 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      <div className="flex gap-4 border-b border-apple-gray-200">
        <button
          onClick={() => setActiveTab('providers')}
          className={`pb-3 px-1 font-medium transition-colors ${
            activeTab === 'providers'
              ? 'text-apple-blue border-b-2 border-apple-blue'
              : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          Providers
        </button>
        <button
          onClick={() => setActiveTab('api-keys')}
          className={`pb-3 px-1 font-medium transition-colors ${
            activeTab === 'api-keys'
              ? 'text-apple-blue border-b-2 border-apple-blue'
              : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          API Keys
        </button>
        <button
          onClick={() => setActiveTab('proxies')}
          className={`pb-3 px-1 font-medium transition-colors ${
            activeTab === 'proxies'
              ? 'text-apple-blue border-b-2 border-apple-blue'
              : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          Proxies
        </button>
        <button
          onClick={() => setActiveTab('alerts')}
          className={`pb-3 px-1 font-medium transition-colors flex items-center gap-2 ${
            activeTab === 'alerts'
              ? 'text-apple-blue border-b-2 border-apple-blue'
              : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          Alerts
          {activeAlerts > 0 && (
            <span className="bg-apple-red text-white text-xs px-2 py-0.5 rounded-full">
              {activeAlerts}
            </span>
          )}
        </button>
      </div>

      {activeTab === 'providers' && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="card"
        >
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-apple-gray-900">Provider Health</h3>
            <button
              onClick={checkAllProviders}
              className="btn btn-secondary text-sm"
            >
              <ArrowPathIcon className="w-4 h-4 mr-1" />
              Check All
            </button>
          </div>
          {providerHealth.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No active providers configured</p>
          ) : (
            <div className="space-y-4">
              {providerHealth.map((provider) => (
                <div
                  key={provider.id}
                  className="flex items-center justify-between p-4 bg-apple-gray-50 rounded-apple"
                >
                  <div className="flex items-center gap-4">
                    {getStatusIcon(provider.is_healthy)}
                    <div>
                      <p className="font-medium text-apple-gray-900">{provider.name}</p>
                      <p className="text-sm text-apple-gray-500">{provider.base_url}</p>
                      {provider.error_message && (
                        <p className="text-xs text-apple-red mt-1">{provider.error_message}</p>
                      )}
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
                      <p className="text-sm font-medium text-apple-gray-900">
                        {provider.response_time > 0 ? `${provider.response_time}ms` : '-'}
                      </p>
                      <p className="text-xs text-apple-gray-500">Latency</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">
                        {(provider.success_rate * 100).toFixed(1)}%
                      </p>
                      <p className="text-xs text-apple-gray-500">Success rate</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm text-apple-gray-500">
                        {provider.last_check ? formatDate(provider.last_check) : 'Never'}
                      </p>
                      <p className="text-xs text-apple-gray-400">Last checked</p>
                    </div>
                    <button
                      onClick={() => checkProvider(provider.id)}
                      className="btn btn-ghost p-2"
                      title="Check now"
                    >
                      <ArrowPathIcon className="w-5 h-5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}

      {activeTab === 'api-keys' && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="card"
        >
          {apiKeyHealth.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No API keys configured</p>
          ) : (
            <div className="space-y-4">
              {apiKeyHealth.map((key) => (
                <div
                  key={key.id}
                  className="flex items-center justify-between p-4 bg-apple-gray-50 rounded-apple"
                >
                  <div className="flex items-center gap-4">
                    {getStatusIcon(key.status)}
                    <div>
                      <p className="font-medium text-apple-gray-900">{key.alias}</p>
                      <p className="text-sm text-apple-gray-500">{key.provider_name}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">
                        {key.latency_ms}ms
                      </p>
                      <p className="text-xs text-apple-gray-500">Latency</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm text-apple-gray-500">
                        {formatDate(key.last_checked)}
                      </p>
                      <p className="text-xs text-apple-gray-400">Last checked</p>
                    </div>
                    <button
                      onClick={() => checkApiKey(key.id)}
                      className="btn btn-ghost p-2"
                      title="Check now"
                    >
                      <ArrowPathIcon className="w-5 h-5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}

      {activeTab === 'proxies' && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="card"
        >
          {proxyHealth.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No proxies configured</p>
          ) : (
            <div className="space-y-4">
              {proxyHealth.map((proxy) => (
                <div
                  key={proxy.id}
                  className="flex items-center justify-between p-4 bg-apple-gray-50 rounded-apple"
                >
                  <div className="flex items-center gap-4">
                    {getStatusIcon(proxy.status)}
                    <div>
                      <p className="font-medium text-apple-gray-900">{proxy.name}</p>
                      <p className="text-sm text-apple-gray-500">
                        {proxy.host}:{proxy.port}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">
                        {proxy.latency_ms}ms
                      </p>
                      <p className="text-xs text-apple-gray-500">Latency</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium text-apple-gray-900">
                        {(proxy.success_rate * 100).toFixed(1)}%
                      </p>
                      <p className="text-xs text-apple-gray-500">Success rate</p>
                    </div>
                    <button
                      onClick={() => checkProxy(proxy.id)}
                      className="btn btn-ghost p-2"
                      title="Check now"
                    >
                      <ArrowPathIcon className="w-5 h-5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}

      {activeTab === 'alerts' && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="card"
        >
          {alerts.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No alerts</p>
          ) : (
            <div className="space-y-4">
              {alerts.map((alert) => (
                <div
                  key={alert.id}
                  className={`p-4 rounded-apple border-l-4 ${
                    alert.status === 'active'
                      ? 'bg-red-50 border-apple-red'
                      : alert.status === 'acknowledged'
                      ? 'bg-yellow-50 border-apple-orange'
                      : 'bg-green-50 border-apple-green'
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <p className="font-medium text-apple-gray-900">{alert.message}</p>
                      <p className="text-sm text-apple-gray-500 mt-1">
                        {alert.target_type} - {formatDate(alert.created_at)}
                      </p>
                    </div>
                    {alert.status === 'active' && (
                      <div className="flex gap-2">
                        <button
                          onClick={() => acknowledgeAlert(alert.id)}
                          className="btn-secondary text-sm px-3 py-1"
                        >
                          Acknowledge
                        </button>
                        <button
                          onClick={() => resolveAlert(alert.id)}
                          className="btn-primary text-sm px-3 py-1"
                        >
                          Resolve
                        </button>
                      </div>
                    )}
                    {alert.status === 'acknowledged' && (
                      <button
                        onClick={() => resolveAlert(alert.id)}
                        className="btn-primary text-sm px-3 py-1"
                      >
                        Resolve
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </motion.div>
      )}
    </div>
  );
}

export default HealthPage;
