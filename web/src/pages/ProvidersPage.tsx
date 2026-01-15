import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { PlusIcon, TrashIcon } from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { providersApi, Provider, ProviderApiKey } from '@/lib/api';

function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<Provider | null>(null);
  const [apiKeys, setApiKeys] = useState<ProviderApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState({ api_key: '', alias: '' });
  const [adding, setAdding] = useState(false);

  useEffect(() => {
    loadProviders();
  }, []);

  useEffect(() => {
    if (selectedProvider) {
      loadApiKeys(selectedProvider.id);
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

  const handleAddKey = async () => {
    if (!selectedProvider || !newKey.api_key.trim() || !newKey.alias.trim()) {
      toast.error('Please fill in all fields');
      return;
    }

    setAdding(true);
    try {
      const key = await providersApi.createApiKey(selectedProvider.id, newKey);
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

  const handleDeleteKey = async (keyId: string) => {
    if (!selectedProvider) return;
    if (!confirm('Are you sure you want to delete this API key?')) return;

    try {
      await providersApi.deleteApiKey(selectedProvider.id, keyId);
      setApiKeys((prev) => prev.filter((k) => k.id !== keyId));
      toast.success('API key deleted');
    } catch (error) {
      toast.error('Failed to delete API key');
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
        <p className="text-apple-gray-500 mt-1">Manage LLM provider API keys</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <motion.div
          initial={{ opacity: 0, x: -10 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-1"
        >
          <div className="card">
            <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Providers</h2>
            <div className="space-y-2">
              {providers.map((provider) => (
                <button
                  key={provider.id}
                  onClick={() => setSelectedProvider(provider)}
                  className={`w-full text-left px-4 py-3 rounded-apple transition-colors ${
                    selectedProvider?.id === provider.id
                      ? 'bg-apple-blue text-white'
                      : 'hover:bg-apple-gray-100 text-apple-gray-900'
                  }`}
                >
                  <p className="font-medium">{provider.name}</p>
                  <p
                    className={`text-sm ${
                      selectedProvider?.id === provider.id
                        ? 'text-white/80'
                        : 'text-apple-gray-500'
                    }`}
                  >
                    {provider.is_enabled ? 'Enabled' : 'Disabled'}
                  </p>
                </button>
              ))}
            </div>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, x: 10 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-3"
        >
          <div className="card">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-lg font-semibold text-apple-gray-900">
                  {selectedProvider?.name} API Keys
                </h2>
                <p className="text-sm text-apple-gray-500 mt-1">
                  {selectedProvider?.base_url}
                </p>
              </div>
              <button onClick={() => setShowAddModal(true)} className="btn-primary">
                <PlusIcon className="w-5 h-5 mr-2" />
                Add Key
              </button>
            </div>

            {apiKeys.length === 0 ? (
              <div className="text-center py-12">
                <p className="text-apple-gray-500 mb-4">No API keys for this provider</p>
                <button onClick={() => setShowAddModal(true)} className="btn-primary">
                  Add your first key
                </button>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-apple-gray-200">
                  <thead>
                    <tr>
                      <th className="table-header">Alias</th>
                      <th className="table-header">Status</th>
                      <th className="table-header">Priority</th>
                      <th className="table-header">Created</th>
                      <th className="table-header">Last Used</th>
                      <th className="table-header">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-apple-gray-100">
                    {apiKeys.map((key) => (
                      <tr key={key.id} className="hover:bg-apple-gray-50">
                        <td className="table-cell font-medium">{key.alias}</td>
                        <td className="table-cell">
                          <span
                            className={
                              key.status === 'active' ? 'badge-success' : 'badge-error'
                            }
                          >
                            {key.status}
                          </span>
                        </td>
                        <td className="table-cell">{key.priority}</td>
                        <td className="table-cell text-apple-gray-500">
                          {formatDate(key.created_at)}
                        </td>
                        <td className="table-cell text-apple-gray-500">
                          {key.last_used_at ? formatDate(key.last_used_at) : 'Never'}
                        </td>
                        <td className="table-cell">
                          <button
                            onClick={() => handleDeleteKey(key.id)}
                            className="text-apple-red hover:text-red-600 transition-colors"
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
        </motion.div>
      </div>

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
                  placeholder="e.g., Primary, Backup"
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
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => {
                  setShowAddModal(false);
                  setNewKey({ api_key: '', alias: '' });
                }}
                className="btn-secondary"
              >
                Cancel
              </button>
              <button onClick={handleAddKey} className="btn-primary" disabled={adding}>
                {adding ? 'Adding...' : 'Add Key'}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default ProvidersPage;
