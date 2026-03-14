import { useEffect, useState, useCallback } from 'react';
import toast from 'react-hot-toast';
import { healthApi, alertsApi, ApiKeyHealth, ProxyHealth, ProviderHealth, Alert } from '@/lib/api';

export type HealthTab = 'providers' | 'api-keys' | 'proxies' | 'alerts';

/**
 * Custom hook encapsulating all Health Monitor state and API logic.
 */
export function useHealth() {
  const [apiKeyHealth, setApiKeyHealth] = useState<ApiKeyHealth[]>([]);
  const [proxyHealth, setProxyHealth] = useState<ProxyHealth[]>([]);
  const [providerHealth, setProviderHealth] = useState<ProviderHealth[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [activeTab, setActiveTab] = useState<HealthTab>('providers');

  const loadHealthData = useCallback(async () => {
    try {
      const [apiKeysRes, proxiesRes, providersRes, alertsRes] = await Promise.all([
        healthApi.getApiKeysHealth(),
        healthApi.getProxiesHealth(),
        healthApi.getProvidersHealth(),
        alertsApi.list(),
      ]);
      setApiKeyHealth(apiKeysRes?.data || []);
      setProxyHealth(proxiesRes?.data || []);
      setProviderHealth(providersRes || []);
      setAlerts(alertsRes?.data || []);
    } catch {
      toast.error('Failed to load health data');
      setApiKeyHealth([]);
      setProxyHealth([]);
      setProviderHealth([]);
      setAlerts([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadHealthData(); }, [loadHealthData]);

  const refreshAll = useCallback(async () => {
    setRefreshing(true);
    await loadHealthData();
    setRefreshing(false);
    toast.success('Health data refreshed');
  }, [loadHealthData]);

  const checkApiKey = useCallback(async (id: string) => {
    try {
      const result = await healthApi.checkApiKey(id);
      setApiKeyHealth((prev) => prev.map((key) => (key.id === id ? result : key)));
      toast.success('API key checked');
    } catch {
      toast.error('Health check failed');
    }
  }, []);

  const checkProxy = useCallback(async (id: string) => {
    try {
      const result = await healthApi.checkProxy(id);
      setProxyHealth((prev) => prev.map((p) => (p.id === id ? result : p)));
      toast.success('Proxy checked');
    } catch {
      toast.error('Health check failed');
    }
  }, []);

  const checkProvider = useCallback(async (id: string) => {
    try {
      const result = await healthApi.checkProvider(id);
      setProviderHealth((prev) => prev.map((p) => (p.id === id ? result : p)));
      toast.success('Provider checked');
    } catch {
      toast.error('Health check failed');
    }
  }, []);

  const checkAllProviders = useCallback(async () => {
    try {
      await healthApi.checkAllProviders();
      await loadHealthData();
      toast.success('All providers checked');
    } catch {
      toast.error('Health check failed');
    }
  }, [loadHealthData]);

  const acknowledgeAlert = useCallback(async (id: string) => {
    try {
      await alertsApi.acknowledge(id);
      setAlerts((prev) =>
        prev.map((a) => a.id === id ? { ...a, status: 'acknowledged', acknowledged_at: new Date().toISOString() } : a)
      );
      toast.success('Alert acknowledged');
    } catch {
      toast.error('Failed to acknowledge alert');
    }
  }, []);

  const resolveAlert = useCallback(async (id: string) => {
    try {
      await alertsApi.resolve(id);
      setAlerts((prev) =>
        prev.map((a) => a.id === id ? { ...a, status: 'resolved', resolved_at: new Date().toISOString() } : a)
      );
      toast.success('Alert resolved');
    } catch {
      toast.error('Failed to resolve alert');
    }
  }, []);

  const formatDate = useCallback((dateString: string): string => {
    return new Date(dateString).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }, []);

  const activeAlerts = alerts.filter((a) => a.status === 'active').length;

  return {
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
  };
}
