import { motion } from 'framer-motion';
import { ArrowPathIcon, CheckCircleIcon, XCircleIcon, ExclamationTriangleIcon, ServerIcon } from '@heroicons/react/24/outline';
import { useHealth } from '@/hooks/useHealth';

function getStatusIcon(status: string | boolean) {
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

      <div className="flex gap-4 border-b border-apple-gray-200">
        {(['providers', 'api-keys', 'proxies', 'alerts'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`pb-3 px-1 font-medium transition-colors flex items-center gap-2 ${
              activeTab === tab ? 'text-apple-blue border-b-2 border-apple-blue' : 'text-apple-gray-500 hover:text-apple-gray-700'
            }`}
          >
            {tab === 'api-keys' ? 'API Keys' : tab.charAt(0).toUpperCase() + tab.slice(1)}
            {tab === 'alerts' && activeAlerts > 0 && (
              <span className="bg-apple-red text-white text-xs px-2 py-0.5 rounded-full">{activeAlerts}</span>
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
            <p className="text-center text-apple-gray-500 py-8">No active providers configured</p>
          ) : (
            <div className="space-y-4">
              {providerHealth.map((provider) => (
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
                    <button onClick={() => checkProvider(provider.id)} className="btn btn-ghost p-2" title="Check now">
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
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          {apiKeyHealth.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No API keys configured</p>
          ) : (
            <div className="space-y-4">
              {apiKeyHealth.map((key) => (
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
                    <button onClick={() => checkApiKey(key.id)} className="btn btn-ghost p-2" title="Check now">
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
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          {proxyHealth.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No proxies configured</p>
          ) : (
            <div className="space-y-4">
              {proxyHealth.map((proxy) => (
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
                    <button onClick={() => checkProxy(proxy.id)} className="btn btn-ghost p-2" title="Check now">
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
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card">
          {alerts.length === 0 ? (
            <p className="text-center text-apple-gray-500 py-8">No alerts</p>
          ) : (
            <div className="space-y-4">
              {alerts.map((alert) => (
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
    </div>
  );
}

export default HealthPage;
