import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  PlusIcon,
  TrashIcon,
  PlayIcon,
  CheckCircleIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { providersApi, Provider, ProviderApiKey, ProviderHealthStatus } from '@/lib/api';

interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText: string;
  confirmColor: 'red' | 'orange';
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

function ConfirmModal({
  isOpen,
  title,
  message,
  confirmText,
  confirmColor,
  onConfirm,
  onCancel,
  loading,
}: ConfirmModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="bg-white rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
      >
        <div className="flex items-start gap-4">
          <div
            className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${
              confirmColor === 'red' ? 'bg-red-100' : 'bg-orange-100'
            }`}
          >
            <ExclamationTriangleIcon
              className={`w-6 h-6 ${confirmColor === 'red' ? 'text-apple-red' : 'text-apple-orange'}`}
            />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-apple-gray-900">{title}</h3>
            <p className="mt-2 text-sm text-apple-gray-600">{message}</p>
          </div>
        </div>
        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onCancel} className="btn btn-secondary" disabled={loading}>
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className={`btn ${confirmColor === 'red' ? 'btn-danger' : 'bg-apple-orange text-white hover:opacity-90'}`}
            disabled={loading}
          >
            {loading ? 'Processing...' : confirmText}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<Provider | null>(null);
  const [apiKeys, setApiKeys] = useState<ProviderApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState({ api_key: '', alias: '' });
  const [adding, setAdding] = useState(false);
  const [testing, setTesting] = useState(false);
  const [healthStatus, setHealthStatus] = useState<ProviderHealthStatus | null>(null);
  const [editingEndpoint, setEditingEndpoint] = useState(false);
  const [endpointValue, setEndpointValue] = useState('');
  const [savingEndpoint, setSavingEndpoint] = useState(false);
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    type: 'delete';
    keyId: string;
  }>({ isOpen: false, type: 'delete', keyId: '' });
  const [processing, setProcessing] = useState(false);

  useEffect(() => {
    loadProviders();
  }, []);

  useEffect(() => {
    if (selectedProvider) {
      loadApiKeys(selectedProvider.id);
      setHealthStatus(null);
      setEditingEndpoint(false);
      setEndpointValue(selectedProvider.base_url);
    }
  }, [selectedProvider]);

  const loadProviders = async () => {
    try {
      const response = await providersApi.list();
      const data = response?.data || [];
      setProviders(data);
      if (data.length > 0) {
        setSelectedProvider(data[0]);
      }
    } catch (error) {
      toast.error('Failed to load providers');
      setProviders([]);
    } finally {
      setLoading(false);
    }
  };

  const loadApiKeys = async (providerId: string) => {
    try {
      const response = await providersApi.getApiKeys(providerId);
      setApiKeys(response?.data || []);
    } catch (error) {
      console.error('Failed to load API keys:', error);
      setApiKeys([]);
    }
  };

  const handleToggleProvider = async (provider: Provider) => {
    try {
      const updated = await providersApi.toggle(provider.id);
      setProviders((prev) =>
        prev.map((p) => (p.id === provider.id ? updated : p))
      );
      if (selectedProvider?.id === provider.id) {
        setSelectedProvider(updated);
      }
      toast.success(`${provider.name} ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch (error) {
      toast.error('Failed to toggle provider');
    }
  };

  const handleTestConnection = async () => {
    if (!selectedProvider) return;

    setTesting(true);
    setHealthStatus(null);
    try {
      const status = await providersApi.checkHealth(selectedProvider.name);
      setHealthStatus(status);
      if (status.is_healthy) {
        toast.success(`Connection successful! Latency: ${Math.round(status.latency / 1000000)}ms`);
      } else {
        toast.error('Connection failed');
      }
    } catch (error) {
      toast.error('Failed to test connection');
      setHealthStatus({
        provider_name: selectedProvider.name,
        is_healthy: false,
        latency: 0,
        last_checked: new Date().toISOString(),
      });
    } finally {
      setTesting(false);
    }
  };

  const handleToggleProxy = async () => {
    if (!selectedProvider) return;

    try {
      const updated = await providersApi.toggleProxy(selectedProvider.id);
      setProviders((prev) =>
        prev.map((p) => (p.id === selectedProvider.id ? updated : p))
      );
      setSelectedProvider(updated);
      toast.success(`Proxy ${updated.use_proxy ? 'enabled' : 'disabled'} for ${selectedProvider.name}`);
    } catch (error) {
      toast.error('Failed to toggle proxy');
    }
  };

  const handleSaveEndpoint = async () => {
    if (!selectedProvider || !endpointValue.trim()) {
      toast.error('Please enter a valid endpoint URL');
      return;
    }

    setSavingEndpoint(true);
    try {
      const updated = await providersApi.update(selectedProvider.id, {
        base_url: endpointValue.trim(),
      });
      setProviders((prev) =>
        prev.map((p) => (p.id === selectedProvider.id ? updated : p))
      );
      setSelectedProvider(updated);
      setEditingEndpoint(false);
      toast.success('Endpoint updated successfully');
    } catch (error) {
      toast.error('Failed to update endpoint');
    } finally {
      setSavingEndpoint(false);
    }
  };

  const handleCancelEndpointEdit = () => {
    setEditingEndpoint(false);
    setEndpointValue(selectedProvider?.base_url || '');
  };

  const handleAddKey = async () => {
    if (!selectedProvider || !newKey.alias.trim()) {
      toast.error('Please enter an alias');
      return;
    }

    // Allow empty API key for local providers like Ollama
    const apiKeyValue = newKey.api_key.trim() || 'default';

    setAdding(true);
    try {
      const key = await providersApi.createApiKey(selectedProvider.id, {
        api_key: apiKeyValue,
        alias: newKey.alias.trim(),
      });
      setApiKeys((prev) => [...prev, key]);
      setShowAddModal(false);
      setNewKey({ api_key: '', alias: '' });
      toast.success('API key added');
    } catch (error) {
      toast.error('Failed to add API key');
    } finally {
      setAdding(false);
    }
  };

  const handleToggleKey = async (key: ProviderApiKey) => {
    if (!selectedProvider) return;

    try {
      const updated = await providersApi.toggleApiKey(selectedProvider.id, key.id);
      setApiKeys((prev) =>
        prev.map((k) => (k.id === key.id ? updated : k))
      );
      toast.success(`API key ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch (error) {
      toast.error('Failed to toggle API key');
    }
  };

  const openDeleteModal = (keyId: string) => {
    setConfirmModal({ isOpen: true, type: 'delete', keyId });
  };

  const closeConfirmModal = () => {
    setConfirmModal({ isOpen: false, type: 'delete', keyId: '' });
  };

  const handleConfirmDelete = async () => {
    if (!selectedProvider) return;

    setProcessing(true);
    try {
      await providersApi.deleteApiKey(selectedProvider.id, confirmModal.keyId);
      setApiKeys((prev) => prev.filter((k) => k.id !== confirmModal.keyId));
      toast.success('API key deleted');
      closeConfirmModal();
    } catch (error) {
      toast.error('Failed to delete API key');
    } finally {
      setProcessing(false);
    }
  };

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Providers</h1>
        <p className="text-apple-gray-500 mt-1">Manage LLM providers and their API keys</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* Provider List */}
        <motion.div
          initial={{ opacity: 0, x: -10 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-1"
        >
          <div className="card">
            <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Providers</h2>
            <div className="space-y-2">
              {providers.map((provider) => (
                <div
                  key={provider.id}
                  className={`flex items-center justify-between px-4 py-3 rounded-apple transition-colors cursor-pointer ${
                    selectedProvider?.id === provider.id
                      ? 'bg-apple-blue text-white'
                      : 'hover:bg-apple-gray-100 text-apple-gray-900'
                  }`}
                  onClick={() => setSelectedProvider(provider)}
                >
                  <div className="flex-1 min-w-0">
                    <p className="font-medium truncate">{provider.name}</p>
                    <p
                      className={`text-sm ${
                        selectedProvider?.id === provider.id
                          ? 'text-white/80'
                          : 'text-apple-gray-500'
                      }`}
                    >
                      Priority: {provider.priority}
                    </p>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleToggleProvider(provider);
                    }}
                    className={`ml-2 p-1 rounded-full transition-colors ${
                      selectedProvider?.id === provider.id
                        ? 'hover:bg-white/20'
                        : 'hover:bg-apple-gray-200'
                    }`}
                    title={provider.is_active ? 'Disable provider' : 'Enable provider'}
                  >
                    {provider.is_active ? (
                      <CheckCircleIcon
                        className={`w-5 h-5 ${
                          selectedProvider?.id === provider.id ? 'text-white' : 'text-apple-green'
                        }`}
                      />
                    ) : (
                      <XCircleIcon
                        className={`w-5 h-5 ${
                          selectedProvider?.id === provider.id ? 'text-white/60' : 'text-apple-gray-400'
                        }`}
                      />
                    )}
                  </button>
                </div>
              ))}
            </div>
          </div>
        </motion.div>

        {/* Provider Details */}
        <motion.div
          initial={{ opacity: 0, x: 10 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-3"
        >
          {selectedProvider && (
            <div className="space-y-6">
              {/* Provider Info Card */}
              <div className="card">
                <div className="flex items-start justify-between">
                  <div>
                    <h2 className="text-xl font-semibold text-apple-gray-900">
                      {selectedProvider.name}
                    </h2>
                    <p className="text-sm text-apple-gray-500 mt-1">
                      {selectedProvider.base_url}
                    </p>
                    <div className="flex items-center gap-4 mt-3">
                      <span
                        className={selectedProvider.is_active ? 'badge-success' : 'badge-error'}
                      >
                        {selectedProvider.is_active ? 'Enabled' : 'Disabled'}
                      </span>
                      <span className="text-sm text-apple-gray-500">
                        Timeout: {selectedProvider.timeout}s
                      </span>
                      <span className="text-sm text-apple-gray-500">
                        Retries: {selectedProvider.max_retries}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={handleTestConnection}
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
                      onClick={handleToggleProxy}
                      className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                        selectedProvider.use_proxy ? 'bg-apple-blue' : 'bg-apple-gray-200'
                      }`}
                    >
                      <span
                        className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                          selectedProvider.use_proxy ? 'translate-x-5' : 'translate-x-0'
                        }`}
                      />
                    </button>
                  </div>
                </div>

                {healthStatus && (
                  <div
                    className={`mt-4 p-4 rounded-apple ${
                      healthStatus.is_healthy
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
                        className={`font-medium ${
                          healthStatus.is_healthy ? 'text-apple-green' : 'text-apple-red'
                        }`}
                      >
                        {healthStatus.is_healthy ? 'Connection Successful' : 'Connection Failed'}
                      </span>
                      {healthStatus.is_healthy && (
                        <span className="text-sm text-apple-gray-500 ml-2">
                          Latency: {Math.round(healthStatus.latency / 1000000)}ms
                        </span>
                      )}
                    </div>
                  </div>
                )}
              </div>

              {/* API Keys Card - Only show if provider requires API key */}
              {selectedProvider.requires_api_key && (
              <div className="card">
                <div className="flex items-center justify-between mb-6">
                  <div>
                    <h3 className="text-lg font-semibold text-apple-gray-900">API Keys</h3>
                    <p className="text-sm text-apple-gray-500 mt-1">
                      Manage API keys for {selectedProvider.name}
                    </p>
                  </div>
                  <button onClick={() => setShowAddModal(true)} className="btn btn-primary">
                    <PlusIcon className="w-5 h-5 mr-2" />
                    Add Key
                  </button>
                </div>

                {apiKeys.length === 0 ? (
                  <div className="text-center py-12">
                    <p className="text-apple-gray-500 mb-4">No API keys for this provider</p>
                    <button onClick={() => setShowAddModal(true)} className="btn btn-primary">
                      Add your first key
                    </button>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="min-w-full divide-y divide-apple-gray-200">
                      <thead>
                        <tr>
                          <th className="table-header">Alias</th>
                          <th className="table-header">Key</th>
                          <th className="table-header">Status</th>
                          <th className="table-header">Usage</th>
                          <th className="table-header">Last Used</th>
                          <th className="table-header">Actions</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-apple-gray-100">
                        {apiKeys.map((key) => (
                          <tr key={key.id} className="hover:bg-apple-gray-50">
                            <td className="table-cell font-medium">{key.alias}</td>
                            <td className="table-cell">
                              <code className="text-sm bg-apple-gray-100 px-2 py-1 rounded">
                                {key.key_prefix}
                              </code>
                            </td>
                            <td className="table-cell">
                              <button
                                onClick={() => handleToggleKey(key)}
                                className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${
                                  key.is_active
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
                            <td className="table-cell text-apple-gray-500">
                              {key.usage_count.toLocaleString()} requests
                            </td>
                            <td className="table-cell text-apple-gray-500">
                              {key.last_used_at ? formatDate(key.last_used_at) : 'Never'}
                            </td>
                            <td className="table-cell">
                              <button
                                onClick={() => openDeleteModal(key.id)}
                                className="text-apple-red hover:text-red-600 transition-colors"
                                title="Delete API key"
                              >
                                <TrashIcon className="w-5 h-5" />
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
              )}

              {/* Local Provider Info Card - Show for Ollama/LM Studio */}
              {!selectedProvider.requires_api_key && (
                <div className="card">
                  <div className="flex items-center gap-3 mb-4">
                    <div className="flex-shrink-0 w-10 h-10 bg-apple-blue/10 rounded-full flex items-center justify-center">
                      <CheckCircleIcon className="w-6 h-6 text-apple-blue" />
                    </div>
                    <div>
                      <h3 className="text-lg font-semibold text-apple-gray-900">Local Provider</h3>
                      <p className="text-sm text-apple-gray-500">
                        {selectedProvider.name === 'ollama' ? 'Ollama' : 'LM Studio'} runs locally and does not require API keys.
                      </p>
                    </div>
                  </div>
                  
                  {/* Endpoint Configuration */}
                  <div className="border-t border-apple-gray-100 pt-4">
                    <div className="flex items-center justify-between mb-2">
                      <label className="text-sm font-medium text-apple-gray-900">
                        Endpoint URL
                      </label>
                      {!editingEndpoint && (
                        <button
                          onClick={() => setEditingEndpoint(true)}
                          className="text-sm text-apple-blue hover:text-blue-600 transition-colors"
                        >
                          Edit
                        </button>
                      )}
                    </div>
                    {editingEndpoint ? (
                      <div className="space-y-3">
                        <input
                          type="text"
                          value={endpointValue}
                          onChange={(e) => setEndpointValue(e.target.value)}
                          className="input w-full"
                          placeholder={selectedProvider.name === 'ollama' ? 'http://localhost:11434' : 'http://localhost:1234/v1'}
                        />
                        <div className="flex justify-end gap-2">
                          <button
                            onClick={handleCancelEndpointEdit}
                            className="btn btn-secondary text-sm"
                            disabled={savingEndpoint}
                          >
                            Cancel
                          </button>
                          <button
                            onClick={handleSaveEndpoint}
                            className="btn btn-primary text-sm"
                            disabled={savingEndpoint}
                          >
                            {savingEndpoint ? 'Saving...' : 'Save'}
                          </button>
                        </div>
                        <p className="text-xs text-apple-gray-500">
                          {selectedProvider.name === 'ollama' 
                            ? 'Default: http://localhost:11434 (use host.docker.internal:11434 when running in Docker)'
                            : 'Default: http://localhost:1234/v1 (use host.docker.internal:1234/v1 when running in Docker)'}
                        </p>
                      </div>
                    ) : (
                      <p className="text-sm text-apple-gray-600 font-mono bg-apple-gray-50 px-3 py-2 rounded-apple">
                        {selectedProvider.base_url}
                      </p>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </motion.div>
      </div>

      {/* Add Key Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-white rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">
              Add API Key for {selectedProvider?.name}
            </h2>
            <div className="space-y-4">
              <div>
                <label htmlFor="alias" className="label">
                  Alias
                </label>
                <input
                  type="text"
                  id="alias"
                  value={newKey.alias}
                  onChange={(e) => setNewKey((prev) => ({ ...prev, alias: e.target.value }))}
                  className="input"
                  placeholder="e.g., Primary, Backup, Team-A"
                />
              </div>
              <div>
                <label htmlFor="apiKey" className="label">
                  API Key
                </label>
                <input
                  type="password"
                  id="apiKey"
                  value={newKey.api_key}
                  onChange={(e) => setNewKey((prev) => ({ ...prev, api_key: e.target.value }))}
                  className="input"
                  placeholder="Enter the API key"
                />
                <p className="text-xs text-apple-gray-500 mt-1">
                  For Ollama/LM Studio, you can leave this empty or use any value.
                </p>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => {
                  setShowAddModal(false);
                  setNewKey({ api_key: '', alias: '' });
                }}
                className="btn btn-secondary"
              >
                Cancel
              </button>
              <button onClick={handleAddKey} className="btn btn-primary" disabled={adding}>
                {adding ? 'Adding...' : 'Add Key'}
              </button>
            </div>
          </motion.div>
        </div>
      )}

      {/* Confirm Delete Modal */}
      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title="Delete API Key"
        message="Are you sure you want to permanently delete this API key? This action cannot be undone."
        confirmText="Delete"
        confirmColor="red"
        onConfirm={handleConfirmDelete}
        onCancel={closeConfirmModal}
        loading={processing}
      />
    </div>
  );
}

export default ProvidersPage;
