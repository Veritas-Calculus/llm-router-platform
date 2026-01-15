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
    name: '',
    host: '',
    port: 8080,
    protocol: 'http',
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
    if (!formData.name.trim() || !formData.host.trim()) {
      toast.error('Please fill in all required fields');
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
      name: proxy.name,
      host: proxy.host,
      port: proxy.port,
      protocol: proxy.protocol,
    });
    setShowModal(true);
  };

  const openCreateModal = () => {
    setEditingProxy(null);
    setFormData({ name: '', host: '', port: 8080, protocol: 'http' });
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingProxy(null);
    setFormData({ name: '', host: '', port: 8080, protocol: 'http' });
  };

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
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
        <button onClick={openCreateModal} className="btn-primary">
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
            <button onClick={openCreateModal} className="btn-primary">
              Add your first proxy
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-apple-gray-200">
              <thead>
                <tr>
                  <th className="table-header">Name</th>
                  <th className="table-header">Host</th>
                  <th className="table-header">Protocol</th>
                  <th className="table-header">Status</th>
                  <th className="table-header">Success Rate</th>
                  <th className="table-header">Latency</th>
                  <th className="table-header">Requests</th>
                  <th className="table-header">Created</th>
                  <th className="table-header">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-apple-gray-100">
                {proxies.map((proxy) => (
                  <tr key={proxy.id} className="hover:bg-apple-gray-50">
                    <td className="table-cell font-medium">{proxy.name}</td>
                    <td className="table-cell">
                      {proxy.host}:{proxy.port}
                    </td>
                    <td className="table-cell uppercase text-sm">{proxy.protocol}</td>
                    <td className="table-cell">
                      <span
                        className={
                          proxy.status === 'active' ? 'badge-success' : 'badge-error'
                        }
                      >
                        {proxy.status}
                      </span>
                    </td>
                    <td className="table-cell">
                      {(proxy.success_rate * 100).toFixed(1)}%
                    </td>
                    <td className="table-cell">{proxy.avg_latency_ms}ms</td>
                    <td className="table-cell">
                      {new Intl.NumberFormat().format(proxy.total_requests)}
                    </td>
                    <td className="table-cell text-apple-gray-500">
                      {formatDate(proxy.created_at)}
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
                <label htmlFor="name" className="label">
                  Name
                </label>
                <input
                  type="text"
                  id="name"
                  value={formData.name}
                  onChange={(e) =>
                    setFormData((prev) => ({ ...prev, name: e.target.value }))
                  }
                  className="input"
                  placeholder="e.g., US Proxy 1"
                />
              </div>
              <div>
                <label htmlFor="host" className="label">
                  Host
                </label>
                <input
                  type="text"
                  id="host"
                  value={formData.host}
                  onChange={(e) =>
                    setFormData((prev) => ({ ...prev, host: e.target.value }))
                  }
                  className="input"
                  placeholder="e.g., proxy.example.com"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="port" className="label">
                    Port
                  </label>
                  <input
                    type="number"
                    id="port"
                    value={formData.port}
                    onChange={(e) =>
                      setFormData((prev) => ({ ...prev, port: parseInt(e.target.value) || 8080 }))
                    }
                    className="input"
                    min="1"
                    max="65535"
                  />
                </div>
                <div>
                  <label htmlFor="protocol" className="label">
                    Protocol
                  </label>
                  <select
                    id="protocol"
                    value={formData.protocol}
                    onChange={(e) =>
                      setFormData((prev) => ({ ...prev, protocol: e.target.value }))
                    }
                    className="input"
                  >
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                    <option value="socks5">SOCKS5</option>
                  </select>
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={closeModal} className="btn-secondary">
                Cancel
              </button>
              <button onClick={handleSubmit} className="btn-primary" disabled={saving}>
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
