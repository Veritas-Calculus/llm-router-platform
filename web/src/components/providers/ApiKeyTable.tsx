import { useRef, useState } from 'react';
import {
  ArrowDownTrayIcon,
  DocumentArrowUpIcon,
  PlusIcon,
  TrashIcon,
  CheckCircleIcon,
  XCircleIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { downloadCsvTemplate, parseCsv } from '@/lib/csv';
import { ApiKeyRuntimeStatus, ProviderApiKey, Proxy, ProxyPool } from '@/lib/types';
import ConfirmModal from '@/components/ConfirmModal';

interface ApiKeyTableProps {
  providerName: string;
  apiKeys: ProviderApiKey[];
  proxies: Proxy[];
  proxyPools: ProxyPool[];
  healthByKeyId?: Record<string, ApiKeyRuntimeStatus>;
  onAddKey: (data: { api_key: string; alias: string; priority: number; weight: number; rate_limit: number; proxy_id?: string; proxy_pool_id?: string }) => Promise<void>;
  onUpdateKey: (keyId: string, data: { priority: number; weight: number; rate_limit: number; proxy_id?: string; proxy_pool_id?: string }) => Promise<void>;
  onToggleKey: (key: ProviderApiKey) => void;
  onDeleteKey: (keyId: string) => Promise<void>;
}

export default function ApiKeyTable({
  providerName,
  apiKeys,
  proxies,
  proxyPools,
  healthByKeyId = {},
  onAddKey,
  onUpdateKey,
  onToggleKey,
  onDeleteKey,
}: ApiKeyTableProps) {
  const csvInputRef = useRef<HTMLInputElement>(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState({ api_key: '', alias: '', priority: 1, weight: 1.0, rate_limit: 0, binding: '' });
  const [adding, setAdding] = useState(false);
  const [editingKeyId, setEditingKeyId] = useState<string | null>(null);
  const [editKeyData, setEditKeyData] = useState({ priority: 1, weight: 1.0, rate_limit: 0, binding: '' });
  const [updatingKey, setUpdatingKey] = useState(false);
  const [importingCsv, setImportingCsv] = useState(false);
  const [confirmModal, setConfirmModal] = useState<{ isOpen: boolean; keyId: string }>({ isOpen: false, keyId: '' });
  const [processing, setProcessing] = useState(false);
  const canAddKey = Boolean(newKey.api_key.trim() && !adding);

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const handleAddKey = async () => {
    const trimmedKey = newKey.api_key.trim();
    if (!trimmedKey) return;
    setAdding(true);
    try {
      const binding = parseBinding(newKey.binding);
      await onAddKey({
        api_key: trimmedKey,
        alias: newKey.alias.trim() || `${providerName} primary`,
        priority: newKey.priority,
        weight: newKey.weight,
        rate_limit: newKey.rate_limit,
        proxy_id: binding.proxy_id,
        proxy_pool_id: binding.proxy_pool_id,
      });
      setShowAddModal(false);
      setNewKey({ api_key: '', alias: '', priority: 1, weight: 1.0, rate_limit: 0, binding: '' });
    } finally {
      setAdding(false);
    }
  };

  const startEditingKey = (key: ProviderApiKey) => {
    setEditingKeyId(key.id);
    setEditKeyData({
      priority: key.priority || 1,
      weight: key.weight || 1.0,
      rate_limit: key.rate_limit || 0,
      binding: bindingValue(key),
    });
  };

  const handleUpdateKey = async () => {
    if (!editingKeyId) return;
    setUpdatingKey(true);
    try {
      const binding = parseBinding(editKeyData.binding);
      await onUpdateKey(editingKeyId, {
        priority: editKeyData.priority,
        weight: editKeyData.weight,
        rate_limit: editKeyData.rate_limit,
        proxy_id: binding.proxy_id,
        proxy_pool_id: binding.proxy_pool_id,
      });
      setEditingKeyId(null);
    } finally {
      setUpdatingKey(false);
    }
  };

  const handleConfirmDelete = async () => {
    setProcessing(true);
    try {
      await onDeleteKey(confirmModal.keyId);
      setConfirmModal({ isOpen: false, keyId: '' });
    } finally {
      setProcessing(false);
    }
  };

  const handleCsvFile = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      void handleImportCsv(String(reader.result || ''));
    };
    reader.readAsText(file);
    event.target.value = '';
  };

  const handleImportCsv = async (csvText: string) => {
    const rows = parseCsv(csvText);
    if (rows.length === 0) { toast.error('CSV contains no API keys'); return; }
    setImportingCsv(true);
    let success = 0;
    let failed = 0;
    try {
      for (const row of rows) {
        const apiKey = row.api_key || row.key || row.token;
        if (!apiKey?.trim()) continue;
        const binding = bindingFromCsv(row, proxies, proxyPools);
        try {
          await onAddKey({
            api_key: apiKey.trim(),
            alias: row.alias?.trim() || `${providerName} imported ${success + failed + 1}`,
            priority: parseInt(row.priority || '1', 10) || 1,
            weight: parseFloat(row.weight || '1') || 1,
            rate_limit: parseInt(row.rate_limit || row.rate_limit_rps || '0', 10) || 0,
            proxy_id: binding.proxy_id,
            proxy_pool_id: binding.proxy_pool_id,
          });
          success++;
        } catch {
          failed++;
        }
      }
      if (success > 0) toast.success(`Imported ${success} API keys`);
      if (failed > 0) toast.error(`Failed to import ${failed} API keys`);
      if (success === 0 && failed === 0) toast.error('CSV contains no valid API keys');
    } finally {
      setImportingCsv(false);
    }
  };

  return (
    <div className="card overflow-x-auto">
      <input ref={csvInputRef} type="file" accept=".csv,text/csv" className="hidden" onChange={handleCsvFile} />
      <div className="flex items-center justify-between mb-6">
        <div>
          <h3 className="text-lg font-semibold text-apple-gray-900">API Keys</h3>
          <p className="text-sm text-apple-gray-500 mt-1">
            Manage API keys for {providerName}
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-3">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => downloadCsvTemplate(
              `${providerName}-api-keys-template.csv`,
              ['alias', 'api_key', 'priority', 'weight', 'rate_limit', 'binding_type', 'binding_value'],
              [
                [`${providerName}-primary`, 'sk-...', '1', '1', '0', 'pool', 'residential-us'],
                [`${providerName}-backup`, 'sk-...', '2', '0.5', '5', 'proxy', 'http://proxy.example.com:8080'],
              ],
            )}
          >
            <ArrowDownTrayIcon className="w-5 h-5 mr-2" />
            Template
          </button>
          <button type="button" onClick={() => csvInputRef.current?.click()} className="btn btn-secondary" disabled={importingCsv}>
            <DocumentArrowUpIcon className="w-5 h-5 mr-2" />
            {importingCsv ? 'Importing' : 'Import CSV'}
          </button>
          <button onClick={() => setShowAddModal(true)} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" />
            Add Key
          </button>
        </div>
      </div>

      {apiKeys.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-apple-gray-500 mb-4">No API keys for this provider</p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-apple-gray-200">
            <thead>
              <tr>
                <th className="table-header">Alias</th>
                <th className="table-header w-48">Config</th>
                <th className="table-header">Binding</th>
                <th className="table-header">Status</th>
                <th className="table-header">Health</th>
                <th className="table-header">Usage</th>
                <th className="table-header">Last Used</th>
                <th className="table-header text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {apiKeys.map((key) => (
                <tr key={key.id} className="hover:bg-apple-gray-50">
                  <td className="table-cell">
                    <span className="font-medium text-apple-gray-900 block">{key.alias}</span>
                    <code className="text-xs bg-apple-gray-100 px-1 py-0.5 rounded mt-1 inline-block">
                      {key.key_prefix}
                    </code>
                  </td>
                  <td className="table-cell">
                    {editingKeyId === key.id ? (
                      <div className="space-y-2">
                        <div className="flex items-center gap-2">
                          <label className="text-xs w-16 text-apple-gray-500">Priority</label>
                          <input
                            type="number" min="1" max="100"
                            value={editKeyData.priority}
                            onChange={e => setEditKeyData(p => ({ ...p, priority: parseInt(e.target.value) || 1 }))}
                            className="input text-xs py-1 px-2 w-20"
                          />
                        </div>
                        <div className="flex items-center gap-2">
                          <label className="text-xs w-16 text-apple-gray-500">Weight</label>
                          <input
                            type="number" step="0.1" min="0" max="100"
                            value={editKeyData.weight}
                            onChange={e => setEditKeyData(p => ({ ...p, weight: parseFloat(e.target.value) || 0 }))}
                            className="input text-xs py-1 px-2 w-20"
                          />
                        </div>
                        <div className="flex items-center gap-2">
                          <label className="text-xs w-16 text-apple-gray-500">Rate Limit</label>
                          <input
                            type="number" min="0"
                            value={editKeyData.rate_limit}
                            onChange={e => setEditKeyData(p => ({ ...p, rate_limit: parseInt(e.target.value) || 0 }))}
                            className="input text-xs py-1 px-2 w-20"
                            placeholder="0 = unltd"
                          />
                        </div>
                        <BindingSelect
                          value={editKeyData.binding}
                          proxies={proxies}
                          proxyPools={proxyPools}
                          onChange={(binding) => setEditKeyData(p => ({ ...p, binding }))}
                        />
                      </div>
                    ) : (
                      <div className="space-y-1 text-xs text-apple-gray-600">
                        <div><span className="text-apple-gray-400 w-16 inline-block">Priority:</span> <span className="font-medium">{key.priority || 1}</span></div>
                        <div><span className="text-apple-gray-400 w-16 inline-block">Weight:</span> <span className="font-medium">{key.weight || 1.0}</span></div>
                        <div><span className="text-apple-gray-400 w-16 inline-block">Rate Limit:</span> <span className="font-medium">{key.rate_limit ? `${key.rate_limit} RPS` : 'Unlimited'}</span></div>
                      </div>
                    )}
                  </td>
                  <td className="table-cell text-sm text-apple-gray-600">
                    {bindingLabel(key, proxies, proxyPools)}
                  </td>
                  <td className="table-cell">
                    <button
                      onClick={() => onToggleKey(key)}
                      className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${key.is_active
                        ? 'bg-green-100 text-apple-green hover:bg-green-200'
                        : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
                        }`}
                    >
                      {key.is_active ? (
                        <>
                          <CheckCircleIcon className="w-3.5 h-3.5" />
                          Active
                        </>
                      ) : (
                        <>
                          <XCircleIcon className="w-3.5 h-3.5" />
                          Inactive
                        </>
                      )}
                    </button>
                  </td>
                  <td className="table-cell">
                    <HealthBadge status={healthByKeyId[key.id]} />
                  </td>
                  <td className="table-cell text-sm text-apple-gray-500">
                    {key.usage_count.toLocaleString()} reqs
                  </td>
                  <td className="table-cell text-sm text-apple-gray-500">
                    {key.last_used_at ? formatDate(key.last_used_at) : 'Never'}
                  </td>
                  <td className="table-cell text-right">
                    {editingKeyId === key.id ? (
                      <div className="flex items-center justify-end gap-2">
                        <button onClick={() => setEditingKeyId(null)} className="text-xs text-apple-gray-500 hover:text-apple-gray-700">Cancel</button>
                        <button onClick={handleUpdateKey} disabled={updatingKey} className="text-xs bg-apple-blue text-white px-2 py-1 rounded hover:bg-blue-600">
                          {updatingKey ? 'Saving' : 'Save'}
                        </button>
                      </div>
                    ) : (
                      <div className="flex items-center justify-end gap-3">
                        <button
                          onClick={() => startEditingKey(key)}
                          className="text-apple-blue hover:text-blue-600 transition-colors text-sm"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => setConfirmModal({ isOpen: true, keyId: key.id })}
                          className="text-apple-red hover:text-red-600 transition-colors text-sm inline-flex items-center gap-1"
                          title="Delete API key"
                        >
                          <TrashIcon className="w-4 h-4" />
                          Delete
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Key Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4">
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">Add API Key</h2>
            <div className="space-y-4">
              <div>
                <label className="label">API Key</label>
                <input
                  type="password"
                  value={newKey.api_key}
                  onChange={(e) => setNewKey((prev) => ({ ...prev, api_key: e.target.value }))}
                  className="input"
                  placeholder="sk-..."
                  autoComplete="off"
                  required
                />
              </div>
              <div>
                <label className="label">Alias (optional)</label>
                <input
                  type="text"
                  value={newKey.alias}
                  onChange={(e) => setNewKey((prev) => ({ ...prev, alias: e.target.value }))}
                  className="input"
                  placeholder="e.g., Production Key 1"
                />
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="label">Priority</label>
                  <input
                    type="number" min="1" max="100"
                    value={newKey.priority}
                    onChange={(e) => setNewKey((prev) => ({ ...prev, priority: parseInt(e.target.value) || 1 }))}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Weight</label>
                  <input
                    type="number" step="0.1" min="0"
                    value={newKey.weight}
                    onChange={(e) => setNewKey((prev) => ({ ...prev, weight: parseFloat(e.target.value) || 1.0 }))}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Rate Limit</label>
                  <input
                    type="number" min="0"
                    value={newKey.rate_limit}
                    onChange={(e) => setNewKey((prev) => ({ ...prev, rate_limit: parseInt(e.target.value) || 0 }))}
                    className="input"
                    placeholder="0"
                  />
                </div>
              </div>
              <div>
                <label className="label">Proxy Binding</label>
                <BindingSelect
                  value={newKey.binding}
                  proxies={proxies}
                  proxyPools={proxyPools}
                  onChange={(binding) => setNewKey((prev) => ({ ...prev, binding }))}
                />
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowAddModal(false)} className="btn btn-secondary">Cancel</button>
              <button onClick={handleAddKey} className="btn btn-primary" disabled={!canAddKey}>
                {adding ? 'Adding...' : 'Add Key'}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title="Delete API Key"
        message="This action cannot be undone. The API key will be permanently deleted."
        confirmText="Delete"
        confirmColor="red"
        onConfirm={handleConfirmDelete}
        onCancel={() => setConfirmModal({ isOpen: false, keyId: '' })}
        loading={processing}
      />
    </div>
  );
}

function bindingValue(key: ProviderApiKey): string {
  if (key.proxy_id) return `proxy:${key.proxy_id}`;
  if (key.proxy_pool_id) return `pool:${key.proxy_pool_id}`;
  return '';
}

function parseBinding(binding: string): { proxy_id?: string; proxy_pool_id?: string } {
  if (binding.startsWith('proxy:')) return { proxy_id: binding.slice('proxy:'.length) };
  if (binding.startsWith('pool:')) return { proxy_pool_id: binding.slice('pool:'.length) };
  return {};
}

function bindingFromCsv(row: Record<string, string>, proxies: Proxy[], proxyPools: ProxyPool[]): { proxy_id?: string; proxy_pool_id?: string } {
  if (row.proxy_id) return { proxy_id: row.proxy_id.trim() };
  if (row.proxy_pool_id) return { proxy_pool_id: row.proxy_pool_id.trim() };

  const type = (row.binding_type || row.proxy_binding_type || '').trim().toLowerCase();
  const value = (row.binding_value || row.proxy_binding_value || row.proxy || row.proxy_pool || '').trim();
  if (!type || !value) return {};

  if (type === 'proxy') {
    const proxy = proxies.find((item) => item.id === value || item.url === value);
    return { proxy_id: proxy?.id || value };
  }
  if (type === 'pool' || type === 'proxy_pool') {
    const pool = proxyPools.find((item) => item.id === value || item.name.toLowerCase() === value.toLowerCase());
    return { proxy_pool_id: pool?.id || value };
  }
  return {};
}

function bindingLabel(key: ProviderApiKey, proxies: Proxy[], proxyPools: ProxyPool[]): string {
  if (key.proxy_id) {
    const proxy = proxies.find((p) => p.id === key.proxy_id);
    return proxy ? `Proxy: ${proxy.url}` : 'Missing proxy';
  }
  if (key.proxy_pool_id) {
    const pool = proxyPools.find((p) => p.id === key.proxy_pool_id);
    return pool ? `Pool: ${pool.name}` : 'Missing pool';
  }
  return 'Provider default';
}

function HealthBadge({ status }: { status?: ApiKeyRuntimeStatus }) {
  if (!status) {
    return (
      <span className="inline-flex items-center px-2 py-1 rounded-full bg-apple-gray-100 text-apple-gray-500 text-xs font-medium">
        Not checked
      </span>
    );
  }

  if (status.quota_status === 'limited') {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full bg-orange-100 text-apple-orange text-xs font-medium" title={status.message || undefined}>
        <XCircleIcon className="w-3.5 h-3.5" />
        Quota limited
      </span>
    );
  }

  if (status.is_healthy) {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full bg-green-100 text-apple-green text-xs font-medium">
        <CheckCircleIcon className="w-3.5 h-3.5" />
        Healthy
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full bg-red-100 text-apple-red text-xs font-medium" title={status.message || undefined}>
      <XCircleIcon className="w-3.5 h-3.5" />
      Failed
    </span>
  );
}

function BindingSelect({
  value,
  proxies,
  proxyPools,
  onChange,
}: {
  value: string;
  proxies: Proxy[];
  proxyPools: ProxyPool[];
  onChange: (value: string) => void;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="input text-xs py-1.5 px-2"
    >
      <option value="">Provider default</option>
      {proxyPools.map((pool) => (
        <option key={pool.id} value={`pool:${pool.id}`}>
          Pool: {pool.name} ({pool.active_proxy_count}/{pool.proxy_count})
        </option>
      ))}
      {proxies.map((proxy) => (
        <option key={proxy.id} value={`proxy:${proxy.id}`}>
          Proxy: {proxy.url}{proxy.region ? ` (${proxy.region})` : ''}
        </option>
      ))}
    </select>
  );
}
