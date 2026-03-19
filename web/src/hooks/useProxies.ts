import { useState, useCallback, useRef, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { Proxy } from '@/lib/types';
import {
  PROXIES_QUERY,
  CREATE_PROXY,
  BATCH_CREATE_PROXIES,
  UPDATE_PROXY,
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
}

const emptyForm: ProxyFormData = {
  url: '', type: 'http', region: '', username: '', password: '', upstream_proxy_id: '',
};

// Map GraphQL camelCase → snake_case for backward compat
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

export function useProxies() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { data, loading, refetch } = useQuery<any>(PROXIES_QUERY);
  const proxies = useMemo(() => (data?.proxies || []).map(mapProxy), [data]);

  const [showModal, setShowModal] = useState(false);
  const [showBatchModal, setShowBatchModal] = useState(false);
  const [editingProxy, setEditingProxy] = useState<Proxy | null>(null);
  const [formData, setFormData] = useState<ProxyFormData>({ ...emptyForm });
  const [saving, setSaving] = useState(false);
  const [batchInput, setBatchInput] = useState('');
  const [batchImporting, setBatchImporting] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testingAll, setTestingAll] = useState(false);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  // ── Mutations ──
  const [createProxyMut] = useMutation(CREATE_PROXY);
  const [batchCreateMut] = useMutation(BATCH_CREATE_PROXIES);
  const [updateProxyMut] = useMutation(UPDATE_PROXY);
  const [deleteProxyMut] = useMutation(DELETE_PROXY);
  const [toggleProxyMut] = useMutation(TOGGLE_PROXY_STATUS);
  const [testProxyMut] = useMutation(TEST_PROXY);
  const [testAllMut] = useMutation(TEST_ALL_PROXIES);

  const closeModal = useCallback(() => {
    setShowModal(false); setEditingProxy(null); setFormData({ ...emptyForm });
  }, []);

  const closeBatchModal = useCallback(() => {
    setShowBatchModal(false); setBatchInput('');
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
    });
    setShowModal(true);
  }, []);

  const openBatchModal = useCallback(() => {
    setBatchInput(''); setShowBatchModal(true);
  }, []);

  const handleSubmit = useCallback(async () => {
    if (!formData.url.trim()) { toast.error('Please fill in the URL'); return; }
    setSaving(true);
    try {
      const input = {
        url: formData.url, type: formData.type, region: formData.region || undefined,
        username: formData.username || undefined, password: formData.password || undefined,
        upstreamProxyId: formData.upstream_proxy_id || undefined,
      };
      if (editingProxy) {
        await updateProxyMut({ variables: { id: editingProxy.id, input } });
        toast.success('Proxy updated');
      } else {
        await createProxyMut({ variables: { input } });
        toast.success('Proxy created');
      }
      await refetch();
      closeModal();
    } catch { toast.error(editingProxy ? 'Failed to update proxy' : 'Failed to create proxy'); }
    finally { setSaving(false); }
  }, [formData, editingProxy, closeModal, createProxyMut, updateProxyMut, refetch]);

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
      const { data: result } = await batchCreateMut({ variables: { input: { proxies: proxiesToCreate } } });
      const r = (result as any)?.batchCreateProxies;
      if (r?.success > 0) { toast.success(`Successfully added ${r.success} proxies`); await refetch(); }
      if (r?.failed > 0) toast.error(`Failed to add ${r.failed} proxies`);
      if (r?.success > 0) closeBatchModal();
    } catch { toast.error('Failed to import proxies'); }
    finally { setBatchImporting(false); }
  }, [batchInput, closeBatchModal, batchCreateMut, refetch]);

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
      await refetch();
    } catch { toast.error('Failed to test proxy'); }
    finally { setTestingId(null); }
  }, [testProxyMut, refetch]);

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
      await refetch();
    } catch { toast.error('Failed to test proxies'); }
    finally { setTestingAll(false); }
  }, [testAllMut, refetch]);

  const handleConfirmDelete = useCallback(async (id: string) => {
    setDeleting(true);
    try {
      await deleteProxyMut({ variables: { id } });
      await refetch();
      toast.success('Proxy deleted');
      setDeleteConfirmId(null);
    } catch { toast.error('Failed to delete proxy'); }
    finally { setDeleting(false); }
  }, [deleteProxyMut, refetch]);

  const handleToggle = useCallback(async (id: string) => {
    try {
      const { data: result } = await toggleProxyMut({ variables: { id } });
      await refetch();
      toast.success(`Proxy ${(result as any)?.toggleProxyStatus?.isActive ? 'enabled' : 'disabled'}`);
    } catch { toast.error('Failed to toggle proxy'); }
  }, [toggleProxyMut, refetch]);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => { setBatchInput(event.target?.result as string); setShowBatchModal(true); };
    reader.readAsText(file);
    e.target.value = '';
  }, []);

  return {
    fileInputRef, proxies, loading, showModal, showBatchModal, editingProxy,
    formData, setFormData, saving, batchInput, setBatchInput, batchImporting,
    testingId, testingAll, testResults, deleteConfirmId, setDeleteConfirmId, deleting,
    openCreateModal, openEditModal, openBatchModal, closeModal, closeBatchModal,
    handleSubmit, handleBatchImport, handleTestProxy, handleTestAllProxies,
    handleConfirmDelete, handleToggle, handleFileUpload,
  };
}
