/* eslint-disable @typescript-eslint/no-explicit-any */
import { motion, AnimatePresence } from 'framer-motion';
import {
  PlusIcon, TrashIcon, ClipboardIcon, XCircleIcon,
  KeyIcon, InformationCircleIcon,
} from '@heroicons/react/24/outline';
import QuickStartGuide from '@/components/QuickStartGuide';
import {
  SubscriptionQuotaBanner, RateLimitStatusCell, ConfirmModal, formatDate,
  useApiKeys,
} from '@/components/api-keys';

function ApiKeysPage() {
  const {
    t, AVAILABLE_SCOPES,
    orgs, selectedOrgId, setSelectedOrgId,
    projects, selectedProjectId, setSelectedProjectId,
    apiKeys, loading,
    showCreateModal, setShowCreateModal, newKeyName, setNewKeyName,
    selectedScopes, setSelectedScopes, newKeyRateLimit, setNewKeyRateLimit,
    newKeyTokenLimit, setNewKeyTokenLimit, createdKey, setCreatedKey, creating, handleCreate,
    showQuickGuide, setShowQuickGuide,
    openRevokeModal, openDeleteModal, closeConfirmModal, handleConfirmAction,
    confirmModal, processing, copyToClipboard,
    isProjectSettingsOpen, setIsProjectSettingsOpen, projectWhiteListedIps, setProjectWhiteListedIps,
    updatingProject, openProjectSettings, saveProjectSettings,
  } = useApiKeys();

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <SubscriptionQuotaBanner />

      <AnimatePresence>
        {showQuickGuide && <QuickStartGuide onDismiss={() => setShowQuickGuide(false)} />}
      </AnimatePresence>

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('api_keys.title')}</h1>
          <p className="text-apple-gray-500 mt-1">{t('api_keys.subtitle')}</p>
          <div className="mt-4 flex gap-4 items-end">
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-apple-gray-500">{t('common.organization')}</label>
              <select value={selectedOrgId} onChange={(e) => setSelectedOrgId(e.target.value)} className="input py-2 pl-3 pr-8 min-w-[220px]">
                {orgs.map(org => <option key={org.id} value={org.id}>{org.name}</option>)}
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-apple-gray-500">{t('common.project')}</label>
              <select value={selectedProjectId} onChange={(e) => setSelectedProjectId(e.target.value)} className="input py-2 pl-3 pr-8 min-w-[220px]" disabled={!projects.length}>
                {projects.length === 0 && <option value="">{t('common.no_projects')}</option>}
                {projects.map(proj => <option key={proj.id} value={proj.id}>{proj.name}</option>)}
              </select>
            </div>
            {selectedProjectId && (
              <button onClick={openProjectSettings} className="btn btn-secondary px-3" title={t('api_keys.project_settings')}>Settings</button>
            )}
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={() => setShowQuickGuide(!showQuickGuide)} className="btn btn-secondary bg-white dark:bg-[#1C1C1E]">
            <InformationCircleIcon className="w-5 h-5 mr-2 -ml-1" />Quick API Reference
          </button>
          {apiKeys.length > 0 && (
            <button onClick={() => setShowCreateModal(true)} className="btn btn-primary" disabled={!selectedProjectId}>
              <PlusIcon className="w-5 h-5 mr-2 -ml-1" />Create API Key
            </button>
          )}
        </div>
      </div>

      {/* Created Key Banner */}
      {createdKey && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card border-2 border-apple-green bg-green-50">
          <div className="flex items-start justify-between">
            <div>
              <h3 className="text-lg font-semibold text-apple-gray-900 mb-2">API Key Created Successfully</h3>
              <p className="text-sm text-apple-gray-600 mb-4">Please copy your API key now. You will not be able to see it again.</p>
              <div className="flex items-center gap-2 bg-[var(--theme-bg-input)] rounded-apple border border-apple-gray-200 p-3">
                <code className="text-sm text-apple-gray-900 flex-1 break-all">{createdKey.key}</code>
                <button onClick={() => copyToClipboard(createdKey.key)} className="btn btn-ghost p-2" title={t('api_keys.copy_clipboard')}>
                  <ClipboardIcon className="w-5 h-5" />
                </button>
              </div>
            </div>
            <button onClick={() => setCreatedKey(null)} className="text-apple-gray-400 hover:text-apple-gray-600">
              <span className="sr-only">{t('common.dismiss')}</span>&times;
            </button>
          </div>
        </motion.div>
      )}

      {/* Key Table / Empty State */}
      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card">
        {apiKeys.length === 0 ? (
          <div className="text-center py-16">
            <div className="w-16 h-16 bg-blue-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
              <KeyIcon className="w-8 h-8 text-apple-blue" />
            </div>
            <h3 className="text-lg font-semibold text-apple-gray-900 mb-1">{t('api_keys.no_keys')}</h3>
            <p className="text-apple-gray-500 text-sm mb-6 max-w-sm mx-auto">Create an API key to start routing requests through the LLM Router.</p>
            <button onClick={() => setShowCreateModal(true)} className="btn btn-primary rounded-xl">Create your first API key</button>
          </div>
        ) : (
          <>
            {/* Desktop Table */}
            <div className="overflow-x-auto hidden lg:block">
              <table className="min-w-full divide-y divide-apple-gray-200">
                <thead>
                  <tr>
                    <th className="table-header">{t('common.name')}</th>
                    <th className="table-header">{t('common.key')}</th>
                    <th className="table-header">{t('common.status')}</th>
                    <th className="table-header">{t('common.scopes')}</th>
                    <th className="table-header">{t('common.limits')}</th>
                    <th className="table-header">{t('common.expires')}</th>
                    <th className="table-header">{t('common.created')}</th>
                    <th className="table-header">{t('common.last_used')}</th>
                    <th className="table-header">{t('common.actions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-apple-gray-100">
                  {apiKeys.map((key) => (
                    <tr key={key.id} className="hover:bg-apple-gray-50">
                      <td className="table-cell font-medium">{key.name}</td>
                      <td className="table-cell"><code className="text-sm bg-apple-gray-100 px-2 py-1 rounded">{key.key_prefix}...</code></td>
                      <td className="table-cell"><span className={key.is_active ? 'badge-success' : 'badge-error'}>{key.is_active ? t('common.active') : t('common.revoked')}</span></td>
                      <td className="table-cell">
                        <div className="flex flex-wrap gap-1">
                          {key.scopes === 'all' ? (
                            <span className="badge-purple">{t('common.all')}</span>
                          ) : (
                            key.scopes?.split(',').map((s: string) => (
                              <span key={s} className="px-2 py-0.5 rounded-full bg-apple-gray-100 text-apple-gray-600 text-xs border border-apple-gray-200">{s}</span>
                            ))
                          )}
                        </div>
                      </td>
                      <td className="table-cell">
                        {key.is_active ? (
                          <RateLimitStatusCell keyId={key.id} isActive={key.is_active} />
                        ) : (
                          <div className="text-xs text-apple-gray-600 space-y-1">
                            <div><span className="text-apple-gray-400">RPM:</span> {key.rate_limit || 'Unlimited'}</div>
                            <div><span className="text-apple-gray-400">TPM:</span> {key.token_limit || 'Unlimited'}</div>
                            <div><span className="text-apple-gray-400">Daily:</span> {key.daily_limit || 'Unlimited'}</div>
                          </div>
                        )}
                      </td>
                      <td className="table-cell text-apple-gray-500">{key.expires_at && new Date(key.expires_at).getTime() > 0 ? formatDate(key.expires_at) : 'Never'}</td>
                      <td className="table-cell text-apple-gray-500">{formatDate(key.created_at)}</td>
                      <td className="table-cell text-apple-gray-500">{key.last_used_at && new Date(key.last_used_at).getTime() > 0 ? formatDate(key.last_used_at) : 'Never'}</td>
                      <td className="table-cell">
                        <div className="flex items-center gap-2">
                          {key.is_active && (
                            <button onClick={() => openRevokeModal(key.id)} className="text-apple-orange hover:text-orange-600 transition-colors" title={t('api_keys.revoke_key')}>
                              <XCircleIcon className="w-5 h-5" />
                            </button>
                          )}
                          <button onClick={() => openDeleteModal(key.id)} className="text-apple-red hover:text-red-600 transition-colors" title={t('api_keys.delete_key')}>
                            <TrashIcon className="w-5 h-5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Mobile Card List */}
            <div className="grid grid-cols-1 gap-4 lg:hidden sm:bg-apple-gray-50/50">
              {apiKeys.map((key) => (
                <div key={key.id} className="bg-white border border-apple-gray-200 rounded-apple-lg p-5 shadow-sm flex flex-col gap-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <h4 className="text-base font-semibold text-apple-gray-900">{key.name}</h4>
                      <code className="text-[11px] font-mono bg-apple-gray-100 px-1.5 py-0.5 rounded text-apple-gray-600 mt-1 inline-block border border-apple-gray-200">{key.key_prefix}...</code>
                    </div>
                    <span className={key.is_active ? 'badge-success text-[10px]' : 'badge-error text-[10px]'}>{key.is_active ? t('common.active') : t('common.revoked')}</span>
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {key.scopes === 'all' ? (
                      <span className="badge-purple text-[10px]">{t('common.all')}</span>
                    ) : (
                      key.scopes?.split(',').map((s: string) => (
                        <span key={s} className="px-2 py-0.5 rounded-full bg-apple-gray-100 text-apple-gray-600 text-[10px] font-medium border border-apple-gray-200">{s}</span>
                      ))
                    )}
                  </div>
                  <div className="bg-apple-gray-50 rounded-xl p-3 border border-apple-gray-100">
                    {key.is_active ? (
                      <RateLimitStatusCell keyId={key.id} isActive={key.is_active} />
                    ) : (
                      <div className="text-[11px] text-apple-gray-600 space-y-1.5">
                        <div className="flex justify-between items-center"><span className="text-apple-gray-500 font-medium tracking-wide">RPM</span> <span className="font-mono">{key.rate_limit || 'Unlimited'}</span></div>
                        <div className="flex justify-between items-center"><span className="text-apple-gray-500 font-medium tracking-wide">TPM</span> <span className="font-mono">{key.token_limit || 'Unlimited'}</span></div>
                        <div className="flex justify-between items-center"><span className="text-apple-gray-500 font-medium tracking-wide">Daily</span> <span className="font-mono">{key.daily_limit || 'Unlimited'}</span></div>
                      </div>
                    )}
                  </div>
                  <div className="flex items-center justify-between text-[11px] text-apple-gray-500 bg-apple-gray-50/50 p-2.5 rounded-lg border border-apple-gray-100/50">
                    <div>
                      <span className="block text-apple-gray-400 font-medium mb-0.5 uppercase tracking-wider text-[9px]">{t('common.created')}</span>
                      {formatDate(key.created_at)}
                    </div>
                    <div className="text-right">
                      <span className="block text-apple-gray-400 font-medium mb-0.5 uppercase tracking-wider text-[9px]">{t('common.last_used')}</span>
                      {key.last_used_at && new Date(key.last_used_at).getTime() > 0 ? formatDate(key.last_used_at) : 'Never'}
                    </div>
                  </div>
                  <div className="flex items-center justify-end gap-2 pt-2 border-t border-apple-gray-100">
                    {key.is_active && (
                      <button onClick={() => openRevokeModal(key.id)} className="flex items-center gap-1.5 px-3 py-2 bg-orange-50 text-apple-orange hover:bg-orange-100 hover:text-orange-600 text-xs font-semibold rounded-lg transition-colors border border-orange-200/50">
                        <XCircleIcon className="w-4 h-4" />{t('api_keys.revoke_key')}
                      </button>
                    )}
                    <button onClick={() => openDeleteModal(key.id)} className="flex items-center gap-1.5 px-3 py-2 bg-red-50 text-apple-red hover:bg-red-100 hover:text-red-600 text-xs font-semibold rounded-lg transition-colors border border-red-200/50">
                      <TrashIcon className="w-4 h-4" />{t('api_keys.delete_key')}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </motion.div>

      {/* Create Key Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }} className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4">
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">Create API Key</h2>
            <div className="mb-6">
              <label htmlFor="keyName" className="label">Name</label>
              <input type="text" id="keyName" value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} className="input" placeholder="e.g., Production, Development" autoFocus />
            </div>
            <div className="mb-6">
              <label className="label mb-2">Permissions (Scopes)</label>
              <div className="space-y-2 max-h-48 overflow-y-auto p-3 border border-apple-gray-200 rounded-apple bg-apple-gray-50/50">
                {AVAILABLE_SCOPES.map(scope => {
                  const isChecked = selectedScopes.includes(scope.id);
                  const isAllChecked = selectedScopes.includes('all');
                  const isDisabled = scope.id !== 'all' && isAllChecked;
                  return (
                    <label key={scope.id} className={`flex items-start gap-3 p-2 rounded-lg transition-colors ${isDisabled ? 'opacity-50 grayscale cursor-not-allowed' : 'hover:bg-[var(--theme-bg-input)] cursor-pointer'}`}>
                      <div className="pt-0.5">
                        <input type="checkbox" className="w-4 h-4 text-apple-blue border-apple-gray-300 rounded focus:ring-apple-blue transition-all" checked={isChecked || isDisabled} disabled={isDisabled}
                          onChange={(e) => {
                            if (scope.id === 'all') {
                              setSelectedScopes(e.target.checked ? ['all'] : []);
                            } else {
                              if (e.target.checked) setSelectedScopes([...selectedScopes.filter(s => s !== 'all'), scope.id]);
                              else setSelectedScopes(selectedScopes.filter(s => s !== scope.id));
                            }
                          }}
                        />
                      </div>
                      <div className="flex-1"><p className="text-sm font-medium text-apple-gray-900">{scope.label}</p></div>
                    </label>
                  );
                })}
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4 mb-6">
              <div>
                <label htmlFor="rateLimit" className="label">Requests/Min (RPM)</label>
                <input type="number" id="rateLimit" value={newKeyRateLimit} onChange={(e) => setNewKeyRateLimit(e.target.value)} className="input mt-1 block w-full" placeholder="Unlimited (1000)" />
              </div>
              <div>
                <label htmlFor="tokenLimit" className="label">Tokens/Min (TPM)</label>
                <input type="number" id="tokenLimit" value={newKeyTokenLimit} onChange={(e) => setNewKeyTokenLimit(e.target.value)} className="input mt-1 block w-full" placeholder="Unlimited (0)" />
              </div>
            </div>
            <div className="flex justify-end gap-3">
              <button onClick={() => { setShowCreateModal(false); setNewKeyName(''); setSelectedScopes(['all']); setNewKeyRateLimit(''); setNewKeyTokenLimit(''); }} className="btn btn-secondary">Cancel</button>
              <button onClick={handleCreate} className="btn btn-primary" disabled={creating}>{creating ? 'Creating...' : 'Create'}</button>
            </div>
          </motion.div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title={confirmModal.type === 'revoke' ? 'Revoke API Key' : 'Delete API Key'}
        message={confirmModal.type === 'revoke' ? 'Are you sure you want to revoke this API key? It will be deactivated but can still be deleted later.' : 'Are you sure you want to permanently delete this API key? This action cannot be undone.'}
        confirmText={confirmModal.type === 'revoke' ? 'Revoke' : 'Delete'}
        confirmColor={confirmModal.type === 'revoke' ? 'orange' : 'red'}
        onConfirm={handleConfirmAction}
        onCancel={closeConfirmModal}
        loading={processing}
      />

      {/* Project Settings Modal */}
      {isProjectSettingsOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }} className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4">
            <h3 className="text-xl font-semibold text-apple-gray-900 mb-6">Project Settings</h3>
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">API IP Whitelist</label>
                <textarea value={projectWhiteListedIps} onChange={(e) => setProjectWhiteListedIps(e.target.value)} placeholder="e.g. 192.168.1.1, 10.0.0.0/24 (comma separated)" rows={4} className="input w-full font-mono text-sm" />
                <p className="mt-2 text-xs text-apple-gray-500 max-w">Restrict API key usage to specific IP addresses or CIDR blocks. Leave empty to allow any IP. <strong>Note:</strong> This takes effect for all API keys in this project.</p>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-8">
              <button onClick={() => setIsProjectSettingsOpen(false)} className="btn btn-secondary" disabled={updatingProject}>Cancel</button>
              <button onClick={saveProjectSettings} className="btn btn-primary" disabled={updatingProject}>{updatingProject ? 'Saving...' : 'Save Settings'}</button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default ApiKeysPage;
