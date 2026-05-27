import { useCallback, useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  PlusIcon, TrashIcon, ClipboardIcon, XCircleIcon,
  KeyIcon, InformationCircleIcon,
} from '@heroicons/react/24/outline';
import QuickStartGuide from '@/components/QuickStartGuide';
import { useDialogA11y } from '@/hooks/useDialogA11y';
import {
  SubscriptionQuotaBanner, RateLimitStatusCell, ConfirmModal, formatDate,
  useApiKeys,
} from '@/components/api-keys';

// L-03: how long the freshly-created API key stays visible in the reveal
// banner before we wipe it from React state. 60s is enough to copy +
// paste into a password manager without overstaying. The Sentry Replay
// `data-sentry-mask` attribute on the key element is a belt-and-braces
// guard — Replay is disabled today (audit H-02 path), but the marker
// keeps the secret out of any future Replay capture.
const REVEAL_COUNTDOWN_SECONDS = 60;

function AccessBadges({ items, fallback }: { items?: string[]; fallback: string }) {
  const values = (items || []).filter(Boolean);

  if (values.length === 0) {
    return <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 text-xs border border-emerald-100">{fallback}</span>;
  }

  return (
    <div className="flex flex-wrap gap-1">
      {values.slice(0, 2).map(item => (
        <span key={item} title={item} className="max-w-32 truncate px-2 py-0.5 rounded-full bg-apple-gray-100 text-apple-gray-700 text-xs border border-apple-gray-200">
          {item}
        </span>
      ))}
      {values.length > 2 && (
        <span className="px-2 py-0.5 rounded-full bg-apple-gray-100 text-apple-gray-500 text-xs border border-apple-gray-200">+{values.length - 2}</span>
      )}
    </div>
  );
}

function ApiKeysPage() {
  const {
    t, AVAILABLE_SCOPES,
    orgs, selectedOrgId, setSelectedOrgId,
    projects, selectedProjectId, setSelectedProjectId,
    apiKeys, loading,
    showCreateModal, setShowCreateModal, newKeyName, setNewKeyName,
    selectedScopes, setSelectedScopes, newAllowedModels, setNewAllowedModels,
    newAllowedProviders, setNewAllowedProviders, newKeyRateLimit, setNewKeyRateLimit,
    newKeyTokenLimit, setNewKeyTokenLimit, createdKey, setCreatedKey, creating, handleCreate,
    fieldErrors, clearFieldError,
    showQuickGuide, setShowQuickGuide,
    openRevokeModal, openDeleteModal, closeConfirmModal, handleConfirmAction,
    confirmModal, processing, copyToClipboard,
    isProjectSettingsOpen, setIsProjectSettingsOpen, projectWhiteListedIps, setProjectWhiteListedIps,
    updatingProject, openProjectSettings, saveProjectSettings,
  } = useApiKeys();
  const closeCreateModal = useCallback(() => {
    setShowCreateModal(false);
    setNewKeyName('');
    setSelectedScopes(['chat']);
    setNewAllowedModels('');
    setNewAllowedProviders('');
    setNewKeyRateLimit('');
    setNewKeyTokenLimit('');
  }, [setNewAllowedModels, setNewAllowedProviders, setNewKeyName, setNewKeyRateLimit, setNewKeyTokenLimit, setSelectedScopes, setShowCreateModal]);

  // L-03: countdown for the "show me the key once" banner. The createdKey
  // state holds the full plaintext secret, so we wipe it (setCreatedKey(null))
  // after REVEAL_COUNTDOWN_SECONDS even if the user forgets to dismiss the
  // banner. Each new key resets the countdown.
  const [revealRemaining, setRevealRemaining] = useState<number>(0);
  useEffect(() => {
    if (!createdKey) {
      setRevealRemaining(0);
      return;
    }
    setRevealRemaining(REVEAL_COUNTDOWN_SECONDS);
    const interval = setInterval(() => {
      setRevealRemaining((s) => {
        if (s <= 1) {
          // Drop the key from state — not just hidden, fully gone.
          setCreatedKey(null);
          return 0;
        }
        return s - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [createdKey, setCreatedKey]);
  const closeProjectSettings = useCallback(() => setIsProjectSettingsOpen(false), [setIsProjectSettingsOpen]);
  const createDialogRef = useDialogA11y<HTMLDivElement>(showCreateModal, closeCreateModal);
  const projectDialogRef = useDialogA11y<HTMLDivElement>(isProjectSettingsOpen, closeProjectSettings);

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
      <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('api_keys.title')}</h1>
          <p className="text-apple-gray-500 mt-1">{t('api_keys.subtitle')}</p>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:flex lg:items-end lg:gap-4">
            <div className="flex min-w-0 flex-col gap-1">
              <label className="text-xs font-medium text-apple-gray-500">{t('common.organization')}</label>
              <select value={selectedOrgId} onChange={(e) => setSelectedOrgId(e.target.value)} className="input w-full min-w-0 py-2 pl-3 pr-8 lg:min-w-[220px]">
                {orgs.map(org => <option key={org.id} value={org.id}>{org.name}</option>)}
              </select>
            </div>
            <div className="flex min-w-0 flex-col gap-1">
              <label className="text-xs font-medium text-apple-gray-500">{t('common.project')}</label>
              <select value={selectedProjectId} onChange={(e) => setSelectedProjectId(e.target.value)} className="input w-full min-w-0 py-2 pl-3 pr-8 lg:min-w-[220px]" disabled={!projects.length}>
                {projects.length === 0 && <option value="">{t('common.no_projects')}</option>}
                {projects.map(proj => <option key={proj.id} value={proj.id}>{proj.name}</option>)}
              </select>
            </div>
            {selectedProjectId && (
              <button onClick={openProjectSettings} className="btn btn-secondary w-full justify-center px-3 sm:col-span-2 lg:w-auto" title={t('api_keys.project_settings')}>{t('common.configure')}</button>
            )}
          </div>
        </div>
        <div className="flex w-full flex-col gap-3 sm:flex-row xl:w-auto xl:items-center">
          <button onClick={() => setShowQuickGuide(!showQuickGuide)} className="btn btn-secondary justify-center bg-white dark:bg-[#1C1C1E]">
            <InformationCircleIcon className="w-5 h-5 mr-2 -ml-1" />{t('api_keys.quick_reference')}
          </button>
          {apiKeys.length > 0 && (
            <button onClick={() => setShowCreateModal(true)} className="btn btn-primary justify-center" disabled={!selectedProjectId}>
              <PlusIcon className="w-5 h-5 mr-2 -ml-1" />{t('api_keys.create')}
            </button>
          )}
        </div>
      </div>

      {/* Created Key Banner — L-03: countdown + sentry mask + auto-wipe */}
      {createdKey && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card border-2 border-apple-green bg-green-50">
          <div className="flex items-start justify-between">
            <div className="flex-1 min-w-0">
              <h3 className="text-lg font-semibold text-apple-gray-900 mb-2">{t('api_keys.created_banner_title')}</h3>
              <p className="text-sm text-apple-gray-600 mb-4">
                {t('api_keys.created_banner_body')}
                <span className="ml-2 text-xs font-mono text-apple-gray-500" aria-live="polite">
                  ({t('api_keys.reveal_countdown', { seconds: revealRemaining })})
                </span>
              </p>
              <div className="flex items-center gap-2 bg-[var(--theme-bg-input)] rounded-apple border border-apple-gray-200 p-3">
                <code
                  className="text-sm text-apple-gray-900 flex-1 break-all"
                  data-sentry-mask
                  data-sentry-replay-mask
                >
                  {createdKey.key}
                </code>
                <button onClick={() => copyToClipboard(createdKey.key)} className="btn btn-ghost p-2" title={t('api_keys.copy_clipboard')}>
                  <ClipboardIcon className="w-5 h-5" />
                </button>
              </div>
            </div>
            <button onClick={() => setCreatedKey(null)} className="text-apple-gray-400 hover:text-apple-gray-600 ml-2">
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
            <p className="text-apple-gray-500 text-sm mb-6 max-w-sm mx-auto">
              {selectedProjectId ? t('api_keys.no_keys_desc') : t('api_keys.no_project_desc')}
            </p>
            <button onClick={() => setShowCreateModal(true)} disabled={!selectedProjectId} className="btn btn-primary rounded-xl disabled:opacity-50 disabled:cursor-not-allowed">
              {t('api_keys.create_first')}
            </button>
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
                    <th className="table-header">{t('api_keys.access')}</th>
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
                        <div className="space-y-1.5">
                          <div className="flex items-center gap-1.5">
                            <span className="text-[10px] font-medium uppercase tracking-wide text-apple-gray-400 w-14">{t('api_keys.models_label')}</span>
                            <AccessBadges items={key.allowed_models} fallback={t('api_keys.all_models')} />
                          </div>
                          <div className="flex items-center gap-1.5">
                            <span className="text-[10px] font-medium uppercase tracking-wide text-apple-gray-400 w-14">{t('api_keys.providers_label')}</span>
                            <AccessBadges items={key.allowed_providers} fallback={t('api_keys.all_providers')} />
                          </div>
                        </div>
                      </td>
                      <td className="table-cell">
                        {key.is_active ? (
                          <RateLimitStatusCell keyId={key.id} isActive={key.is_active} />
                        ) : (
                          <div className="text-xs text-apple-gray-600 space-y-1">
                            <div><span className="text-apple-gray-400">RPM:</span> {key.rate_limit || t('users.unlimited')}</div>
                            <div><span className="text-apple-gray-400">TPM:</span> {key.token_limit || t('users.unlimited')}</div>
                            <div><span className="text-apple-gray-400">Daily:</span> {key.daily_limit || t('users.unlimited')}</div>
                          </div>
                        )}
                      </td>
                      <td className="table-cell text-apple-gray-500">{key.expires_at && new Date(key.expires_at).getTime() > 0 ? formatDate(key.expires_at) : t('common.never')}</td>
                      <td className="table-cell text-apple-gray-500">{formatDate(key.created_at)}</td>
                      <td className="table-cell text-apple-gray-500">{key.last_used_at && new Date(key.last_used_at).getTime() > 0 ? formatDate(key.last_used_at) : t('common.never')}</td>
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
                  <div className="bg-apple-gray-50 rounded-xl p-3 border border-apple-gray-100 space-y-2">
                    <div>
                      <span className="block text-apple-gray-400 font-medium mb-1 uppercase tracking-wider text-[9px]">{t('api_keys.models_label')}</span>
                      <AccessBadges items={key.allowed_models} fallback={t('api_keys.all_models')} />
                    </div>
                    <div>
                      <span className="block text-apple-gray-400 font-medium mb-1 uppercase tracking-wider text-[9px]">{t('api_keys.providers_label')}</span>
                      <AccessBadges items={key.allowed_providers} fallback={t('api_keys.all_providers')} />
                    </div>
                  </div>
                  <div className="bg-apple-gray-50 rounded-xl p-3 border border-apple-gray-100">
                    {key.is_active ? (
                      <RateLimitStatusCell keyId={key.id} isActive={key.is_active} />
                    ) : (
                      <div className="text-[11px] text-apple-gray-600 space-y-1.5">
                        <div className="flex justify-between items-center"><span className="text-apple-gray-500 font-medium tracking-wide">RPM</span> <span className="font-mono">{key.rate_limit || t('users.unlimited')}</span></div>
                        <div className="flex justify-between items-center"><span className="text-apple-gray-500 font-medium tracking-wide">TPM</span> <span className="font-mono">{key.token_limit || t('users.unlimited')}</span></div>
                        <div className="flex justify-between items-center"><span className="text-apple-gray-500 font-medium tracking-wide">Daily</span> <span className="font-mono">{key.daily_limit || t('users.unlimited')}</span></div>
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
                      {key.last_used_at && new Date(key.last_used_at).getTime() > 0 ? formatDate(key.last_used_at) : t('common.never')}
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
        <div data-modal-root="true" className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-api-key-title"
            ref={createDialogRef}
            tabIndex={-1}
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto"
          >
            <h2 id="create-api-key-title" className="text-xl font-semibold text-apple-gray-900 mb-4">{t('api_keys.create_modal_title')}</h2>
            <div className="mb-6">
              <label htmlFor="keyName" className="label">{t('common.name')}</label>
              <input
                type="text"
                id="keyName"
                value={newKeyName}
                onChange={(e) => { setNewKeyName(e.target.value); clearFieldError('name'); }}
                className="input"
                placeholder={t('api_keys.name_placeholder')}
                autoFocus
                aria-invalid={!!fieldErrors.name}
                aria-describedby={fieldErrors.name ? 'apikey-name-error' : undefined}
              />
              {fieldErrors.name && (
                <p id="apikey-name-error" role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.name}</p>
              )}
            </div>
            <div className="mb-6">
              <label className="label mb-2">{t('api_keys.permissions_label')}</label>
              <p className="mb-3 text-xs leading-relaxed text-apple-gray-500">
                {t('api_keys.scopes_help')}
              </p>
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
                <label htmlFor="rateLimit" className="label">{t('api_keys.rpm_label')}</label>
                <input type="number" id="rateLimit" value={newKeyRateLimit} onChange={(e) => setNewKeyRateLimit(e.target.value)} className="input mt-1 block w-full" placeholder={t('api_keys.rpm_placeholder')} />
              </div>
              <div>
                <label htmlFor="tokenLimit" className="label">{t('api_keys.tpm_label')}</label>
                <input type="number" id="tokenLimit" value={newKeyTokenLimit} onChange={(e) => setNewKeyTokenLimit(e.target.value)} className="input mt-1 block w-full" placeholder={t('api_keys.tpm_placeholder')} />
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
              <div>
                <label htmlFor="allowedModels" className="label">{t('api_keys.allowed_models_label')}</label>
                <textarea id="allowedModels" value={newAllowedModels} onChange={(e) => setNewAllowedModels(e.target.value)} rows={3} className="input mt-1 block w-full font-mono text-xs" placeholder={t('api_keys.all_models')} />
              </div>
              <div>
                <label htmlFor="allowedProviders" className="label">{t('api_keys.allowed_providers_label')}</label>
                <textarea id="allowedProviders" value={newAllowedProviders} onChange={(e) => setNewAllowedProviders(e.target.value)} rows={3} className="input mt-1 block w-full font-mono text-xs" placeholder={t('api_keys.all_providers')} />
              </div>
            </div>
            <div className="flex justify-end gap-3">
              <button onClick={closeCreateModal} className="btn btn-secondary">{t('common.cancel')}</button>
              <button onClick={handleCreate} className="btn btn-primary" disabled={creating}>{creating ? t('common.adding') : t('common.create')}</button>
            </div>
          </motion.div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title={confirmModal.type === 'revoke' ? t('api_keys.revoke_key') : t('api_keys.delete_key')}
        message={confirmModal.type === 'revoke' ? t('api_keys.revoke_confirm_desc') : t('api_keys.delete_confirm_desc')}
        confirmText={confirmModal.type === 'revoke' ? t('api_keys.revoke') : t('common.delete')}
        confirmColor={confirmModal.type === 'revoke' ? 'orange' : 'red'}
        onConfirm={handleConfirmAction}
        onCancel={closeConfirmModal}
        loading={processing}
      />

      {/* Project Settings Modal */}
      {isProjectSettingsOpen && (
        <div data-modal-root="true" className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            role="dialog"
            aria-modal="true"
            aria-labelledby="project-settings-title"
            ref={projectDialogRef}
            tabIndex={-1}
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4"
          >
            <h3 id="project-settings-title" className="text-xl font-semibold text-apple-gray-900 mb-6">{t('api_keys.project_settings')}</h3>
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">{t('api_keys.ip_whitelist_label')}</label>
                <textarea value={projectWhiteListedIps} onChange={(e) => setProjectWhiteListedIps(e.target.value)} placeholder="e.g. 192.168.1.1, 10.0.0.0/24" rows={4} className="input w-full font-mono text-sm" />
                <p className="mt-2 text-xs text-apple-gray-500 max-w">{t('api_keys.ip_whitelist_hint')}</p>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-8">
              <button onClick={closeProjectSettings} className="btn btn-secondary" disabled={updatingProject}>{t('common.cancel')}</button>
              <button onClick={saveProjectSettings} className="btn btn-primary" disabled={updatingProject}>{updatingProject ? t('common.saving') : t('common.save')}</button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default ApiKeysPage;
