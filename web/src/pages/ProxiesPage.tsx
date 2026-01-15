import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { PlusIcon, TrashIcon, PencilIcon } from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { proxiesApi, Proxy } from '@/lib/api';

function ProxiesPage() {
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingProxy, setEditingProxy] = useState<Proxy | null>(null);
  const [formData, setFormData] = useState({
    url: '',
    type: 'http',
    region: '',
  });
  const [saving, setSaving] = useState(false);

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

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this proxy?')) return;

    try {
      await proxiesApi.delete(id);
      setProxies((prev) => prev.filter((p) => p.id !== id));
      toast.success('Proxy deleted');
    } catch (error) {
      toast.error('Failed to delete proxy');
    }
  };

  const openEditModal = (proxy: Proxy) => {
    setEditingProxy(proxy);
    setFormData({
      url: proxy.url,
      type: proxy.type,
      region: proxy.region || '',
    });
    setShowModal(true);
  };

  const openCreateModal = () => {
    setEditingProxy(null);
    setFormData({ url: '', type: 'http', region: '' });
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingProxy(null);
    setFormData({ url: '', type: 'http', region: '' });
  };

  const formatDate = (dateString: string): string => {
    if (!dateString) return 'Never';
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const getSuccessRate = (proxy: Proxy): string => {
    const total = proxy.success_count + proxy.failure_count;
    if (total === 0) return '100%';
    return ((proxy.success_count / total) * 100).toFixed(1) + '%';
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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Proxies</h1>
          <p className="text-apple-gray-500 mt-1">Manage proxy nodes for API requests</p>
        </div>
        <button onClick={openCreateModal} className="btn btn-primary">
          <PlusIcon className="w-5 h-5 mr-2" />
          Add Proxy
        </button>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="card"
      >
        {proxies.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-apple-gray-500 mb-4">No proxies configured</p>
            <button onClick={openCreateModal} className="btn btn-primary">
              Add your first proxy
            </button>
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
                  <th className="table-header">Success Rate</th>
                  <th className="table-header">Latency</th>
                  <th className="table-header">Last Checked</th>
                  <th className="table-header">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-apple-gray-100">
                {proxies.map((proxy) => (
                  <tr key={proxy.id} className="hover:bg-apple-gray-50">
                    <td className="table-cell font-medium">{proxy.url}</td>
                    <td className="table-cell uppercase text-sm">{proxy.type}</td>
                    <td className="table-cell">{proxy.region || '-'}</td>
                    <td className="table-cell">
                      <span
                        className={proxy.is_active ? 'badge-success' : 'badge-error'}
                      >
                        {proxy.is_active ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                    <td className="table-cell">{getSuccessRate(proxy)}</td>
                    <td className="table-cell">{proxy.avg_latency.toFixed(0)}ms</td>
                    <td className="table-cell text-apple-gray-500">
                      {formatDate(proxy.last_checked)}
                    </td>
                    <td className="table-cell">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => openEditModal(proxy)}
                          className="text-apple-blue hover:text-blue-600 transition-colors"
                        >
                          <PencilIcon className="w-5 h-5" />
                        </button>
                        <button
                          onClick={() => handleDelete(proxy.id)}
                          className="text-apple-red hover:text-red-600 transition-colors"
                        >
                          <TrashIcon className="w-5 h-5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
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
    </div>
  );
}

export default ProxiesPage;
