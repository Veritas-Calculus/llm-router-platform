import { useState, useCallback, useEffect, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { Provider, ProviderApiKey, ProviderHealthStatus, Proxy } from '@/lib/types';
import {
  PROVIDERS_QUERY,
  PROVIDER_API_KEYS_QUERY,
  PROXIES_QUERY,
  CREATE_PROVIDER,
  DELETE_PROVIDER,
  UPDATE_PROVIDER,
  TOGGLE_PROVIDER,
  TOGGLE_PROVIDER_PROXY,
  CREATE_PROVIDER_API_KEY,
  UPDATE_PROVIDER_API_KEY,
  TOGGLE_PROVIDER_API_KEY,
  DELETE_PROVIDER_API_KEY,
} from '@/lib/graphql/operations';

/* eslint-disable @typescript-eslint/no-explicit-any */

// Map GraphQL camelCase → snake_case for backward compat
function mapProvider(d: any): Provider {
  return {
    id: d.id, name: d.name, base_url: d.baseUrl,
    is_active: d.isActive, priority: d.priority, weight: d.weight,
    max_retries: d.maxRetries, timeout: d.timeout, use_proxy: d.useProxy,
    default_proxy_id: d.defaultProxyId, requires_api_key: d.requiresApiKey,
    created_at: d.createdAt,
  };
}
function mapApiKey(d: any): ProviderApiKey {
  return {
    id: d.id, provider_id: d.providerId, alias: d.alias,
    key_prefix: d.keyPrefix, is_active: d.isActive, priority: d.priority,
    weight: d.weight, rate_limit: d.rateLimit, usage_count: d.usageCount,
    last_used_at: d.lastUsedAt, created_at: d.createdAt,
  };
}
function mapProxy(d: any): Proxy {
  return {
    id: d.id, url: d.url, type: d.type, region: d.region,
    is_active: d.isActive, weight: d.weight,
    success_count: d.successCount, failure_count: d.failureCount,
    avg_latency: d.avgLatency, last_checked: d.lastChecked,
    created_at: d.createdAt, has_auth: d.hasAuth,
    upstream_proxy_id: d.upstreamProxyId, username: d.username || '',
  };
}

export function useProviders() {
  const { data: providersData, loading: providersLoading, refetch: refetchProviders } = useQuery<any>(PROVIDERS_QUERY);
  const { data: proxiesData } = useQuery<any>(PROXIES_QUERY);
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(null);

  const providers = useMemo(() => (providersData?.providers || []).map(mapProvider), [providersData]);
  const proxies = useMemo(() => (proxiesData?.proxies || []).map(mapProxy), [proxiesData]);

  // Auto-select first provider
  useEffect(() => {
    if (providers.length > 0 && !selectedProviderId) {
      setSelectedProviderId(providers[0].id);
    }
  }, [providers, selectedProviderId]);

  const selectedProvider = useMemo(
    () => providers.find((p: Provider) => p.id === selectedProviderId) || null,
    [providers, selectedProviderId]
  );

  // API Keys query — skip if no provider selected
  const { data: keysData, refetch: refetchKeys } = useQuery<any>(PROVIDER_API_KEYS_QUERY, {
    variables: { providerId: selectedProviderId || '' },
    skip: !selectedProviderId,
  });
  const apiKeys = useMemo(() => (keysData?.providerApiKeys || []).map(mapApiKey), [keysData]);

  const [testing, setTesting] = useState(false);
  const [healthStatus, setHealthStatus] = useState<ProviderHealthStatus | null>(null);

  // Clear health status when switching providers
  useEffect(() => {
    setHealthStatus(null);
    setTesting(false);
  }, [selectedProviderId]);
  const setSelectedProvider = useCallback((p: Provider | null) => {
    setSelectedProviderId(p?.id || null);
    setHealthStatus(null);
    setTesting(false);
  }, []);
  const [savingProxy, setSavingProxy] = useState(false);
  const loading = providersLoading;

  // ── Mutations ──
  const [createProviderMut] = useMutation(CREATE_PROVIDER);
  const [deleteProviderMut] = useMutation(DELETE_PROVIDER);
  const [updateProviderMut] = useMutation(UPDATE_PROVIDER);
  const [toggleProviderMut] = useMutation(TOGGLE_PROVIDER);
  const [toggleProxyMut] = useMutation(TOGGLE_PROVIDER_PROXY);
  const [createKeyMut] = useMutation(CREATE_PROVIDER_API_KEY);
  const [updateKeyMut] = useMutation(UPDATE_PROVIDER_API_KEY);
  const [toggleKeyMut] = useMutation(TOGGLE_PROVIDER_API_KEY);
  const [deleteKeyMut] = useMutation(DELETE_PROVIDER_API_KEY);

  const handleProxyChange = useCallback(async (proxyId: string) => {
    if (!selectedProvider) return;
    setSavingProxy(true);
    try {
      await updateProviderMut({
        variables: { id: selectedProvider.id, input: { defaultProxyId: proxyId || null } },
      });
      await refetchProviders();
      toast.success(proxyId ? 'Default proxy updated' : 'Default proxy cleared');
    } catch { toast.error('Failed to update proxy'); }
    finally { setSavingProxy(false); }
  }, [selectedProvider, updateProviderMut, refetchProviders]);

  const handleCreateProvider = useCallback(async (data: { name: string; baseUrl: string; requiresApiKey?: boolean }) => {
    const { data: result } = await createProviderMut({
      variables: {
        input: {
          name: data.name,
          baseUrl: data.baseUrl,
          requiresApiKey: data.requiresApiKey ?? true,
        },
      },
    });
    await refetchProviders();
    const created = (result as any)?.createProvider;
    if (created) {
      setSelectedProviderId(created.id);
    }
    toast.success('Provider created');
  }, [createProviderMut, refetchProviders]);

  const handleDeleteProvider = useCallback(async (id: string) => {
    await deleteProviderMut({ variables: { id } });
    await refetchProviders();
    setSelectedProviderId(null);
    toast.success('Provider deleted');
  }, [deleteProviderMut, refetchProviders]);

  const handleToggleProvider = useCallback(async (provider: Provider) => {
    try {
      const { data } = await toggleProviderMut({ variables: { id: provider.id } });
      await refetchProviders();
      toast.success(`${provider.name} ${(data as any)?.toggleProvider?.isActive ? 'enabled' : 'disabled'}`);
    } catch { toast.error('Failed to toggle provider'); }
  }, [toggleProviderMut, refetchProviders]);

  const handleTestConnection = useCallback(async () => {
    if (!selectedProvider) return;
    setTesting(true);
    setHealthStatus(null);
    try {
      const client = (await import('@/lib/graphql/client')).apolloClient;
      const { CHECK_PROVIDER_HEALTH } = await import('@/lib/graphql/operations');
      const { data } = await client.mutate<any>({ mutation: CHECK_PROVIDER_HEALTH, variables: { id: selectedProvider.id } });
      const status = data?.checkProviderHealth;
      if (status) {
        const errorMsg = status.isHealthy ? '' : (status.errorMessage || 'Connection failed');
        const mapped: ProviderHealthStatus = {
          id: status.id, name: status.name, base_url: status.baseUrl || selectedProvider.base_url,
          is_active: status.isActive ?? selectedProvider.is_active, is_healthy: status.isHealthy, use_proxy: status.useProxy ?? selectedProvider.use_proxy,
          response_time: status.responseTime, last_check: status.lastCheck || new Date().toISOString(), success_rate: status.successRate || 0,
          error_message: errorMsg,
        };
        setHealthStatus(mapped);
        if (status.isHealthy) {
          toast.success(`Connection successful! Latency: ${status.responseTime}ms`);
        } else {
          toast.error(errorMsg || 'Connection failed');
        }
      }
    } catch (err: any) {
      const errMsg = err?.message || 'Failed to test connection';
      toast.error(errMsg);
      setHealthStatus({
        id: selectedProvider.id, name: selectedProvider.name, base_url: selectedProvider.base_url,
        is_active: selectedProvider.is_active, is_healthy: false, use_proxy: selectedProvider.use_proxy,
        response_time: 0, last_check: new Date().toISOString(), success_rate: 0,
        error_message: errMsg,
      });
    }
    finally { setTesting(false); }
  }, [selectedProvider]);

  const handleToggleProxy = useCallback(async () => {
    if (!selectedProvider) return;
    try {
      const { data } = await toggleProxyMut({ variables: { id: selectedProvider.id } });
      await refetchProviders();
      toast.success(`Proxy ${(data as any)?.toggleProviderProxy?.useProxy ? 'enabled' : 'disabled'} for ${selectedProvider.name}`);
    } catch { toast.error('Failed to toggle proxy'); }
  }, [selectedProvider, toggleProxyMut, refetchProviders]);

  const handleToggleRequiresApiKey = useCallback(async () => {
    if (!selectedProvider) return;
    try {
      await updateProviderMut({
        variables: { id: selectedProvider.id, input: { requiresApiKey: !selectedProvider.requires_api_key } },
      });
      await refetchProviders();
      toast.success(`API Key requirement ${!selectedProvider.requires_api_key ? 'enabled' : 'disabled'}`);
    } catch { toast.error('Failed to update API key requirement'); }
  }, [selectedProvider, updateProviderMut, refetchProviders]);

  const handleSaveEndpoint = useCallback(async (url: string) => {
    if (!selectedProvider) return;
    await updateProviderMut({ variables: { id: selectedProvider.id, input: { baseUrl: url } } });
    await refetchProviders();
    toast.success('Endpoint updated successfully');
  }, [selectedProvider, updateProviderMut, refetchProviders]);

  const handleAddKey = useCallback(async (data: { api_key: string; alias: string; priority: number; weight: number; rate_limit: number }) => {
    if (!selectedProvider) return;
    await createKeyMut({
      variables: {
        providerId: selectedProvider.id,
        input: { apiKey: data.api_key, alias: data.alias, priority: data.priority, weight: data.weight, rateLimit: data.rate_limit },
      },
    });
    await refetchKeys();
    toast.success('API key added');
  }, [selectedProvider, createKeyMut, refetchKeys]);

  const handleUpdateKey = useCallback(async (keyId: string, data: { priority: number; weight: number; rate_limit: number }) => {
    if (!selectedProvider) return;
    await updateKeyMut({
      variables: {
        providerId: selectedProvider.id, keyId,
        input: { priority: data.priority, weight: data.weight, rateLimit: data.rate_limit },
      },
    });
    await refetchKeys();
    toast.success('API key updated');
  }, [selectedProvider, updateKeyMut, refetchKeys]);

  const handleToggleKey = useCallback(async (key: ProviderApiKey) => {
    if (!selectedProvider) return;
    try {
      const { data } = await toggleKeyMut({ variables: { providerId: selectedProvider.id, keyId: key.id } });
      await refetchKeys();
      toast.success(`API key ${(data as any)?.toggleProviderApiKey?.isActive ? 'enabled' : 'disabled'}`);
    } catch { toast.error('Failed to toggle API key'); }
  }, [selectedProvider, toggleKeyMut, refetchKeys]);

  const handleDeleteKey = useCallback(async (keyId: string) => {
    if (!selectedProvider) return;
    await deleteKeyMut({ variables: { providerId: selectedProvider.id, keyId } });
    await refetchKeys();
    toast.success('API key deleted');
  }, [selectedProvider, deleteKeyMut, refetchKeys]);

  return {
    providers, selectedProvider, setSelectedProvider,
    apiKeys, proxies, loading, testing, healthStatus, savingProxy,
    handleCreateProvider, handleDeleteProvider,
    handleToggleProvider, handleTestConnection, handleToggleProxy,
    handleProxyChange, handleToggleRequiresApiKey, handleSaveEndpoint,
    handleAddKey, handleUpdateKey, handleToggleKey, handleDeleteKey,
  };
}
