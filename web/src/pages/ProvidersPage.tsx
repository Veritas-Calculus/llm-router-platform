import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { providersApi, proxiesApi, Provider, ProviderApiKey, ProviderHealthStatus, Proxy } from '@/lib/api';
import ProviderList from '@/components/providers/ProviderList';
import ProviderInfoCard from '@/components/providers/ProviderInfoCard';
import ApiKeyTable from '@/components/providers/ApiKeyTable';
import LocalProviderCard from '@/components/providers/LocalProviderCard';

function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<Provider | null>(null);
  const [apiKeys, setApiKeys] = useState<ProviderApiKey[]>([]);
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [loading, setLoading] = useState(true);
  const [testing, setTesting] = useState(false);
  const [healthStatus, setHealthStatus] = useState<ProviderHealthStatus | null>(null);
  const [savingProxy, setSavingProxy] = useState(false);

  useEffect(() => {
    loadProviders();
    loadProxies();
  }, []);

  useEffect(() => {
    if (selectedProvider) {
      loadApiKeys(selectedProvider.id);
      setHealthStatus(null);
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

  const loadProxies = async () => {
    try {
      const response = await proxiesApi.list();
      setProxies(response?.data || []);
    } catch (error) {
      console.error('Failed to load proxies:', error);
      setProxies([]);
    }
  };

  const updateProvider = (updated: Provider) => {
    setProviders((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
    if (selectedProvider?.id === updated.id) {
      setSelectedProvider(updated);
    }
  };

  const handleProxyChange = async (proxyId: string) => {
    if (!selectedProvider) return;
    setSavingProxy(true);
    try {
      const updated = await providersApi.update(selectedProvider.id, {
        default_proxy_id: proxyId || null,
      });
      updateProvider(updated);
      toast.success(proxyId ? 'Default proxy updated' : 'Default proxy cleared');
    } catch (error) {
      toast.error('Failed to update proxy');
    } finally {
      setSavingProxy(false);
    }
  };

  const handleToggleProvider = async (provider: Provider) => {
    try {
      const updated = await providersApi.toggle(provider.id);
      updateProvider(updated);
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
        toast.success(`Connection successful! Latency: ${status.response_time}ms`);
      } else {
        toast.error(`Connection failed: ${status.error_message || 'Unknown error'}`);
      }
    } catch (error) {
      toast.error('Failed to test connection');
      setHealthStatus({
        id: selectedProvider.id,
        name: selectedProvider.name,
        base_url: selectedProvider.base_url,
        is_active: selectedProvider.is_active,
        is_healthy: false,
        use_proxy: selectedProvider.use_proxy,
        response_time: 0,
        last_check: new Date().toISOString(),
        success_rate: 0,
        error_message: 'Failed to test connection',
      });
    } finally {
      setTesting(false);
    }
  };

  const handleToggleProxy = async () => {
    if (!selectedProvider) return;
    try {
      const updated = await providersApi.toggleProxy(selectedProvider.id);
      updateProvider(updated);
      toast.success(`Proxy ${updated.use_proxy ? 'enabled' : 'disabled'} for ${selectedProvider.name}`);
    } catch (error) {
      toast.error('Failed to toggle proxy');
    }
  };

  const handleToggleRequiresApiKey = async () => {
    if (!selectedProvider) return;
    try {
      const updated = await providersApi.update(selectedProvider.id, {
        requires_api_key: !selectedProvider.requires_api_key,
      });
      updateProvider(updated);
      toast.success(
        `API Key requirement ${updated.requires_api_key ? 'enabled' : 'disabled'} for ${selectedProvider.name}`
      );
    } catch (error) {
      toast.error('Failed to update API key requirement');
    }
  };

  const handleSaveEndpoint = async (url: string) => {
    if (!selectedProvider) return;
    const updated = await providersApi.update(selectedProvider.id, { base_url: url });
    updateProvider(updated);
    toast.success('Endpoint updated successfully');
  };

  const handleAddKey = async (data: { api_key: string; alias: string; priority: number; weight: number; rate_limit: number }) => {
    if (!selectedProvider) return;
    const key = await providersApi.createApiKey(selectedProvider.id, data);
    setApiKeys((prev) => [...prev, key]);
    toast.success('API key added');
  };

  const handleUpdateKey = async (keyId: string, data: { priority: number; weight: number; rate_limit: number }) => {
    if (!selectedProvider) return;
    const updated = await providersApi.updateApiKey(selectedProvider.id, keyId, data);
    setApiKeys((prev) => prev.map((k) => (k.id === keyId ? updated : k)));
    toast.success('API key updated');
  };

  const handleToggleKey = async (key: ProviderApiKey) => {
    if (!selectedProvider) return;
    try {
      const updated = await providersApi.toggleApiKey(selectedProvider.id, key.id);
      setApiKeys((prev) => prev.map((k) => (k.id === key.id ? updated : k)));
      toast.success(`API key ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch (error) {
      toast.error('Failed to toggle API key');
    }
  };

  const handleDeleteKey = async (keyId: string) => {
    if (!selectedProvider) return;
    await providersApi.deleteApiKey(selectedProvider.id, keyId);
    setApiKeys((prev) => prev.filter((k) => k.id !== keyId));
    toast.success('API key deleted');
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
        <ProviderList
          providers={providers}
          selectedProvider={selectedProvider}
          onSelect={setSelectedProvider}
          onToggle={handleToggleProvider}
        />

        <motion.div
          initial={{ opacity: 0, x: 10 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-3"
        >
          {selectedProvider && (
            <div className="space-y-6">
              <ProviderInfoCard
                provider={selectedProvider}
                proxies={proxies}
                healthStatus={healthStatus}
                testing={testing}
                savingProxy={savingProxy}
                onTestConnection={handleTestConnection}
                onToggleProxy={handleToggleProxy}
                onProxyChange={handleProxyChange}
              />

              {selectedProvider.requires_api_key && (
                <ApiKeyTable
                  providerName={selectedProvider.name}
                  apiKeys={apiKeys}
                  onAddKey={handleAddKey}
                  onUpdateKey={handleUpdateKey}
                  onToggleKey={handleToggleKey}
                  onDeleteKey={handleDeleteKey}
                />
              )}

              {['ollama', 'lmstudio', 'vllm'].includes(selectedProvider.name) && (
                <LocalProviderCard
                  provider={selectedProvider}
                  onToggleRequiresApiKey={handleToggleRequiresApiKey}
                  onSaveEndpoint={handleSaveEndpoint}
                />
              )}
            </div>
          )}
        </motion.div>
      </div>
    </div>
  );
}

export default ProvidersPage;
