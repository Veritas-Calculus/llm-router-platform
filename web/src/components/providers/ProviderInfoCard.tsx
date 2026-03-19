import {
  PlayIcon,
  CheckCircleIcon,
  XCircleIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';
import { Provider, ProviderHealthStatus, Proxy } from '@/lib/types';

interface ProviderInfoCardProps {
  provider: Provider;
  proxies: Proxy[];
  healthStatus: ProviderHealthStatus | null;
  testing: boolean;
  savingProxy: boolean;
  onTestConnection: () => void;
  onToggleProxy: () => void;
  onProxyChange: (proxyId: string) => void;
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
}: ProviderInfoCardProps) {
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
