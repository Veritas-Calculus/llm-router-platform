import { useEffect, useState, useCallback, useRef } from 'react';
import toast from 'react-hot-toast';
import { proxiesApi, Proxy } from '@/lib/api';

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

/**
 * Custom hook encapsulating all ProxiesPage state and API logic.
 */
export function useProxies() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [loading, setLoading] = useState(true);
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

  const loadProxies = useCallback(async () => {
    try {
      const response = await proxiesApi.list();
      setProxies(response?.data || []);
    } catch {
      toast.error('Failed to load proxies');
      setProxies([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadProxies(); }, [loadProxies]);

  const closeModal = useCallback(() => {
    setShowModal(false);
    setEditingProxy(null);
    setFormData({ ...emptyForm });
  }, []);

  const closeBatchModal = useCallback(() => {
    setShowBatchModal(false);
    setBatchInput('');
  }, []);

  const openCreateModal = useCallback(() => {
    setEditingProxy(null);
    setFormData({ ...emptyForm });
    setShowModal(true);
  }, []);

  const openEditModal = useCallback((proxy: Proxy) => {
    setEditingProxy(proxy);
    setFormData({
      url: proxy.url, type: proxy.type, region: proxy.region || '',
      username: proxy.username || '', password: '', upstream_proxy_id: proxy.upstream_proxy_id || '',
    });
    setShowModal(true);
  }, []);

  const openBatchModal = useCallback(() => {
    setBatchInput('');
    setShowBatchModal(true);
  }, []);

  const handleSubmit = useCallback(async () => {
    if (!formData.url.trim()) { toast.error('Please fill in the URL'); return; }
    setSaving(true);
    try {
      if (editingProxy) {
        const updated = await proxiesApi.update(editingProxy.id, formData);
        setProxies((prev) => prev.map((p) => (p.id === editingProxy.id ? updated : p)));
        toast.success('Proxy updated');
      } else {
        const created = await proxiesApi.create(formData);
        setProxies((prev) => [...prev, created]);
        toast.success('Proxy created');
      }
      closeModal();
    } catch {
      toast.error(editingProxy ? 'Failed to update proxy' : 'Failed to create proxy');
    } finally {
      setSaving(false);
    }
  }, [formData, editingProxy, closeModal]);

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
      if (parts[1]) {
        const t = parts[1].toLowerCase();
        if (['http', 'https', 'socks5'].includes(t)) type = t;
        else region = parts[1];
      }
      if (parts[2]) region = parts[2];
      return { url, type, region };
    });

    setBatchImporting(true);
    try {
      const result = await proxiesApi.batchCreate(proxiesToCreate);
      if (result.success > 0) {
        setProxies((prev) => [...prev, ...result.proxies]);
        toast.success(`Successfully added ${result.success} proxies`);
      }
      if (result.failed > 0) toast.error(`Failed to add ${result.failed} proxies`);
      if (result.success > 0) closeBatchModal();
    } catch {
      toast.error('Failed to import proxies');
    } finally {
      setBatchImporting(false);
    }
  }, [batchInput, closeBatchModal]);

  const handleTestProxy = useCallback(async (id: string) => {
    setTestingId(id);
    try {
      const result = await proxiesApi.test(id);
      setTestResults((prev) => ({ ...prev, [id]: result }));
      if (result.is_healthy) toast.success(`Proxy healthy - ${result.latency_ms}ms`);
      else toast.error(`Proxy unhealthy: ${result.error || 'Connection failed'}`);
      await loadProxies();
    } catch {
      toast.error('Failed to test proxy');
    } finally {
      setTestingId(null);
    }
  }, [loadProxies]);

  const handleTestAllProxies = useCallback(async () => {
    setTestingAll(true);
    try {
      const response = await proxiesApi.testAll();
      const newResults: Record<string, TestResult> = {};
      let healthy = 0, unhealthy = 0;
      for (const result of response.results) {
        newResults[result.id] = result;
        if (result.is_healthy) healthy++; else unhealthy++;
      }
      setTestResults(newResults);
      if (unhealthy === 0) toast.success(`All ${healthy} proxies are healthy`);
      else toast.error(`${unhealthy} of ${healthy + unhealthy} proxies are unhealthy`);
      await loadProxies();
    } catch {
      toast.error('Failed to test proxies');
    } finally {
      setTestingAll(false);
    }
  }, [loadProxies]);

  const handleConfirmDelete = useCallback(async (id: string) => {
    setDeleting(true);
    try {
      await proxiesApi.delete(id);
      setProxies((prev) => prev.filter((p) => p.id !== id));
      toast.success('Proxy deleted');
      setDeleteConfirmId(null);
    } catch {
      toast.error('Failed to delete proxy');
    } finally {
      setDeleting(false);
    }
  }, []);

  const handleToggle = useCallback(async (id: string) => {
    try {
      const updated = await proxiesApi.toggle(id);
      setProxies((prev) => prev.map((p) => (p.id === id ? updated : p)));
      toast.success(`Proxy ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch {
      toast.error('Failed to toggle proxy');
    }
  }, []);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      setBatchInput(event.target?.result as string);
      setShowBatchModal(true);
    };
    reader.readAsText(file);
    e.target.value = '';
  }, []);

  return {
    fileInputRef,
    proxies,
    loading,
    showModal,
    showBatchModal,
    editingProxy,
    formData,
    setFormData,
    saving,
    batchInput,
    setBatchInput,
    batchImporting,
    testingId,
    testingAll,
    testResults,
    deleteConfirmId,
    setDeleteConfirmId,
    deleting,
    openCreateModal,
    openEditModal,
    openBatchModal,
    closeModal,
    closeBatchModal,
    handleSubmit,
    handleBatchImport,
    handleTestProxy,
    handleTestAllProxies,
    handleConfirmDelete,
    handleToggle,
    handleFileUpload,
  };
}
