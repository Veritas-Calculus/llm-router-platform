import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  PlusIcon,
  TrashIcon,
  ClipboardIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { apiKeysApi, ApiKey } from '@/lib/api';

interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText: string;
  confirmColor: 'red' | 'orange';
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

function ConfirmModal({
  isOpen,
  title,
  message,
  confirmText,
  confirmColor,
  onConfirm,
  onCancel,
  loading,
}: ConfirmModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="bg-white rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
      >
        <div className="flex items-start gap-4">
          <div
            className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${
              confirmColor === 'red' ? 'bg-red-100' : 'bg-orange-100'
            }`}
          >
            <ExclamationTriangleIcon
              className={`w-6 h-6 ${confirmColor === 'red' ? 'text-apple-red' : 'text-apple-orange'}`}
            />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-apple-gray-900">{title}</h3>
            <p className="mt-2 text-sm text-apple-gray-600">{message}</p>
          </div>
        </div>
        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onCancel} className="btn btn-secondary" disabled={loading}>
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className={`btn ${confirmColor === 'red' ? 'btn-danger' : 'bg-apple-orange text-white hover:opacity-90'}`}
            disabled={loading}
          >
            {loading ? 'Processing...' : confirmText}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

function ApiKeysPage() {
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<ApiKey | null>(null);
  const [creating, setCreating] = useState(false);

  // Confirm modal state
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    type: 'revoke' | 'delete';
    keyId: string;
  }>({ isOpen: false, type: 'revoke', keyId: '' });
  const [processing, setProcessing] = useState(false);

  useEffect(() => {
    loadApiKeys();
  }, []);

  const loadApiKeys = async () => {
    try {
      const response = await apiKeysApi.list();
      setApiKeys(response?.data || []);
    } catch (error) {
      toast.error('Failed to load API keys');
      setApiKeys([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newKeyName.trim()) {
      toast.error('Please enter a name for the API key');
      return;
    }

    setCreating(true);
    try {
      const key = await apiKeysApi.create(newKeyName.trim());
      setCreatedKey(key);
      setApiKeys((prev) => [key, ...prev]);
      setNewKeyName('');
      toast.success('API key created successfully');
    } catch (error) {
      toast.error('Failed to create API key');
    } finally {
      setCreating(false);
    }
  };

  const openRevokeModal = (id: string) => {
    setConfirmModal({ isOpen: true, type: 'revoke', keyId: id });
  };

  const openDeleteModal = (id: string) => {
    setConfirmModal({ isOpen: true, type: 'delete', keyId: id });
  };

  const closeConfirmModal = () => {
    setConfirmModal({ isOpen: false, type: 'revoke', keyId: '' });
  };

  const handleConfirmAction = async () => {
    const { type, keyId } = confirmModal;
    setProcessing(true);

    try {
      if (type === 'revoke') {
        await apiKeysApi.revoke(keyId);
        setApiKeys((prev) =>
          prev.map((key) => (key.id === keyId ? { ...key, is_active: false } : key))
        );
        toast.success('API key revoked');
      } else {
        await apiKeysApi.delete(keyId);
        setApiKeys((prev) => prev.filter((key) => key.id !== keyId));
        toast.success('API key deleted');
      }
      closeConfirmModal();
    } catch (error) {
      toast.error(type === 'revoke' ? 'Failed to revoke API key' : 'Failed to delete API key');
    } finally {
      setProcessing(false);
    }
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success('Copied to clipboard');
    } catch (error) {
      toast.error('Failed to copy');
    }
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
          <h1 className="text-2xl font-semibold text-apple-gray-900">API Keys</h1>
          <p className="text-apple-gray-500 mt-1">Manage your API keys for accessing the LLM Router</p>
        </div>
        <button onClick={() => setShowCreateModal(true)} className="btn btn-primary">
          <PlusIcon className="w-5 h-5 mr-2" />
          Create API Key
        </button>
      </div>

      {createdKey && (
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: 1, y: 0 }}
          className="card border-2 border-apple-green bg-green-50"
        >
          <div className="flex items-start justify-between">
            <div>
              <h3 className="text-lg font-semibold text-apple-gray-900 mb-2">
                API Key Created Successfully
              </h3>
              <p className="text-sm text-apple-gray-600 mb-4">
                Please copy your API key now. You will not be able to see it again.
              </p>
              <div className="flex items-center gap-2 bg-white rounded-apple border border-apple-gray-200 p-3">
                <code className="text-sm text-apple-gray-900 flex-1 break-all">
                  {createdKey.key}
                </code>
                <button
                  onClick={() => copyToClipboard(createdKey.key)}
                  className="btn btn-ghost p-2"
                  title="Copy to clipboard"
                >
                  <ClipboardIcon className="w-5 h-5" />
                </button>
              </div>
            </div>
            <button onClick={() => setCreatedKey(null)} className="text-apple-gray-400 hover:text-apple-gray-600">
              <span className="sr-only">Dismiss</span>
              &times;
            </button>
          </div>
        </motion.div>
      )}

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="card"
      >
        {apiKeys.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-apple-gray-500 mb-4">No API keys yet</p>
            <button onClick={() => setShowCreateModal(true)} className="btn btn-primary">
              Create your first API key
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-apple-gray-200">
              <thead>
                <tr>
                  <th className="table-header">Name</th>
                  <th className="table-header">Key</th>
                  <th className="table-header">Status</th>
                  <th className="table-header">Expires</th>
                  <th className="table-header">Created</th>
                  <th className="table-header">Last Used</th>
                  <th className="table-header">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-apple-gray-100">
                {apiKeys.map((key) => (
                  <tr key={key.id} className="hover:bg-apple-gray-50">
                    <td className="table-cell font-medium">{key.name}</td>
                    <td className="table-cell">
                      <code className="text-sm bg-apple-gray-100 px-2 py-1 rounded">
                        {key.key_prefix}...
                      </code>
                    </td>
                    <td className="table-cell">
                      <span className={key.is_active ? 'badge-success' : 'badge-error'}>
                        {key.is_active ? 'Active' : 'Revoked'}
                      </span>
                    </td>
                    <td className="table-cell text-apple-gray-500">
                      {key.expires_at && new Date(key.expires_at).getTime() > 0
                        ? formatDate(key.expires_at)
                        : 'Never'}
                    </td>
                    <td className="table-cell text-apple-gray-500">
                      {formatDate(key.created_at)}
                    </td>
                    <td className="table-cell text-apple-gray-500">
                      {key.last_used_at && new Date(key.last_used_at).getTime() > 0
                        ? formatDate(key.last_used_at)
                        : 'Never'}
                    </td>
                    <td className="table-cell">
                      <div className="flex items-center gap-2">
                        {key.is_active && (
                          <button
                            onClick={() => openRevokeModal(key.id)}
                            className="text-apple-orange hover:text-orange-600 transition-colors"
                            title="Revoke API key"
                          >
                            <XCircleIcon className="w-5 h-5" />
                          </button>
                        )}
                        <button
                          onClick={() => openDeleteModal(key.id)}
                          className="text-apple-red hover:text-red-600 transition-colors"
                          title="Delete API key"
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

      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-white rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">Create API Key</h2>
            <div className="mb-6">
              <label htmlFor="keyName" className="label">
                Name
              </label>
              <input
                type="text"
                id="keyName"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                className="input"
                placeholder="e.g., Production, Development"
                autoFocus
              />
            </div>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewKeyName('');
                }}
                className="btn btn-secondary"
              >
                Cancel
              </button>
              <button onClick={handleCreate} className="btn btn-primary" disabled={creating}>
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </motion.div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title={confirmModal.type === 'revoke' ? 'Revoke API Key' : 'Delete API Key'}
        message={
          confirmModal.type === 'revoke'
            ? 'Are you sure you want to revoke this API key? It will be deactivated but can still be deleted later.'
            : 'Are you sure you want to permanently delete this API key? This action cannot be undone.'
        }
        confirmText={confirmModal.type === 'revoke' ? 'Revoke' : 'Delete'}
        confirmColor={confirmModal.type === 'revoke' ? 'orange' : 'red'}
        onConfirm={handleConfirmAction}
        onCancel={closeConfirmModal}
        loading={processing}
      />
    </div>
  );
}

export default ApiKeysPage;
