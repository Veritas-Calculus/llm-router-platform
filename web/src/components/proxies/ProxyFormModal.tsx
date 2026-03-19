import { motion } from 'framer-motion';
import { Proxy } from '@/lib/types';

interface ProxyFormData {
  url: string;
  type: string;
  region: string;
  username: string;
  password: string;
  upstream_proxy_id: string;
}

interface ProxyFormModalProps {
  isOpen: boolean;
  editingProxy: Proxy | null;
  formData: ProxyFormData;
  proxies: Proxy[];
  saving: boolean;
  onFormChange: (data: ProxyFormData) => void;
  onSubmit: () => void;
  onClose: () => void;
}

export default function ProxyFormModal({
  isOpen,
  editingProxy,
  formData,
  proxies,
  saving,
  onFormChange,
  onSubmit,
  onClose,
}: ProxyFormModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
      >
        <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">
          {editingProxy ? 'Edit Proxy' : 'Add Proxy'}
        </h2>
        <div className="space-y-4">
          <div>
            <label htmlFor="url" className="label">URL</label>
            <input
              type="text"
              id="url"
              value={formData.url}
              onChange={(e) => onFormChange({ ...formData, url: e.target.value })}
              className="input"
              placeholder="e.g., http://proxy.example.com:8080"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="type" className="label">Type</label>
              <select
                id="type"
                value={formData.type}
                onChange={(e) => onFormChange({ ...formData, type: e.target.value })}
                className="input"
              >
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
                <option value="socks5">SOCKS5</option>
              </select>
            </div>
            <div>
              <label htmlFor="region" className="label">Region</label>
              <input
                type="text"
                id="region"
                value={formData.region}
                onChange={(e) => onFormChange({ ...formData, region: e.target.value })}
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
                <label htmlFor="username" className="label">Username</label>
                <input
                  type="text"
                  id="username"
                  value={formData.username}
                  onChange={(e) => onFormChange({ ...formData, username: e.target.value })}
                  className="input"
                  placeholder="Proxy username"
                />
              </div>
              <div>
                <label htmlFor="password" className="label">Password</label>
                <input
                  type="password"
                  id="password"
                  value={formData.password}
                  onChange={(e) => onFormChange({ ...formData, password: e.target.value })}
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
              <label htmlFor="upstream_proxy" className="label">Upstream Proxy</label>
              <select
                id="upstream_proxy"
                value={formData.upstream_proxy_id}
                onChange={(e) => onFormChange({ ...formData, upstream_proxy_id: e.target.value })}
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
          <button onClick={onClose} className="btn btn-secondary">Cancel</button>
          <button onClick={onSubmit} className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving...' : editingProxy ? 'Update' : 'Create'}
          </button>
        </div>
      </motion.div>
    </div>
  );
}
