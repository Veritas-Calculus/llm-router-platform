import { useEffect, useState, useRef } from 'react';
import { motion } from 'framer-motion';
import {
  PlusIcon,
  PlayIcon,
  ArrowPathIcon,
  DocumentArrowUpIcon,
  ArrowUpTrayIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { proxiesApi, Proxy } from '@/lib/api';
import ProxyTable from '@/components/proxies/ProxyTable';
import ProxyFormModal from '@/components/proxies/ProxyFormModal';
import BatchImportModal from '@/components/proxies/BatchImportModal';

interface TestResult {
  id: string;
  is_healthy: boolean;
  latency_ms: number;
  error?: string;
}

function ProxiesPage() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [showBatchModal, setShowBatchModal] = useState(false);
  const [editingProxy, setEditingProxy] = useState<Proxy | null>(null);
  const [formData, setFormData] = useState({
    url: '', type: 'http', region: '', username: '', password: '', upstream_proxy_id: '',
  });
  const [saving, setSaving] = useState(false);
  const [batchInput, setBatchInput] = useState('');
  const [batchImporting, setBatchImporting] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testingAll, setTestingAll] = useState(false);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => { loadProxies(); }, []);

  const loadProxies = async () => {
    try {
      const response = await proxiesApi.list();
      setProxies(response?.data || []);
    } catch (error) {
      toast.error('Failed to load proxies');
      setProxies([]);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
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
    } catch (error) {
      toast.error(editingProxy ? 'Failed to update proxy' : 'Failed to create proxy');
    } finally {
      setSaving(false);
    }
  };

  const handleBatchImport = async () => {
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
    } catch (error) {
      toast.error('Failed to import proxies');
    } finally {
      setBatchImporting(false);
    }
  };

  const handleTestProxy = async (id: string) => {
    setTestingId(id);
    try {
      const result = await proxiesApi.test(id);
      setTestResults((prev) => ({ ...prev, [id]: result }));
      if (result.is_healthy) toast.success(`Proxy healthy - ${result.latency_ms}ms`);
      else toast.error(`Proxy unhealthy: ${result.error || 'Connection failed'}`);
      await loadProxies();
    } catch (error) {
      toast.error('Failed to test proxy');
    } finally {
      setTestingId(null);
    }
  };

  const handleTestAllProxies = async () => {
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
    } catch (error) {
      toast.error('Failed to test proxies');
    } finally {
      setTestingAll(false);
    }
  };

  const handleConfirmDelete = async (id: string) => {
    setDeleting(true);
    try {
      await proxiesApi.delete(id);
      setProxies((prev) => prev.filter((p) => p.id !== id));
      toast.success('Proxy deleted');
      setDeleteConfirmId(null);
    } catch (error) {
      toast.error('Failed to delete proxy');
    } finally {
      setDeleting(false);
    }
  };

  const handleToggle = async (id: string) => {
    try {
      const updated = await proxiesApi.toggle(id);
      setProxies((prev) => prev.map((p) => (p.id === id ? updated : p)));
      toast.success(`Proxy ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch (error) {
      toast.error('Failed to toggle proxy');
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      setBatchInput(event.target?.result as string);
      setShowBatchModal(true);
    };
    reader.readAsText(file);
    e.target.value = '';
  };

  const openEditModal = (proxy: Proxy) => {
    setEditingProxy(proxy);
    setFormData({
      url: proxy.url, type: proxy.type, region: proxy.region || '',
      username: proxy.username || '', password: '', upstream_proxy_id: proxy.upstream_proxy_id || '',
    });
    setShowModal(true);
  };

  const openCreateModal = () => {
    setEditingProxy(null);
    setFormData({ url: '', type: 'http', region: '', username: '', password: '', upstream_proxy_id: '' });
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingProxy(null);
    setFormData({ url: '', type: 'http', region: '', username: '', password: '', upstream_proxy_id: '' });
  };

  const closeBatchModal = () => {
    setShowBatchModal(false);
    setBatchInput('');
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
      <input type="file" ref={fileInputRef} onChange={handleFileUpload} accept=".txt,.csv,.conf" className="hidden" />

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Proxies</h1>
          <p className="text-apple-gray-500 mt-1">Manage proxy nodes for API requests</p>
        </div>
        <div className="flex items-center gap-3">
          {proxies.length > 0 && (
            <button onClick={handleTestAllProxies} className="btn btn-secondary" disabled={testingAll}>
              {testingAll ? <ArrowPathIcon className="w-5 h-5 mr-2 animate-spin" /> : <PlayIcon className="w-5 h-5 mr-2" />}
              Test All
            </button>
          )}
          <button onClick={() => fileInputRef.current?.click()} className="btn btn-secondary" title="Upload proxy list file">
            <ArrowUpTrayIcon className="w-5 h-5 mr-2" /> Upload File
          </button>
          <button onClick={() => { setBatchInput(''); setShowBatchModal(true); }} className="btn btn-secondary">
            <DocumentArrowUpIcon className="w-5 h-5 mr-2" /> Batch Import
          </button>
          <button onClick={openCreateModal} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" /> Add Proxy
          </button>
        </div>
      </div>

      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
        {proxies.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-apple-gray-500 mb-4">No proxies configured</p>
            <div className="flex items-center justify-center gap-3">
              <button onClick={() => { setBatchInput(''); setShowBatchModal(true); }} className="btn btn-secondary">
                <DocumentArrowUpIcon className="w-5 h-5 mr-2" /> Batch Import
              </button>
              <button onClick={openCreateModal} className="btn btn-primary">Add your first proxy</button>
            </div>
          </div>
        ) : (
          <ProxyTable
            proxies={proxies}
            testResults={testResults}
            testingId={testingId}
            deleteConfirmId={deleteConfirmId}
            deleting={deleting}
            onTest={handleTestProxy}
            onEdit={openEditModal}
            onToggle={handleToggle}
            onDeleteClick={setDeleteConfirmId}
            onConfirmDelete={handleConfirmDelete}
            onCancelDelete={() => setDeleteConfirmId(null)}
          />
        )}
      </motion.div>

      <ProxyFormModal
        isOpen={showModal}
        editingProxy={editingProxy}
        formData={formData}
        proxies={proxies}
        saving={saving}
        onFormChange={setFormData}
        onSubmit={handleSubmit}
        onClose={closeModal}
      />

      <BatchImportModal
        isOpen={showBatchModal}
        batchInput={batchInput}
        importing={batchImporting}
        onInputChange={setBatchInput}
        onImport={handleBatchImport}
        onClose={closeBatchModal}
      />
    </div>
  );
}

export default ProxiesPage;
