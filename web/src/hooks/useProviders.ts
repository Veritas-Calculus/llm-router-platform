import { useEffect, useState, useCallback } from 'react';
import toast from 'react-hot-toast';
import { providersApi, proxiesApi, Provider, ProviderApiKey, ProviderHealthStatus, Proxy } from '@/lib/api';

/**
 * Custom hook encapsulating all ProvidersPage state and API logic.
 * This separates business logic from the rendering layer.
 */
export function useProviders() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<Provider | null>(null);
  const [apiKeys, setApiKeys] = useState<ProviderApiKey[]>([]);
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [loading, setLoading] = useState(true);
  const [testing, setTesting] = useState(false);
  const [healthStatus, setHealthStatus] = useState<ProviderHealthStatus | null>(null);
  const [savingProxy, setSavingProxy] = useState(false);

  const loadProviders = useCallback(async () => {
    try {
      const response = await providersApi.list();
      const data = response?.data || [];
      setProviders(data);
      if (data.length > 0) setSelectedProvider(data[0]);
    } catch {
      toast.error('Failed to load providers');
      setProviders([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadProxies = useCallback(async () => {
    try {
      const response = await proxiesApi.list();
      setProxies(response?.data || []);
    } catch {
      setProxies([]);
    }
  }, []);

  const loadApiKeys = useCallback(async (providerId: string) => {
    try {
      const response = await providersApi.getApiKeys(providerId);
      setApiKeys(response?.data || []);
    } catch {
      setApiKeys([]);
    }
  }, []);

  useEffect(() => { loadProviders(); loadProxies(); }, [loadProviders, loadProxies]);

  useEffect(() => {
    if (selectedProvider) {
      loadApiKeys(selectedProvider.id);
      setHealthStatus(null);
    }
  }, [selectedProvider, loadApiKeys]);

  const updateProvider = useCallback((updated: Provider) => {
    setProviders((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
    setSelectedProvider((prev) => (prev?.id === updated.id ? updated : prev));
  }, []);

  const handleProxyChange = useCallback(async (proxyId: string) => {
    if (!selectedProvider) return;
    setSavingProxy(true);
    try {
      const updated = await providersApi.update(selectedProvider.id, {
        default_proxy_id: proxyId || null,
      });
      updateProvider(updated);
      toast.success(proxyId ? 'Default proxy updated' : 'Default proxy cleared');
    } catch {
      toast.error('Failed to update proxy');
    } finally {
      setSavingProxy(false);
    }
  }, [selectedProvider, updateProvider]);

  const handleToggleProvider = useCallback(async (provider: Provider) => {
    try {
      const updated = await providersApi.toggle(provider.id);
      updateProvider(updated);
      toast.success(`${provider.name} ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch {
      toast.error('Failed to toggle provider');
    }
  }, [updateProvider]);

  const handleTestConnection = useCallback(async () => {
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
    } catch {
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
  }, [selectedProvider]);

  const handleToggleProxy = useCallback(async () => {
    if (!selectedProvider) return;
    try {
      const updated = await providersApi.toggleProxy(selectedProvider.id);
      updateProvider(updated);
      toast.success(`Proxy ${updated.use_proxy ? 'enabled' : 'disabled'} for ${selectedProvider.name}`);
    } catch {
      toast.error('Failed to toggle proxy');
    }
  }, [selectedProvider, updateProvider]);

  const handleToggleRequiresApiKey = useCallback(async () => {
    if (!selectedProvider) return;
    try {
      const updated = await providersApi.update(selectedProvider.id, {
        requires_api_key: !selectedProvider.requires_api_key,
      });
      updateProvider(updated);
      toast.success(
        `API Key requirement ${updated.requires_api_key ? 'enabled' : 'disabled'} for ${selectedProvider.name}`
      );
    } catch {
      toast.error('Failed to update API key requirement');
    }
  }, [selectedProvider, updateProvider]);

  const handleSaveEndpoint = useCallback(async (url: string) => {
    if (!selectedProvider) return;
    const updated = await providersApi.update(selectedProvider.id, { base_url: url });
    updateProvider(updated);
    toast.success('Endpoint updated successfully');
  }, [selectedProvider, updateProvider]);

  const handleAddKey = useCallback(async (data: { api_key: string; alias: string; priority: number; weight: number; rate_limit: number }) => {
    if (!selectedProvider) return;
    const key = await providersApi.createApiKey(selectedProvider.id, data);
    setApiKeys((prev) => [...prev, key]);
    toast.success('API key added');
  }, [selectedProvider]);

  const handleUpdateKey = useCallback(async (keyId: string, data: { priority: number; weight: number; rate_limit: number }) => {
    if (!selectedProvider) return;
    const updated = await providersApi.updateApiKey(selectedProvider.id, keyId, data);
    setApiKeys((prev) => prev.map((k) => (k.id === keyId ? updated : k)));
    toast.success('API key updated');
  }, [selectedProvider]);

  const handleToggleKey = useCallback(async (key: ProviderApiKey) => {
    if (!selectedProvider) return;
    try {
      const updated = await providersApi.toggleApiKey(selectedProvider.id, key.id);
      setApiKeys((prev) => prev.map((k) => (k.id === key.id ? updated : k)));
      toast.success(`API key ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch {
      toast.error('Failed to toggle API key');
    }
  }, [selectedProvider]);

  const handleDeleteKey = useCallback(async (keyId: string) => {
    if (!selectedProvider) return;
    await providersApi.deleteApiKey(selectedProvider.id, keyId);
    setApiKeys((prev) => prev.filter((k) => k.id !== keyId));
    toast.success('API key deleted');
  }, [selectedProvider]);

  return {
    providers,
    selectedProvider,
    setSelectedProvider,
    apiKeys,
    proxies,
    loading,
    testing,
    healthStatus,
    savingProxy,
    handleToggleProvider,
    handleTestConnection,
    handleToggleProxy,
    handleProxyChange,
    handleToggleRequiresApiKey,
    handleSaveEndpoint,
    handleAddKey,
    handleUpdateKey,
    handleToggleKey,
    handleDeleteKey,
  };
}
