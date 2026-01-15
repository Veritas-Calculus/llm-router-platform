import { useEffect, useState, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  PlusIcon,
  TrashIcon,
  PencilIcon,
  PlayIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  XCircleIcon,
  DocumentArrowUpIcon,
  ArrowUpTrayIcon,
  LockClosedIcon,
  LinkIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { proxiesApi, Proxy } from '@/lib/api';

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
    url: '',
    type: 'http',
    region: '',
    username: '',
    password: '',
    upstream_proxy_id: '',
  });
  const [saving, setSaving] = useState(false);
  const [batchInput, setBatchInput] = useState('');
  const [batchImporting, setBatchImporting] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testingAll, setTestingAll] = useState(false);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    loadProxies();
  }, []);

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
    if (!formData.url.trim()) {
      toast.error('Please fill in the URL');
      return;
    }

    setSaving(true);
    try {
      if (editingProxy) {
        const updated = await proxiesApi.update(editingProxy.id, formData);
        setProxies((prev) =>
          prev.map((p) => (p.id === editingProxy.id ? updated : p))
        );
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
    const lines = batchInput
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith('#'));

    if (lines.length === 0) {
      toast.error('Please enter at least one proxy URL');
      return;
    }

    const proxiesToCreate = lines.map((line) => {
      // Parse line format: url [type] [region]
      const parts = line.split(/\s+/);
      const url = parts[0];
      let type = 'http';
      let region = '';

      // Auto-detect type from URL
      if (url.startsWith('socks5://')) {
        type = 'socks5';
      } else if (url.startsWith('https://')) {
        type = 'https';
      }

      // Override with explicit type if provided
      if (parts[1]) {
        const t = parts[1].toLowerCase();
        if (['http', 'https', 'socks5'].includes(t)) {
          type = t;
        } else {
          region = parts[1];
        }
      }

      if (parts[2]) {
        region = parts[2];
      }

      return { url, type, region };
    });

    setBatchImporting(true);
    try {
      const result = await proxiesApi.batchCreate(proxiesToCreate);
      
      if (result.success > 0) {
        setProxies((prev) => [...prev, ...result.proxies]);
        toast.success(`Successfully added ${result.success} proxies`);
      }
      
      if (result.failed > 0) {
        toast.error(`Failed to add ${result.failed} proxies`);
      }

      if (result.success > 0) {
        closeBatchModal();
      }
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
      setTestResults((prev) => ({
        ...prev,
        [id]: {
          id: result.id,
          is_healthy: result.is_healthy,
          latency_ms: result.latency_ms,
          error: result.error,
        },
      }));
      
      if (result.is_healthy) {
        toast.success(`Proxy healthy - ${result.latency_ms}ms`);
      } else {
        toast.error(`Proxy unhealthy: ${result.error || 'Connection failed'}`);
      }

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
      let healthy = 0;
      let unhealthy = 0;

      for (const result of response.results) {
        newResults[result.id] = result;
        if (result.is_healthy) {
          healthy++;
        } else {
          unhealthy++;
        }
      }

      setTestResults(newResults);
      
      if (unhealthy === 0) {
        toast.success(`All ${healthy} proxies are healthy`);
      } else {
        toast.error(`${unhealthy} of ${healthy + unhealthy} proxies are unhealthy`);
      }

      await loadProxies();
    } catch (error) {
      toast.error('Failed to test proxies');
    } finally {
      setTestingAll(false);
    }
  };

  const handleDeleteClick = (id: string) => {
    setDeleteConfirmId(id);
  };

  const handleCancelDelete = () => {
    setDeleteConfirmId(null);
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

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      const content = event.target?.result as string;
      setBatchInput(content);
      setShowBatchModal(true);
    };
    reader.readAsText(file);

    // Reset input so same file can be selected again
    e.target.value = '';
  };

  const triggerFileUpload = () => {
    fileInputRef.current?.click();
  };

  const handleToggle = async (id: string) => {
    try {
      const updated = await proxiesApi.toggle(id);
      setProxies((prev) =>
        prev.map((p) => (p.id === id ? updated : p))
      );
      toast.success(`Proxy ${updated.is_active ? 'enabled' : 'disabled'}`);
    } catch (error) {
      toast.error('Failed to toggle proxy');
    }
  };

  const openEditModal = (proxy: Proxy) => {
    setEditingProxy(proxy);
    setFormData({
      url: proxy.url,
      type: proxy.type,
      region: proxy.region || '',
      username: proxy.username || '',
      password: '',
      upstream_proxy_id: proxy.upstream_proxy_id || '',
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

  const openBatchModal = () => {
    setBatchInput('');
    setShowBatchModal(true);
  };

  const closeBatchModal = () => {
    setShowBatchModal(false);
    setBatchInput('');
  };

  const formatDate = (dateString: string): string => {
    if (!dateString) return 'Never';
    const date = new Date(dateString);
    if (date.getTime() === 0) return 'Never';
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getSuccessRate = (proxy: Proxy): string => {
    const total = proxy.success_count + proxy.failure_count;
    if (total === 0) return '-';
    return ((proxy.success_count / total) * 100).toFixed(1) + '%';
  };

  const getProxyHealth = (proxy: Proxy) => {
    const testResult = testResults[proxy.id];
    if (testResult) {
      return testResult.is_healthy;
    }
    const total = proxy.success_count + proxy.failure_count;
    if (total === 0) return null;
    return (proxy.success_count / total) > 0.5;
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
      {/* Hidden file input for file upload */}
      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileUpload}
        accept=".txt,.csv,.conf"
        className="hidden"
      />

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Proxies</h1>
          <p className="text-apple-gray-500 mt-1">Manage proxy nodes for API requests</p>
        </div>
        <div className="flex items-center gap-3">
          {proxies.length > 0 && (
            <button
              onClick={handleTestAllProxies}
              className="btn btn-secondary"
              disabled={testingAll}
            >
              {testingAll ? (
                <ArrowPathIcon className="w-5 h-5 mr-2 animate-spin" />
              ) : (
                <PlayIcon className="w-5 h-5 mr-2" />
              )}
              Test All
            </button>
          )}
          <button onClick={triggerFileUpload} className="btn btn-secondary" title="Upload proxy list file">
            <ArrowUpTrayIcon className="w-5 h-5 mr-2" />
            Upload File
          </button>
          <button onClick={openBatchModal} className="btn btn-secondary">
            <DocumentArrowUpIcon className="w-5 h-5 mr-2" />
            Batch Import
          </button>
          <button onClick={openCreateModal} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" />
            Add Proxy
          </button>
        </div>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="card"
      >
        {proxies.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-apple-gray-500 mb-4">No proxies configured</p>
            <div className="flex items-center justify-center gap-3">
              <button onClick={openBatchModal} className="btn btn-secondary">
                <DocumentArrowUpIcon className="w-5 h-5 mr-2" />
                Batch Import
              </button>
              <button onClick={openCreateModal} className="btn btn-primary">
                Add your first proxy
              </button>
            </div>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-apple-gray-200">
              <thead>
                <tr>
                  <th className="table-header">URL</th>
                  <th className="table-header">Type</th>
                  <th className="table-header">Region</th>
                  <th className="table-header">Status</th>
                  <th className="table-header">Health</th>
                  <th className="table-header">Success Rate</th>
                  <th className="table-header">Latency</th>
                  <th className="table-header">Last Checked</th>
                  <th className="table-header">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-apple-gray-100">
                {proxies.map((proxy) => {
                  const healthStatus = getProxyHealth(proxy);
                  const testResult = testResults[proxy.id];
                  const upstreamProxy = proxy.upstream_proxy_id
                    ? proxies.find((p) => p.id === proxy.upstream_proxy_id)
                    : null;
                  
                  return (
                    <tr key={proxy.id} className="hover:bg-apple-gray-50">
                      <td className="table-cell font-medium font-mono text-sm">
                        <div className="flex items-center gap-2">
                          {proxy.url}
                          {proxy.has_auth && (
                            <LockClosedIcon className="w-4 h-4 text-apple-blue" title="Authenticated" />
                          )}
                          {upstreamProxy && (
                            <span
                              className="inline-flex items-center gap-1 text-xs text-apple-gray-500"
                              title={`Via: ${upstreamProxy.url}`}
                            >
                              <LinkIcon className="w-3 h-3" />
                              <span className="max-w-[100px] truncate">{upstreamProxy.url.split('://').pop()?.split(':')[0] || upstreamProxy.url}</span>
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="table-cell uppercase text-sm">{proxy.type}</td>
                      <td className="table-cell">{proxy.region || '-'}</td>
                      <td className="table-cell">
                        <button
                          onClick={() => handleToggle(proxy.id)}
                          className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${
                            proxy.is_active
                              ? 'bg-green-100 text-apple-green hover:bg-green-200'
                              : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
                          }`}
                        >
                          {proxy.is_active ? 'Active' : 'Inactive'}
                        </button>
                      </td>
                      <td className="table-cell">
                        {healthStatus === null ? (
                          <span className="text-apple-gray-400">-</span>
                        ) : healthStatus ? (
                          <CheckCircleIcon className="w-5 h-5 text-apple-green" />
                        ) : (
                          <XCircleIcon className="w-5 h-5 text-apple-red" />
                        )}
                      </td>
                      <td className="table-cell">{getSuccessRate(proxy)}</td>
                      <td className="table-cell">
                        {testResult
                          ? `${testResult.latency_ms}ms`
                          : proxy.avg_latency > 0
                          ? `${proxy.avg_latency.toFixed(0)}ms`
                          : '-'}
                      </td>
                      <td className="table-cell text-apple-gray-500 text-sm">
                        {formatDate(proxy.last_checked)}
                      </td>
                      <td className="table-cell">
                        <AnimatePresence mode="wait">
                          {deleteConfirmId === proxy.id ? (
                            <motion.div
                              key="confirm"
                              initial={{ opacity: 0, x: 10 }}
                              animate={{ opacity: 1, x: 0 }}
                              exit={{ opacity: 0, x: -10 }}
                              className="flex items-center gap-2"
                            >
                              <span className="text-sm text-apple-gray-600">Delete?</span>
                              <button
                                onClick={() => handleConfirmDelete(proxy.id)}
                                className="px-2 py-1 text-xs bg-apple-red text-white rounded hover:bg-red-600 transition-colors"
                                disabled={deleting}
                              >
                                {deleting ? 'Deleting...' : 'Yes'}
                              </button>
                              <button
                                onClick={handleCancelDelete}
                                className="px-2 py-1 text-xs bg-apple-gray-200 text-apple-gray-700 rounded hover:bg-apple-gray-300 transition-colors"
                                disabled={deleting}
                              >
                                No
                              </button>
                            </motion.div>
                          ) : (
                            <motion.div
                              key="actions"
                              initial={{ opacity: 0, x: -10 }}
                              animate={{ opacity: 1, x: 0 }}
                              exit={{ opacity: 0, x: 10 }}
                              className="flex items-center gap-2"
                            >
                              <button
                                onClick={() => handleTestProxy(proxy.id)}
                                className="text-apple-blue hover:text-blue-600 transition-colors"
                                title="Test proxy"
                                disabled={testingId === proxy.id}
                              >
                                {testingId === proxy.id ? (
                                  <ArrowPathIcon className="w-5 h-5 animate-spin" />
                                ) : (
                                  <PlayIcon className="w-5 h-5" />
                                )}
                              </button>
                              <button
                                onClick={() => openEditModal(proxy)}
                                className="text-apple-gray-500 hover:text-apple-gray-700 transition-colors"
                                title="Edit proxy"
                              >
                                <PencilIcon className="w-5 h-5" />
                              </button>
                              <button
                                onClick={() => handleDeleteClick(proxy.id)}
                                className="text-apple-red hover:text-red-600 transition-colors"
                                title="Delete proxy"
                              >
                                <TrashIcon className="w-5 h-5" />
                              </button>
                            </motion.div>
                          )}
                        </AnimatePresence>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </motion.div>

      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-white rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">
              {editingProxy ? 'Edit Proxy' : 'Add Proxy'}
            </h2>
            <div className="space-y-4">
              <div>
                <label htmlFor="url" className="label">
                  URL
                </label>
                <input
                  type="text"
                  id="url"
                  value={formData.url}
                  onChange={(e) =>
                    setFormData((prev) => ({ ...prev, url: e.target.value }))
                  }
                  className="input"
                  placeholder="e.g., http://proxy.example.com:8080"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="type" className="label">
                    Type
                  </label>
                  <select
                    id="type"
                    value={formData.type}
                    onChange={(e) =>
                      setFormData((prev) => ({ ...prev, type: e.target.value }))
                    }
                    className="input"
                  >
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                    <option value="socks5">SOCKS5</option>
                  </select>
                </div>
                <div>
                  <label htmlFor="region" className="label">
                    Region
                  </label>
                  <input
                    type="text"
                    id="region"
                    value={formData.region}
                    onChange={(e) =>
                      setFormData((prev) => ({ ...prev, region: e.target.value }))
                    }
                    className="input"
                    placeholder="e.g., US-West"
                  />
                </div>
              </div>
              <div className="border-t border-apple-gray-200 pt-4 mt-2">
                <p className="text-sm font-medium text-apple-gray-700 mb-3">
                  Authentication (Optional)
                </p>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="username" className="label">
                      Username
                    </label>
                    <input
                      type="text"
                      id="username"
                      value={formData.username}
                      onChange={(e) =>
                        setFormData((prev) => ({ ...prev, username: e.target.value }))
                      }
                      className="input"
                      placeholder="Proxy username"
                    />
                  </div>
                  <div>
                    <label htmlFor="password" className="label">
                      Password
                    </label>
                    <input
                      type="password"
                      id="password"
                      value={formData.password}
                      onChange={(e) =>
                        setFormData((prev) => ({ ...prev, password: e.target.value }))
                      }
                      className="input"
                      placeholder={editingProxy?.has_auth ? '••••••••' : 'Proxy password'}
                    />
                  </div>
                </div>
                {editingProxy?.has_auth && !formData.password && (
                  <p className="text-xs text-apple-gray-500 mt-2">
                    Leave password empty to keep existing credentials
                  </p>
                )}
              </div>
              <div className="border-t border-apple-gray-200 pt-4 mt-2">
                <p className="text-sm font-medium text-apple-gray-700 mb-3">
                  Proxy Chain (Optional)
                </p>
                <div>
                  <label htmlFor="upstream_proxy" className="label">
                    Upstream Proxy
                  </label>
                  <select
                    id="upstream_proxy"
                    value={formData.upstream_proxy_id}
                    onChange={(e) =>
                      setFormData((prev) => ({ ...prev, upstream_proxy_id: e.target.value }))
                    }
                    className="input"
                  >
                    <option value="">Direct connection (no upstream)</option>
                    {proxies
                      .filter((p) => p.id !== editingProxy?.id)
                      .map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.url} ({p.type}) {p.region && `- ${p.region}`}
                        </option>
                      ))}
                  </select>
                  <p className="text-xs text-apple-gray-500 mt-1">
                    Route this proxy's traffic through another proxy first
                  </p>
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={closeModal} className="btn btn-secondary">
                Cancel
              </button>
              <button onClick={handleSubmit} className="btn btn-primary" disabled={saving}>
                {saving ? 'Saving...' : editingProxy ? 'Update' : 'Create'}
              </button>
            </div>
          </motion.div>
        </div>
      )}

      {/* Batch Import Modal */}
      {showBatchModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-white rounded-apple-lg shadow-apple-xl p-6 w-full max-w-2xl mx-4"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-2">
              Batch Import Proxies
            </h2>
            <p className="text-sm text-apple-gray-500 mb-4">
              Enter one proxy per line. Format: <code className="bg-apple-gray-100 px-1 rounded">URL [type] [region]</code>
            </p>
            <div className="space-y-4">
              <div>
                <textarea
                  value={batchInput}
                  onChange={(e) => setBatchInput(e.target.value)}
                  className="input font-mono text-sm"
                  rows={12}
                  placeholder={`# Examples (lines starting with # are ignored):
http://proxy1.example.com:8080
http://proxy2.example.com:8080 http US-West
socks5://proxy3.example.com:1080 socks5 EU
https://user:pass@proxy4.example.com:8080 https Asia

# You can also just paste a list of URLs:
http://1.2.3.4:8080
http://5.6.7.8:3128
socks5://9.10.11.12:1080`}
                />
              </div>
              <div className="bg-apple-gray-50 p-3 rounded-apple">
                <p className="text-xs text-apple-gray-600">
                  <strong>Supported formats:</strong>
                </p>
                <ul className="text-xs text-apple-gray-500 mt-1 space-y-1">
                  <li>• <code>http://host:port</code> - HTTP proxy</li>
                  <li>• <code>https://host:port</code> - HTTPS proxy</li>
                  <li>• <code>socks5://host:port</code> - SOCKS5 proxy</li>
                  <li>• <code>http://user:pass@host:port</code> - With authentication</li>
                </ul>
              </div>
            </div>
            <div className="flex justify-between items-center mt-6">
              <p className="text-sm text-apple-gray-500">
                {batchInput.split('\n').filter((l) => l.trim() && !l.trim().startsWith('#')).length} proxies to import
              </p>
              <div className="flex gap-3">
                <button onClick={closeBatchModal} className="btn btn-secondary">
                  Cancel
                </button>
                <button
                  onClick={handleBatchImport}
                  className="btn btn-primary"
                  disabled={batchImporting}
                >
                  {batchImporting ? (
                    <>
                      <ArrowPathIcon className="w-5 h-5 mr-2 animate-spin" />
                      Importing...
                    </>
                  ) : (
                    <>
                      <DocumentArrowUpIcon className="w-5 h-5 mr-2" />
                      Import
                    </>
                  )}
                </button>
              </div>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default ProxiesPage;
