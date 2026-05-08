import {
  PlayIcon,
  CheckCircleIcon,
  XCircleIcon,
  ArrowPathIcon,
  TrashIcon,
  AdjustmentsHorizontalIcon,
} from '@heroicons/react/24/outline';
import { Provider, ProviderHealthStatus, Proxy } from '@/lib/types';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

interface ProviderInfoCardProps {
  provider: Provider;
  proxies: Proxy[];
  healthStatus: ProviderHealthStatus | null;
  testing: boolean;
  savingProxy: boolean;
  onTestConnection: () => void;
  onToggleProxy: () => void;
  onProxyChange: (proxyId: string) => void;
  onUpdateProviderSettings: (input: {
    priority: number;
    weight: number;
    maxRetries: number;
    timeout: number;
  }) => Promise<void>;
  onDeleteProvider: (id: string) => Promise<void>;
}

export default function ProviderInfoCard({
  provider,
  proxies,
  healthStatus,
  testing,
  savingProxy,
  onTestConnection,
  onToggleProxy,
  onProxyChange,
  onUpdateProviderSettings,
  onDeleteProvider,
}: ProviderInfoCardProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [savingSettings, setSavingSettings] = useState(false);
  const [settings, setSettings] = useState({
    priority: provider.priority,
    weight: provider.weight,
    maxRetries: provider.max_retries,
    timeout: provider.timeout,
  });

  useEffect(() => {
    setSettings({
      priority: provider.priority,
      weight: provider.weight,
      maxRetries: provider.max_retries,
      timeout: provider.timeout,
    });
  }, [provider.id, provider.priority, provider.weight, provider.max_retries, provider.timeout]);

  const settingsChanged =
    settings.priority !== provider.priority ||
    settings.weight !== provider.weight ||
    settings.maxRetries !== provider.max_retries ||
    settings.timeout !== provider.timeout;

  const saveSettings = async () => {
    setSavingSettings(true);
    try {
      await onUpdateProviderSettings(settings);
    } finally {
      setSavingSettings(false);
    }
  };

  return (
    <div className="card">
      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-xl font-semibold text-apple-gray-900">
            {provider.name}
          </h2>
          <p className="text-sm text-apple-gray-500 mt-1">
            {provider.base_url}
          </p>
          <div className="flex items-center gap-4 mt-3">
            <span
              className={provider.is_active ? 'badge-success' : 'badge-error'}
            >
              {provider.is_active ? 'Enabled' : 'Disabled'}
            </span>
            <span className="text-sm text-apple-gray-500">
              Timeout: {provider.timeout}s
            </span>
            <span className="text-sm text-apple-gray-500">
              Retries: {provider.max_retries}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onTestConnection}
            className="btn btn-secondary"
            disabled={testing}
          >
            {testing ? (
              <ArrowPathIcon className="w-5 h-5 mr-2 animate-spin" />
            ) : (
              <PlayIcon className="w-5 h-5 mr-2" />
            )}
            Test Connection
          </button>
          {!confirmDelete ? (
            <button
              onClick={() => setConfirmDelete(true)}
              className="p-2 rounded-apple text-apple-gray-400 hover:text-apple-red hover:bg-red-50 transition-colors"
              title="Delete provider"
            >
              <TrashIcon className="w-5 h-5" />
            </button>
          ) : (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-apple bg-red-50 border border-apple-red/20">
              <span className="text-sm text-apple-red font-medium">确认删除？</span>
              <button
                onClick={async () => {
                  setDeleting(true);
                  try {
                    await onDeleteProvider(provider.id);
                  } finally {
                    setDeleting(false);
                    setConfirmDelete(false);
                  }
                }}
                disabled={deleting}
                className="px-2.5 py-1 text-xs font-medium text-white bg-apple-red rounded-md hover:bg-red-600 transition-colors disabled:opacity-50"
              >
                {deleting ? '删除中...' : '删除'}
              </button>
              <button
                onClick={() => setConfirmDelete(false)}
                className="px-2.5 py-1 text-xs font-medium text-apple-gray-700 bg-white rounded-md hover:bg-apple-gray-100 transition-colors border border-apple-gray-200"
              >
                取消
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Routing Settings */}
      <div className="mt-4 pt-4 border-t border-apple-gray-100">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 flex-1">
            <div>
              <label className="label">Priority</label>
              <input
                type="number"
                min="0"
                value={settings.priority}
                onChange={(e) => setSettings((prev) => ({ ...prev, priority: parseInt(e.target.value) || 0 }))}
                className="input"
              />
            </div>
            <div>
              <label className="label">Weight</label>
              <input
                type="number"
                min="0"
                step="0.1"
                value={settings.weight}
                onChange={(e) => setSettings((prev) => ({ ...prev, weight: parseFloat(e.target.value) || 0 }))}
                className="input"
              />
            </div>
            <div>
              <label className="label">Retries</label>
              <input
                type="number"
                min="0"
                value={settings.maxRetries}
                onChange={(e) => setSettings((prev) => ({ ...prev, maxRetries: parseInt(e.target.value) || 0 }))}
                className="input"
              />
            </div>
            <div>
              <label className="label">Timeout</label>
              <input
                type="number"
                min="1"
                value={settings.timeout}
                onChange={(e) => setSettings((prev) => ({ ...prev, timeout: parseInt(e.target.value) || 1 }))}
                className="input"
              />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Link to="/admin/routing-rules" className="btn btn-secondary whitespace-nowrap">
              <AdjustmentsHorizontalIcon className="w-5 h-5 mr-2" />
              Routing Rules
            </Link>
            <button
              onClick={saveSettings}
              className="btn btn-primary whitespace-nowrap"
              disabled={!settingsChanged || savingSettings}
            >
              {savingSettings ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      </div>

      {/* Proxy Toggle */}
      <div className="mt-4 pt-4 border-t border-apple-gray-100">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="text-sm font-medium text-apple-gray-900">Use Proxy</h4>
            <p className="text-xs text-apple-gray-500 mt-0.5">
              Route requests through configured proxy servers
            </p>
          </div>
          <button
            onClick={onToggleProxy}
            className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${provider.use_proxy ? 'bg-apple-blue' : 'bg-apple-gray-200'
              }`}
          >
            <span
              className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${provider.use_proxy ? 'translate-x-5' : 'translate-x-0'
                }`}
            />
          </button>
        </div>

        {/* Proxy Selection Dropdown */}
        {provider.use_proxy && (
          <div className="mt-4">
            <label className="block text-sm font-medium text-apple-gray-700 mb-2">
              Default Proxy
            </label>
            <select
              value={provider.default_proxy_id || ''}
              onChange={(e) => onProxyChange(e.target.value)}
              disabled={savingProxy}
              className="w-full px-3 py-2 border border-apple-gray-200 rounded-apple text-sm focus:outline-none focus:ring-2 focus:ring-apple-blue focus:border-transparent disabled:opacity-50"
            >
              <option value="">Auto-select (first available)</option>
              {proxies
                .filter((p) => p.is_active)
                .map((proxy) => (
                  <option key={proxy.id} value={proxy.id}>
                    {proxy.url} {proxy.region ? `(${proxy.region})` : ''}
                  </option>
                ))}
            </select>
            {savingProxy && (
              <p className="text-xs text-apple-gray-500 mt-1">Saving...</p>
            )}
          </div>
        )}
      </div>

      {healthStatus && (
        <div
          className={`mt-4 p-4 rounded-apple ${healthStatus.is_healthy
            ? 'bg-green-50 border border-apple-green'
            : 'bg-red-50 border border-apple-red'
            }`}
        >
          <div className="flex items-center gap-2">
            {healthStatus.is_healthy ? (
              <CheckCircleIcon className="w-5 h-5 text-apple-green" />
            ) : (
              <XCircleIcon className="w-5 h-5 text-apple-red" />
            )}
            <span
              className={`font-medium ${healthStatus.is_healthy ? 'text-apple-green' : 'text-apple-red'
                }`}
            >
              {healthStatus.is_healthy ? 'Connection Successful' : 'Connection Failed'}
            </span>
            {healthStatus.is_healthy && (
              <span className="text-sm text-apple-gray-500 ml-2">
                Latency: {healthStatus.response_time}ms
              </span>
            )}
            {!healthStatus.is_healthy && healthStatus.error_message && (
              <span className="text-sm text-apple-red ml-2">
                {healthStatus.error_message}
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
