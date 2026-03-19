import { useState, useCallback, useEffect, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { AlertConfig } from '@/lib/types';
import {
  HEALTH_OVERVIEW_QUERY,
  ALERTS_QUERY,
  ALERT_CONFIG_QUERY,
  CHECK_API_KEY_HEALTH,
  CHECK_PROXY_HEALTH,
  CHECK_PROVIDER_HEALTH,
  CHECK_ALL_PROVIDER_HEALTH,
  ACKNOWLEDGE_ALERT,
  RESOLVE_ALERT,
  UPDATE_ALERT_CONFIG,
} from '@/lib/graphql/operations';

/* eslint-disable @typescript-eslint/no-explicit-any */

export type HealthTab = 'providers' | 'api-keys' | 'proxies' | 'alerts' | 'config';

// Map GraphQL camelCase → REST-compatible snake_case shapes
function mapApiKeyHealth(d: any) {
  return {
    id: d.id, provider_id: d.providerId, provider_name: d.providerName,
    key_prefix: d.keyPrefix, is_active: d.isActive, is_healthy: d.isHealthy,
    last_check: d.lastCheck, response_time: d.responseTime, success_rate: d.successRate,
  };
}
function mapProxyHealth(d: any) {
  return {
    id: d.id, url: d.url, type: d.type, region: d.region,
    is_active: d.isActive, is_healthy: d.isHealthy, response_time: d.responseTime,
    last_check: d.lastCheck, success_rate: d.successRate,
  };
}
function mapProviderHealth(d: any) {
  return {
    id: d.id, name: d.name, base_url: d.baseUrl,
    is_active: d.isActive, is_healthy: d.isHealthy, use_proxy: d.useProxy,
    response_time: d.responseTime, last_check: d.lastCheck, success_rate: d.successRate,
    error_message: d.errorMessage,
  };
}
function mapAlert(d: any) {
  return {
    id: d.id, target_type: d.targetType, target_id: d.targetId,
    alert_type: d.alertType, message: d.message, status: d.status,
    resolved_at: d.resolvedAt, acknowledged_at: d.acknowledgedAt, created_at: d.createdAt,
  };
}

export function useHealth() {
  const [activeTab, setActiveTab] = useState<HealthTab>('providers');
  const [alertConfigs, setAlertConfigs] = useState<Map<string, AlertConfig>>(new Map());

  const { data: healthData, loading: healthLoading, refetch: refetchHealth } = useQuery<any>(HEALTH_OVERVIEW_QUERY);
  const { data: alertsData, loading: alertsLoading, refetch: refetchAlerts } = useQuery<any>(ALERTS_QUERY);

  const loading = healthLoading || alertsLoading;
  const [refreshing, setRefreshing] = useState(false);

  const apiKeyHealth = useMemo(() => (healthData?.healthApiKeys || []).map(mapApiKeyHealth), [healthData]);
  const proxyHealth = useMemo(() => (healthData?.healthProxies || []).map(mapProxyHealth), [healthData]);
  const providerHealth = useMemo(() => (healthData?.healthProviders || []).map(mapProviderHealth), [healthData]);
  const alerts = useMemo(() => (alertsData?.alerts?.data || []).map(mapAlert), [alertsData]);
  const activeAlerts = alerts.filter((a: any) => a.status === 'active').length;

  // ── Alert Configs ──
  const loadAlertConfigs = useCallback(async (providers: { id: string }[]) => {
    const client = (await import('@/lib/graphql/client')).apolloClient;
    const newConfigs = new Map<string, AlertConfig>();
    for (const p of providers) {
      try {
        const { data } = await client.query<any>({
          query: ALERT_CONFIG_QUERY,
          variables: { targetType: 'provider', targetId: p.id },
        });
        if (data?.alertConfig) {
          newConfigs.set(`provider:${p.id}`, {
            target_type: data.alertConfig.targetType,
            target_id: data.alertConfig.targetId,
            is_enabled: data.alertConfig.isEnabled,
            failure_threshold: data.alertConfig.failureThreshold,
            webhook_url: data.alertConfig.webhookUrl || '',
            email: data.alertConfig.email || '',
          });
        }
      } catch {
        newConfigs.set(`provider:${p.id}`, {
          target_type: 'provider', target_id: p.id,
          is_enabled: false, failure_threshold: 3, webhook_url: '', email: '',
        });
      }
    }
    setAlertConfigs(newConfigs);
  }, []);

  useEffect(() => {
    if (activeTab === 'config' && providerHealth.length > 0) {
      loadAlertConfigs(providerHealth);
    }
  }, [activeTab, providerHealth, loadAlertConfigs]);

  // ── Mutations ──
  const [checkApiKeyMut] = useMutation(CHECK_API_KEY_HEALTH);
  const [checkProxyMut] = useMutation(CHECK_PROXY_HEALTH);
  const [checkProviderMut] = useMutation(CHECK_PROVIDER_HEALTH);
  const [checkAllProvidersMut] = useMutation(CHECK_ALL_PROVIDER_HEALTH);
  const [ackAlertMut] = useMutation(ACKNOWLEDGE_ALERT);
  const [resolveAlertMut] = useMutation(RESOLVE_ALERT);
  const [updateAlertCfgMut] = useMutation(UPDATE_ALERT_CONFIG);

  const refreshAll = useCallback(async () => {
    setRefreshing(true);
    await Promise.all([refetchHealth(), refetchAlerts()]);
    setRefreshing(false);
    toast.success('Health data refreshed');
  }, [refetchHealth, refetchAlerts]);

  const checkApiKey = useCallback(async (id: string) => {
    try { await checkApiKeyMut({ variables: { id } }); await refetchHealth(); toast.success('API key checked'); }
    catch { toast.error('Health check failed'); }
  }, [checkApiKeyMut, refetchHealth]);

  const checkProxy = useCallback(async (id: string) => {
    try { await checkProxyMut({ variables: { id } }); await refetchHealth(); toast.success('Proxy checked'); }
    catch { toast.error('Health check failed'); }
  }, [checkProxyMut, refetchHealth]);

  const checkProvider = useCallback(async (id: string) => {
    try { await checkProviderMut({ variables: { id } }); await refetchHealth(); toast.success('Provider checked'); }
    catch { toast.error('Health check failed'); }
  }, [checkProviderMut, refetchHealth]);

  const checkAllProviders = useCallback(async () => {
    try { await checkAllProvidersMut(); await refetchHealth(); toast.success('All providers checked'); }
    catch { toast.error('Health check failed'); }
  }, [checkAllProvidersMut, refetchHealth]);

  const acknowledgeAlert = useCallback(async (id: string) => {
    try { await ackAlertMut({ variables: { id } }); await refetchAlerts(); toast.success('Alert acknowledged'); }
    catch { toast.error('Failed to acknowledge alert'); }
  }, [ackAlertMut, refetchAlerts]);

  const resolveAlert = useCallback(async (id: string) => {
    try { await resolveAlertMut({ variables: { id } }); await refetchAlerts(); toast.success('Alert resolved'); }
    catch { toast.error('Failed to resolve alert'); }
  }, [resolveAlertMut, refetchAlerts]);

  const saveAlertConfig = useCallback(async (config: Omit<AlertConfig, 'id'>) => {
    try {
      await updateAlertCfgMut({
        variables: {
          input: {
            targetType: config.target_type, targetId: config.target_id,
            isEnabled: config.is_enabled, failureThreshold: config.failure_threshold,
            webhookUrl: config.webhook_url, email: config.email,
          },
        },
      });
      setAlertConfigs((prev) => {
        const next = new Map(prev);
        next.set(`${config.target_type}:${config.target_id}`, config as AlertConfig);
        return next;
      });
      toast.success('Alert configuration saved');
    } catch { toast.error('Failed to save alert configuration'); }
  }, [updateAlertCfgMut]);

  const formatDate = (dateString: string): string =>
    new Date(dateString).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });

  return {
    apiKeyHealth, proxyHealth, providerHealth, alerts, alertConfigs,
    loading, refreshing, activeTab, setActiveTab, activeAlerts,
    refreshAll, checkApiKey, checkProxy, checkProvider, checkAllProviders,
    acknowledgeAlert, resolveAlert, saveAlertConfig, formatDate,
  };
}
