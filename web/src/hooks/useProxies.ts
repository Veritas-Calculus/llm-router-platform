import { useState, useCallback, useRef, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { Proxy, ProxyPool } from '@/lib/types';
import {
  PROXIES_QUERY,
  PROXY_POOLS_QUERY,
  CREATE_PROXY,
  CREATE_PROXY_POOL,
  BATCH_CREATE_PROXIES,
  UPDATE_PROXY,
  UPDATE_PROXY_POOL,
  DELETE_PROXY_POOL,
  DELETE_PROXY,
  TOGGLE_PROXY_STATUS,
  TEST_PROXY,
  TEST_ALL_PROXIES,
} from '@/lib/graphql/operations';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface TestResult {
  id: string;
  is_healthy: boolean;
  latency_ms: number;
  error?: string;
}

interface ProxyFormData {
  url: string;
  type: string;
  region: string;
  username: string;
  password: string;
  upstream_proxy_id: string;
  pool_id: string;
}

interface ProxyPoolDraft {
  name: string;
  description: string;
  strategy: string;
}

const emptyForm: ProxyFormData = {
  url: '', type: 'http', region: '', username: '', password: '', upstream_proxy_id: '', pool_id: '',
};

// Map GraphQL camelCase → snake_case for backward compat
function mapProxy(d: any): Proxy {
  return {
    id: d.id, pool_id: d.poolId, pool_name: d.poolName,
    url: d.url, type: d.type, region: d.region,
    is_active: d.isActive, weight: d.weight,
    success_count: d.successCount, failure_count: d.failureCount,
    avg_latency: d.avgLatency, last_checked: d.lastChecked,
    created_at: d.createdAt, has_auth: d.hasAuth,
    upstream_proxy_id: d.upstreamProxyId, username: d.username || '',
  };
}

function mapProxyPool(d: any): ProxyPool {
  return {
    id: d.id, name: d.name, description: d.description || '',
    is_active: d.isActive, strategy: d.strategy || 'weighted',
    proxy_count: d.proxyCount || 0, active_proxy_count: d.activeProxyCount || 0,
    created_at: d.createdAt,
  };
}

export function useProxies() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { data, loading, refetch } = useQuery<any>(PROXIES_QUERY);
  const { data: poolsData, refetch: refetchPools } = useQuery<any>(PROXY_POOLS_QUERY);
  const proxies = useMemo(() => (data?.proxies || []).map(mapProxy), [data]);
  const proxyPools = useMemo(() => (poolsData?.proxyPools || []).map(mapProxyPool), [poolsData]);

  const [showModal, setShowModal] = useState(false);
  const [showBatchModal, setShowBatchModal] = useState(false);
  const [editingProxy, setEditingProxy] = useState<Proxy | null>(null);
  const [formData, setFormData] = useState<ProxyFormData>({ ...emptyForm });
  const [saving, setSaving] = useState(false);
  const [batchInput, setBatchInput] = useState('');
  const [batchPoolId, setBatchPoolId] = useState('');
  const [batchImporting, setBatchImporting] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testingAll, setTestingAll] = useState(false);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [poolDraft, setPoolDraft] = useState<ProxyPoolDraft>({ name: '', description: '', strategy: 'weighted' });
  const [creatingPool, setCreatingPool] = useState(false);
  const [updatingPoolId, setUpdatingPoolId] = useState<string | null>(null);
  const [deletingPoolId, setDeletingPoolId] = useState<string | null>(null);

  // ── Mutations ──
  const [createProxyMut] = useMutation(CREATE_PROXY);
  const [createPoolMut] = useMutation(CREATE_PROXY_POOL);
  const [batchCreateMut] = useMutation(BATCH_CREATE_PROXIES);
  const [updateProxyMut] = useMutation(UPDATE_PROXY);
  const [updatePoolMut] = useMutation(UPDATE_PROXY_POOL);
  const [deletePoolMut] = useMutation(DELETE_PROXY_POOL);
  const [deleteProxyMut] = useMutation(DELETE_PROXY);
  const [toggleProxyMut] = useMutation(TOGGLE_PROXY_STATUS);
  const [testProxyMut] = useMutation(TEST_PROXY);
  const [testAllMut] = useMutation(TEST_ALL_PROXIES);

  const refetchProxyState = useCallback(async () => {
    await Promise.all([refetch(), refetchPools()]);
  }, [refetch, refetchPools]);

  const closeModal = useCallback(() => {
    setShowModal(false); setEditingProxy(null); setFormData({ ...emptyForm });
  }, []);

  const closeBatchModal = useCallback(() => {
    setShowBatchModal(false); setBatchInput(''); setBatchPoolId('');
  }, []);

  const openCreateModal = useCallback(() => {
    setEditingProxy(null); setFormData({ ...emptyForm }); setShowModal(true);
  }, []);

  const openEditModal = useCallback((proxy: Proxy) => {
    setEditingProxy(proxy);
    setFormData({
      url: proxy.url, type: proxy.type, region: proxy.region || '',
      username: proxy.username || '', password: '',
      upstream_proxy_id: proxy.upstream_proxy_id || '',
      pool_id: proxy.pool_id || '',
    });
    setShowModal(true);
  }, []);

  const openBatchModal = useCallback(() => {
    setBatchInput(''); setBatchPoolId(''); setShowBatchModal(true);
  }, []);

  const handleSubmit = useCallback(async () => {
    if (!formData.url.trim()) { toast.error('Please fill in the URL'); return; }
    setSaving(true);
    try {
      const input = {
        url: formData.url, type: formData.type, region: formData.region || undefined,
        username: formData.username || undefined, password: formData.password || undefined,
        upstreamProxyId: formData.upstream_proxy_id || undefined,
        poolId: formData.pool_id || undefined,
      };
      if (editingProxy) {
        await updateProxyMut({ variables: { id: editingProxy.id, input } });
        toast.success('Proxy updated');
      } else {
        await createProxyMut({ variables: { input } });
        toast.success('Proxy created');
      }
      await refetchProxyState();
      closeModal();
    } catch { toast.error(editingProxy ? 'Failed to update proxy' : 'Failed to create proxy'); }
    finally { setSaving(false); }
  }, [formData, editingProxy, closeModal, createProxyMut, updateProxyMut, refetchProxyState]);

  const handleBatchImport = useCallback(async () => {
    const lines = batchInput.split('\n').map((l) => l.trim()).filter((l) => l && !l.startsWith('#'));
    if (lines.length === 0) { toast.error('Please enter at least one proxy URL'); return; }
    const proxiesToCreate = lines.map((line) => {
      const parts = line.split(/\s+/);
      const url = parts[0];
      let type = 'http';
      let region = '';
      if (url.startsWith('socks5://')) type = 'socks5';
      else if (url.startsWith('https://')) type = 'https';
      if (parts[1]) { const t = parts[1].toLowerCase(); if (['http', 'https', 'socks5'].includes(t)) type = t; else region = parts[1]; }
      if (parts[2]) region = parts[2];
      return { url, type, region };
    });
    setBatchImporting(true);
    try {
      const { data: result } = await batchCreateMut({
        variables: { input: { proxies: proxiesToCreate, poolId: batchPoolId || undefined } },
      });
      const r = (result as any)?.batchCreateProxies;
      if (r?.success > 0) { toast.success(`Successfully added ${r.success} proxies`); await refetchProxyState(); }
      if (r?.failed > 0) toast.error(`Failed to add ${r.failed} proxies`);
      if (r?.success > 0) closeBatchModal();
    } catch { toast.error('Failed to import proxies'); }
    finally { setBatchImporting(false); }
  }, [batchInput, batchPoolId, closeBatchModal, batchCreateMut, refetchProxyState]);

  const handleTestProxy = useCallback(async (id: string) => {
    setTestingId(id);
    try {
      const { data: result } = await testProxyMut({ variables: { id } });
      const r = (result as any)?.testProxy;
      if (r) {
        setTestResults((prev) => ({ ...prev, [id]: { id: r.proxyId || id, is_healthy: r.isHealthy, latency_ms: r.latencyMs || r.latency, error: r.error } }));
        if (r.isHealthy) toast.success(`Proxy healthy - ${r.latencyMs || r.latency}ms`);
        else toast.error(`Proxy unhealthy: ${r.error || 'Connection failed'}`);
      }
      await refetchProxyState();
    } catch { toast.error('Failed to test proxy'); }
    finally { setTestingId(null); }
  }, [testProxyMut, refetchProxyState]);

  const handleTestAllProxies = useCallback(async () => {
    setTestingAll(true);
    try {
      const { data: result } = await testAllMut();
      const results = (result as any)?.testAllProxies || [];
      const newResults: Record<string, TestResult> = {};
      let healthy = 0, unhealthy = 0;
      for (const r of results) {
        const mapped = { id: r.proxyId || r.id, is_healthy: r.isHealthy, latency_ms: r.latencyMs || r.latency, error: r.error };
        newResults[mapped.id] = mapped;
        if (r.isHealthy) healthy++; else unhealthy++;
      }
      setTestResults(newResults);
      if (unhealthy === 0) toast.success(`All ${healthy} proxies are healthy`);
      else toast.error(`${unhealthy} of ${healthy + unhealthy} proxies are unhealthy`);
      await refetchProxyState();
    } catch { toast.error('Failed to test proxies'); }
    finally { setTestingAll(false); }
  }, [testAllMut, refetchProxyState]);

  const handleConfirmDelete = useCallback(async (id: string) => {
    setDeleting(true);
    try {
      await deleteProxyMut({ variables: { id } });
      await refetchProxyState();
      toast.success('Proxy deleted');
      setDeleteConfirmId(null);
    } catch { toast.error('Failed to delete proxy'); }
    finally { setDeleting(false); }
  }, [deleteProxyMut, refetchProxyState]);

  const handleToggle = useCallback(async (id: string) => {
    try {
      const { data: result } = await toggleProxyMut({ variables: { id } });
      await refetchProxyState();
      toast.success(`Proxy ${(result as any)?.toggleProxyStatus?.isActive ? 'enabled' : 'disabled'}`);
    } catch { toast.error('Failed to toggle proxy'); }
  }, [toggleProxyMut, refetchProxyState]);

  const handleCreatePool = useCallback(async () => {
    const name = poolDraft.name.trim();
    if (!name) { toast.error('Please enter a pool name'); return; }
    setCreatingPool(true);
    try {
      await createPoolMut({
        variables: {
          input: {
            name,
            description: poolDraft.description.trim(),
            strategy: poolDraft.strategy || 'weighted',
            isActive: true,
          },
        },
      });
      setPoolDraft({ name: '', description: '', strategy: 'weighted' });
      await refetchPools();
      toast.success('Proxy pool created');
    } catch { toast.error('Failed to create proxy pool'); }
    finally { setCreatingPool(false); }
  }, [poolDraft, createPoolMut, refetchPools]);

  const handleTogglePool = useCallback(async (pool: ProxyPool) => {
    setUpdatingPoolId(pool.id);
    try {
      await updatePoolMut({
        variables: {
          id: pool.id,
          input: {
            name: pool.name,
            description: pool.description,
            strategy: pool.strategy || 'weighted',
            isActive: !pool.is_active,
          },
        },
      });
      await refetchPools();
      toast.success(`Proxy pool ${pool.is_active ? 'disabled' : 'enabled'}`);
    } catch { toast.error('Failed to update proxy pool'); }
    finally { setUpdatingPoolId(null); }
  }, [updatePoolMut, refetchPools]);

  const handleDeletePool = useCallback(async (id: string) => {
    setDeletingPoolId(id);
    try {
      await deletePoolMut({ variables: { id } });
      await refetchProxyState();
      toast.success('Proxy pool deleted');
    } catch { toast.error('Failed to delete proxy pool'); }
    finally { setDeletingPoolId(null); }
  }, [deletePoolMut, refetchProxyState]);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => { setBatchInput(event.target?.result as string); setShowBatchModal(true); };
    reader.readAsText(file);
    e.target.value = '';
  }, []);

  return {
    fileInputRef, proxies, proxyPools, loading, showModal, showBatchModal, editingProxy,
    formData, setFormData, saving, batchInput, setBatchInput, batchPoolId, setBatchPoolId, batchImporting,
    testingId, testingAll, testResults, deleteConfirmId, setDeleteConfirmId, deleting,
    poolDraft, setPoolDraft, creatingPool, updatingPoolId, deletingPoolId,
    openCreateModal, openEditModal, openBatchModal, closeModal, closeBatchModal,
    handleSubmit, handleBatchImport, handleTestProxy, handleTestAllProxies,
    handleConfirmDelete, handleToggle, handleFileUpload,
    handleCreatePool, handleTogglePool, handleDeletePool,
  };
}
