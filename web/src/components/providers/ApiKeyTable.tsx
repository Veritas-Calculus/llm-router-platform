import { useState } from 'react';
import {
  PlusIcon,
  TrashIcon,
  CheckCircleIcon,
  XCircleIcon,
} from '@heroicons/react/24/outline';
import { ProviderApiKey } from '@/lib/types';
import ConfirmModal from '@/components/ConfirmModal';

interface ApiKeyTableProps {
  providerName: string;
  apiKeys: ProviderApiKey[];
  onAddKey: (data: { api_key: string; alias: string; priority: number; weight: number; rate_limit: number }) => Promise<void>;
  onUpdateKey: (keyId: string, data: { priority: number; weight: number; rate_limit: number }) => Promise<void>;
  onToggleKey: (key: ProviderApiKey) => void;
  onDeleteKey: (keyId: string) => Promise<void>;
}

export default function ApiKeyTable({
  providerName,
  apiKeys,
  onAddKey,
  onUpdateKey,
  onToggleKey,
  onDeleteKey,
}: ApiKeyTableProps) {
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState({ api_key: '', alias: '', priority: 1, weight: 1.0, rate_limit: 0 });
  const [adding, setAdding] = useState(false);
  const [editingKeyId, setEditingKeyId] = useState<string | null>(null);
  const [editKeyData, setEditKeyData] = useState({ priority: 1, weight: 1.0, rate_limit: 0 });
  const [updatingKey, setUpdatingKey] = useState(false);
  const [confirmModal, setConfirmModal] = useState<{ isOpen: boolean; keyId: string }>({ isOpen: false, keyId: '' });
  const [processing, setProcessing] = useState(false);

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const handleAddKey = async () => {
    if (!newKey.alias.trim()) return;
    setAdding(true);
    try {
      await onAddKey({
        ...newKey,
        api_key: newKey.api_key.trim() || 'default',
        alias: newKey.alias.trim(),
      });
      setShowAddModal(false);
      setNewKey({ api_key: '', alias: '', priority: 1, weight: 1.0, rate_limit: 0 });
    } finally {
      setAdding(false);
    }
  };

  const startEditingKey = (key: ProviderApiKey) => {
    setEditingKeyId(key.id);
    setEditKeyData({
      priority: key.priority || 1,
      weight: key.weight || 1.0,
      rate_limit: key.rate_limit || 0,
    });
  };

  const handleUpdateKey = async () => {
    if (!editingKeyId) return;
    setUpdatingKey(true);
    try {
      await onUpdateKey(editingKeyId, editKeyData);
      setEditingKeyId(null);
    } finally {
      setUpdatingKey(false);
    }
  };

  const handleConfirmDelete = async () => {
    setProcessing(true);
    try {
      await onDeleteKey(confirmModal.keyId);
      setConfirmModal({ isOpen: false, keyId: '' });
    } finally {
      setProcessing(false);
    }
  };

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h3 className="text-lg font-semibold text-apple-gray-900">API Keys</h3>
          <p className="text-sm text-apple-gray-500 mt-1">
            Manage API keys for {providerName}
          </p>
        </div>
        <button onClick={() => setShowAddModal(true)} className="btn btn-primary">
          <PlusIcon className="w-5 h-5 mr-2" />
          Add Key
        </button>
      </div>

      {apiKeys.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-apple-gray-500 mb-4">No API keys for this provider</p>
          <button onClick={() => setShowAddModal(true)} className="btn btn-primary">
            Add your first key
          </button>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-apple-gray-200">
            <thead>
              <tr>
                <th className="table-header">Alias</th>
                <th className="table-header w-48">Config</th>
                <th className="table-header">Status</th>
                <th className="table-header">Usage</th>
                <th className="table-header">Last Used</th>
                <th className="table-header text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {apiKeys.map((key) => (
                <tr key={key.id} className="hover:bg-apple-gray-50">
                  <td className="table-cell">
                    <span className="font-medium text-apple-gray-900 block">{key.alias}</span>
                    <code className="text-xs bg-apple-gray-100 px-1 py-0.5 rounded mt-1 inline-block">
                      {key.key_prefix}
                    </code>
                  </td>
                  <td className="table-cell">
                    {editingKeyId === key.id ? (
                      <div className="space-y-2">
                        <div className="flex items-center gap-2">
                          <label className="text-xs w-16 text-apple-gray-500">Priority</label>
                          <input
                            type="number" min="1" max="100"
                            value={editKeyData.priority}
                            onChange={e => setEditKeyData(p => ({ ...p, priority: parseInt(e.target.value) || 1 }))}
                            className="input text-xs py-1 px-2 w-20"
                          />
                        </div>
                        <div className="flex items-center gap-2">
                          <label className="text-xs w-16 text-apple-gray-500">Weight</label>
                          <input
                            type="number" step="0.1" min="0" max="100"
                            value={editKeyData.weight}
                            onChange={e => setEditKeyData(p => ({ ...p, weight: parseFloat(e.target.value) || 0 }))}
                            className="input text-xs py-1 px-2 w-20"
                          />
                        </div>
                        <div className="flex items-center gap-2">
                          <label className="text-xs w-16 text-apple-gray-500">Rate Limit</label>
                          <input
                            type="number" min="0"
                            value={editKeyData.rate_limit}
                            onChange={e => setEditKeyData(p => ({ ...p, rate_limit: parseInt(e.target.value) || 0 }))}
                            className="input text-xs py-1 px-2 w-20"
                            placeholder="0 = unltd"
                          />
                        </div>
                      </div>
                    ) : (
                      <div className="space-y-1 text-xs text-apple-gray-600">
                        <div><span className="text-apple-gray-400 w-16 inline-block">Priority:</span> <span className="font-medium">{key.priority || 1}</span></div>
                        <div><span className="text-apple-gray-400 w-16 inline-block">Weight:</span> <span className="font-medium">{key.weight || 1.0}</span></div>
                        <div><span className="text-apple-gray-400 w-16 inline-block">Rate Limit:</span> <span className="font-medium">{key.rate_limit ? `${key.rate_limit} RPS` : 'Unlimited'}</span></div>
                      </div>
                    )}
                  </td>
                  <td className="table-cell">
                    <button
                      onClick={() => onToggleKey(key)}
                      className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${key.is_active
                        ? 'bg-green-100 text-apple-green hover:bg-green-200'
                        : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
                        }`}
                    >
                      {key.is_active ? (
                        <>
                          <CheckCircleIcon className="w-3.5 h-3.5" />
                          Active
                        </>
                      ) : (
                        <>
                          <XCircleIcon className="w-3.5 h-3.5" />
                          Inactive
                        </>
                      )}
                    </button>
                  </td>
                  <td className="table-cell text-sm text-apple-gray-500">
                    {key.usage_count.toLocaleString()} reqs
                  </td>
                  <td className="table-cell text-sm text-apple-gray-500">
                    {key.last_used_at ? formatDate(key.last_used_at) : 'Never'}
                  </td>
                  <td className="table-cell text-right">
                    {editingKeyId === key.id ? (
                      <div className="flex items-center justify-end gap-2">
                        <button onClick={() => setEditingKeyId(null)} className="text-xs text-apple-gray-500 hover:text-apple-gray-700">Cancel</button>
                        <button onClick={handleUpdateKey} disabled={updatingKey} className="text-xs bg-apple-blue text-white px-2 py-1 rounded hover:bg-blue-600">
                          {updatingKey ? 'Saving' : 'Save'}
                        </button>
                      </div>
                    ) : (
                      <div className="flex items-center justify-end gap-3">
                        <button
                          onClick={() => startEditingKey(key)}
                          className="text-apple-blue hover:text-blue-600 transition-colors text-sm"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => setConfirmModal({ isOpen: true, keyId: key.id })}
                          className="text-apple-red hover:text-red-600 transition-colors text-sm"
                          title="Delete API key"
                        >
                          <TrashIcon className="w-4 h-4" />
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Key Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4">
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">Add API Key</h2>
            <div className="space-y-4">
              <div>
                <label className="label">API Key</label>
                <input
                  type="password"
                  value={newKey.api_key}
                  onChange={(e) => setNewKey((prev) => ({ ...prev, api_key: e.target.value }))}
                  className="input"
                  placeholder="sk-..."
                />
              </div>
              <div>
                <label className="label">Alias</label>
                <input
                  type="text"
                  value={newKey.alias}
                  onChange={(e) => setNewKey((prev) => ({ ...prev, alias: e.target.value }))}
                  className="input"
                  placeholder="e.g., Production Key 1"
                />
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="label">Priority</label>
                  <input
                    type="number" min="1" max="100"
                    value={newKey.priority}
                    onChange={(e) => setNewKey((prev) => ({ ...prev, priority: parseInt(e.target.value) || 1 }))}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Weight</label>
                  <input
                    type="number" step="0.1" min="0"
                    value={newKey.weight}
                    onChange={(e) => setNewKey((prev) => ({ ...prev, weight: parseFloat(e.target.value) || 1.0 }))}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Rate Limit</label>
                  <input
                    type="number" min="0"
                    value={newKey.rate_limit}
                    onChange={(e) => setNewKey((prev) => ({ ...prev, rate_limit: parseInt(e.target.value) || 0 }))}
                    className="input"
                    placeholder="0"
                  />
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowAddModal(false)} className="btn btn-secondary">Cancel</button>
              <button onClick={handleAddKey} className="btn btn-primary" disabled={adding}>
                {adding ? 'Adding...' : 'Add Key'}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title="Delete API Key"
        message="This action cannot be undone. The API key will be permanently deleted."
        confirmText="Delete"
        confirmColor="red"
        onConfirm={handleConfirmDelete}
        onCancel={() => setConfirmModal({ isOpen: false, keyId: '' })}
        loading={processing}
      />
    </div>
  );
}
