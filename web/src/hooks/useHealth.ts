import { useEffect, useState, useCallback } from 'react';
import toast from 'react-hot-toast';
import { healthApi, alertsApi, ApiKeyHealth, ProxyHealth, ProviderHealth, Alert, AlertConfig } from '@/lib/api';

export type HealthTab = 'providers' | 'api-keys' | 'proxies' | 'alerts' | 'config';

/**
 * Custom hook encapsulating all Health Monitor state and API logic.
 */
export function useHealth() {
  const [apiKeyHealth, setApiKeyHealth] = useState<ApiKeyHealth[]>([]);
  const [proxyHealth, setProxyHealth] = useState<ProxyHealth[]>([]);
  const [providerHealth, setProviderHealth] = useState<ProviderHealth[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [alertConfigs, setAlertConfigs] = useState<Map<string, AlertConfig>>(new Map());
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

  // Load alert configs for all providers when config tab is activated
  const loadAlertConfigs = useCallback(async (providers: ProviderHealth[]) => {
    const newConfigs = new Map<string, AlertConfig>();
    for (const p of providers) {
      try {
        const config = await alertsApi.getConfig('provider', p.id);
        newConfigs.set(`provider:${p.id}`, config);
      } catch {
        // No config exists yet — use defaults
        newConfigs.set(`provider:${p.id}`, {
          target_type: 'provider',
          target_id: p.id,
          is_enabled: false,
          failure_threshold: 3,
          webhook_url: '',
          email: '',
        });
      }
    }
    setAlertConfigs(newConfigs);
  }, []);

  const saveAlertConfig = useCallback(async (config: Omit<AlertConfig, 'id'>) => {
    try {
      await alertsApi.updateConfig(config);
      setAlertConfigs((prev) => {
        const next = new Map(prev);
        next.set(`${config.target_type}:${config.target_id}`, config);
        return next;
      });
      toast.success('Alert configuration saved');
    } catch {
      toast.error('Failed to save alert configuration');
    }
  }, []);

  useEffect(() => { loadHealthData(); }, [loadHealthData]);

  // Load configs when switching to config tab
  useEffect(() => {
    if (activeTab === 'config' && providerHealth.length > 0) {
      loadAlertConfigs(providerHealth);
    }
  }, [activeTab, providerHealth, loadAlertConfigs]);

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
    alertConfigs,
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
    saveAlertConfig,
    formatDate,
  };
}
