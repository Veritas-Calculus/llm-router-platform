import { useState, useCallback, useEffect, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { Provider, ProviderApiKey, ProviderHealthStatus, Proxy } from '@/lib/types';
import { t } from '@/lib/i18n';
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
  CHECK_PROVIDER_HEALTH,
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

function mapProviderHealthStatus(status: any, fallback: Provider): ProviderHealthStatus {
  const errorMsg = status?.isHealthy ? '' : (status?.errorMessage || 'Connection failed');
  return {
    id: status?.id || fallback.id,
    name: status?.name || fallback.name,
    base_url: status?.baseUrl || fallback.base_url,
    is_active: status?.isActive ?? fallback.is_active,
    is_healthy: Boolean(status?.isHealthy),
    use_proxy: status?.useProxy ?? fallback.use_proxy,
    response_time: status?.responseTime ?? 0,
    last_check: status?.lastCheck || new Date().toISOString(),
    success_rate: status?.successRate || 0,
    error_message: errorMsg,
  };
}

type CreateProviderData = {
  name: string;
  baseUrl: string;
  requiresApiKey?: boolean;
  apiKey?: string;
  apiKeyAlias?: string;
  validateConnection?: boolean;
};

export function useProviders() {
  const { data: providersData, loading: providersLoading, refetch: refetchProviders } = useQuery<any>(PROVIDERS_QUERY, {
    fetchPolicy: 'cache-and-network',
  });
  const { data: proxiesData } = useQuery<any>(PROXIES_QUERY);
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(null);
  const [localProviders, setLocalProviders] = useState<Provider[]>([]);

  const remoteProviders = useMemo(() => (providersData?.providers || []).map(mapProvider), [providersData]);
  const providers = useMemo(() => {
    const remoteIds = new Set(remoteProviders.map((p: Provider) => p.id));
    const merged = [
      ...remoteProviders,
      ...localProviders.filter((p) => !remoteIds.has(p.id)),
    ];
    return merged.sort((a, b) => (
      b.priority - a.priority ||
      new Date(b.created_at).getTime() - new Date(a.created_at).getTime() ||
      a.name.localeCompare(b.name)
    ));
  }, [remoteProviders, localProviders]);
  const proxies = useMemo(() => (proxiesData?.proxies || []).map(mapProxy), [proxiesData]);

  useEffect(() => {
    if (remoteProviders.length === 0) return;
    const remoteIds = new Set(remoteProviders.map((p: Provider) => p.id));
    setLocalProviders((prev) => {
      const next = prev.filter((p) => !remoteIds.has(p.id));
      return next.length === prev.length ? prev : next;
    });
  }, [remoteProviders]);

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
  const [checkProviderHealthMut] = useMutation(CHECK_PROVIDER_HEALTH);

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

  const handleCreateProvider = useCallback(async (data: CreateProviderData) => {
    const requiresKey = data.requiresApiKey ?? true;
    const { data: result } = await createProviderMut({
      variables: {
        input: {
          name: data.name,
          baseUrl: data.baseUrl,
          requiresApiKey: requiresKey,
          isActive: !requiresKey,
        },
      },
    });
    const created = (result as any)?.createProvider;
    let createdProvider = created ? mapProvider(created) : null;
    let apiKeySaved = false;
    let apiKeyError: unknown = null;

    if (createdProvider) {
      setLocalProviders((prev) => {
        if (prev.some((p) => p.id === createdProvider!.id)) return prev;
        return [...prev, createdProvider!];
      });
      setSelectedProviderId(createdProvider.id);
    }

    if (created?.id && requiresKey && data.apiKey?.trim()) {
      try {
        await createKeyMut({
          variables: {
            providerId: created.id,
            input: {
              apiKey: data.apiKey.trim(),
              alias: data.apiKeyAlias?.trim() || `${data.name} primary`,
              priority: 1,
              weight: 1.0,
              rateLimit: 0,
            },
          },
        });
        apiKeySaved = true;
        await updateProviderMut({
          variables: {
            id: created.id,
            input: { isActive: true },
          },
        });
        if (createdProvider) {
          createdProvider = { ...createdProvider, is_active: true };
          setLocalProviders((prev) => prev.map((p) => (
            p.id === createdProvider!.id ? createdProvider! : p
          )));
        }
      } catch (err) {
        apiKeyError = err;
      }
    }

    const refreshed = await refetchProviders();
    if (created?.id) {
      setSelectedProviderId(created.id);
      const refreshedProvider = refreshed.data?.providers?.find((p: any) => p.id === created.id);
      if (refreshedProvider) {
        createdProvider = mapProvider(refreshedProvider);
        setLocalProviders((prev) => prev.filter((p) => p.id !== createdProvider!.id));
      }
    }

    if (created) {
      if (apiKeyError) {
        toast.error(t('providers.providerCreatedKeyFailed', { error: (apiKeyError as Error)?.message || 'unknown error' }));
      } else {
        toast.success(apiKeySaved ? t('providers.providerAndKeyCreated') : t('providers.providerCreated'));
      }
    }

    if (created?.id && data.validateConnection && (!requiresKey || apiKeySaved) && createdProvider) {
      setTesting(true);
      setHealthStatus(null);
      try {
        const { data: healthData } = await checkProviderHealthMut({
          variables: { id: created.id },
        });
        const status = (healthData as any)?.checkProviderHealth;
        if (status) {
          const mapped = mapProviderHealthStatus(status, createdProvider);
          setHealthStatus(mapped);
          if (mapped.is_healthy) {
            toast.success(t('providers.connectionSuccessful', { latency: mapped.response_time }));
          } else {
            toast.error(mapped.error_message || 'Connection failed');
          }
        }
      } catch (err: any) {
        const errMsg = err?.message || 'Failed to test connection';
        setHealthStatus({
          id: createdProvider.id,
          name: createdProvider.name,
          base_url: createdProvider.base_url,
          is_active: createdProvider.is_active,
          is_healthy: false,
          use_proxy: createdProvider.use_proxy,
          response_time: 0,
          last_check: new Date().toISOString(),
          success_rate: 0,
          error_message: errMsg,
        });
        toast.error(errMsg);
      } finally {
        setTesting(false);
      }
    }
  }, [createProviderMut, createKeyMut, updateProviderMut, refetchProviders, checkProviderHealthMut]);

  const handleDeleteProvider = useCallback(async (id: string) => {
    await deleteProviderMut({ variables: { id } });
    await refetchProviders();
    setLocalProviders((prev) => prev.filter((p) => p.id !== id));
    setSelectedProviderId(null);
    toast.success('Provider deleted');
  }, [deleteProviderMut, refetchProviders]);

  const handleToggleProvider = useCallback(async (provider: Provider) => {
    try {
      const { data } = await toggleProviderMut({ variables: { id: provider.id } });
      await refetchProviders();
      const isActive = (data as any)?.toggleProvider?.isActive;
      setLocalProviders((prev) => prev.map((p) => (
        p.id === provider.id ? { ...p, is_active: isActive ?? !provider.is_active } : p
      )));
      toast.success(`${provider.name} ${(data as any)?.toggleProvider?.isActive ? 'enabled' : 'disabled'}`);
    } catch { toast.error('Failed to toggle provider'); }
  }, [toggleProviderMut, refetchProviders]);

  const handleTestConnection = useCallback(async () => {
    if (!selectedProvider) return;
    setTesting(true);
    setHealthStatus(null);
    try {
      const { data } = await checkProviderHealthMut({ variables: { id: selectedProvider.id } });
      const status = (data as any)?.checkProviderHealth;
      if (status) {
        const mapped = mapProviderHealthStatus(status, selectedProvider);
        setHealthStatus(mapped);
        if (mapped.is_healthy) {
          toast.success(t('providers.connectionSuccessful', { latency: mapped.response_time }));
        } else {
          toast.error(mapped.error_message || 'Connection failed');
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
  }, [selectedProvider, checkProviderHealthMut]);

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

  const handleUpdateProviderSettings = useCallback(async (input: {
    priority: number;
    weight: number;
    maxRetries: number;
    timeout: number;
  }) => {
    if (!selectedProvider) return;
    try {
      const { data } = await updateProviderMut({
        variables: {
          id: selectedProvider.id,
          input: {
            priority: input.priority,
            weight: input.weight,
            maxRetries: input.maxRetries,
            timeout: input.timeout,
          },
        },
      });
      const updated = (data as any)?.updateProvider;
      if (updated) {
        const mapped = mapProvider(updated);
        setLocalProviders((prev) => [
          ...prev.filter((p) => p.id !== mapped.id),
          mapped,
        ]);
      }
      await refetchProviders();
      toast.success(t('providers.settingsUpdated'));
    } catch (err: any) {
      toast.error(err?.message || t('providers.settingsUpdateFailed'));
    }
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
    handleUpdateProviderSettings,
    handleAddKey, handleUpdateKey, handleToggleKey, handleDeleteKey,
  };
}
