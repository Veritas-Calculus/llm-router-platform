/* eslint-disable @typescript-eslint/no-explicit-any */
import { motion, AnimatePresence } from 'framer-motion';
import {
  PlusIcon, TrashIcon, PencilSquareIcon, BoltIcon,
  CheckCircleIcon, XCircleIcon, ClockIcon,
} from '@heroicons/react/24/outline';
import { useWebhooks } from '@/components/webhooks';

export default function WebhooksPage() {
  const {
    t,
    orgs, selectedOrgId, setSelectedOrgId,
    projects, selectedProjectId, setSelectedProjectId,
    isModalOpen, setIsModalOpen, editingWebhook,
    selectedEndpointId, setSelectedEndpointId,
    data, loading, deliveriesData, deliveriesLoading,
    formData, setFormData,
    handleOpenModal, handleSubmit, handleDelete, handleTest,
    parseJson,
  } = useWebhooks();

  /* ── Shared org/project selector ── */
  const OrgProjectSelectors = (
    <div className="flex items-center gap-2">
      <div className="flex flex-col gap-1">
        <label className="text-xs font-medium" style={{ color: 'var(--theme-text-muted)' }}>{t('common.organization')}</label>
        <select title={t('common.organization')} value={selectedOrgId} onChange={(e) => setSelectedOrgId(e.target.value)}
          className="block w-48 rounded-xl py-2 pl-3 pr-10 text-sm focus:outline-none focus:ring-1 focus:ring-apple-blue"
          style={{ backgroundColor: 'var(--theme-bg-card)', border: '1px solid var(--theme-border)', color: 'var(--theme-text)' }}>
          <option value="" disabled>{t('common.select_org')}</option>
          {orgs.map((o) => <option key={o.id} value={o.id}>{o.name}</option>)}
        </select>
      </div>
      <div className="flex flex-col gap-1">
        <label className="text-xs font-medium" style={{ color: 'var(--theme-text-muted)' }}>{t('common.project')}</label>
        <select title={t('common.project')} value={selectedProjectId} onChange={(e) => setSelectedProjectId(e.target.value)}
          className="block w-48 rounded-xl py-2 pl-3 pr-10 text-sm focus:outline-none focus:ring-1 focus:ring-apple-blue"
          style={{ backgroundColor: 'var(--theme-bg-card)', border: '1px solid var(--theme-border)', color: 'var(--theme-text)' }}>
          <option value="" disabled>{t('common.select_project')}</option>
          {projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
      </div>
    </div>
  );

  if (!selectedProjectId) {
    return (
      <div className="space-y-6 max-w-7xl mx-auto">
        <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div>
            <h1 className="text-2xl font-semibold" style={{ color: 'var(--theme-text)' }}>{t('webhooks.title')}</h1>
            <p className="mt-1 text-sm" style={{ color: 'var(--theme-text-secondary)' }}>{t('webhooks.subtitle')}</p>
          </div>
          {OrgProjectSelectors}
        </div>
        <div className="card p-12 text-center">
          <BoltIcon className="w-12 h-12 mx-auto mb-4" style={{ color: 'var(--theme-text-muted)' }} />
          <h3 className="text-lg font-medium mb-2" style={{ color: 'var(--theme-text-secondary)' }}>
            {projects.length === 0 ? t('webhooks.no_projects_available') : t('webhooks.select_project_first')}
          </h3>
          <p className="text-sm max-w-md mx-auto" style={{ color: 'var(--theme-text-muted)' }}>
            {projects.length === 0 ? t('webhooks.create_project_first') : t('webhooks.choose_project')}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-semibold" style={{ color: 'var(--theme-text)' }}>{t('webhooks.title')}</h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--theme-text-secondary)' }}>
            {t('webhooks.subtitle_with_project', { project: projects.find(p => p.id === selectedProjectId)?.name || '' })}
          </p>
        </div>
        <div className="flex items-center gap-4">
          {OrgProjectSelectors}
          <button onClick={() => handleOpenModal()} className="inline-flex items-center gap-x-2 rounded-xl bg-apple-blue px-4 py-2.5 text-sm font-medium text-white shadow hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-apple-blue focus:ring-offset-2 transition-colors">
            <PlusIcon className="-ml-0.5 h-5 w-5" aria-hidden="true" />{t('webhooks.add_endpoint')}
          </button>
        </div>
      </div>

      {/* Endpoints Table */}
      <div className="card overflow-hidden">
        {loading ? (
          <div className="p-8 text-center" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.loading')}</div>
        ) : data?.webhooks?.length === 0 ? (
          <div className="p-12 text-center" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.no_webhooks_desc')}</div>
        ) : (
          <table className="min-w-full divide-y divide-apple-gray-200 dark:divide-[var(--theme-border)]">
            <thead style={{ backgroundColor: 'var(--theme-bg-input)' }}>
              <tr>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.url_and_desc')}</th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.events')}</th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--theme-text-muted)' }}>{t('common.status')}</th>
                <th scope="col" className="relative px-6 py-3"><span className="sr-only">{t('common.actions')}</span></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-200 dark:divide-[var(--theme-border)]">
              {data?.webhooks?.map((webhook: any) => (
                <tr key={webhook.id}>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex flex-col">
                      <span className="text-sm font-medium truncate max-w-xs" style={{ color: 'var(--theme-text)' }}>{webhook.url}</span>
                      <span className="text-xs truncate max-w-xs mt-1" style={{ color: 'var(--theme-text-muted)' }}>{webhook.description || t('webhooks.no_description')}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {webhook.events.map((e: string) => (
                        <span key={e} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium" style={{ backgroundColor: 'var(--theme-bg-input)', color: 'var(--theme-text-secondary)' }}>{e}</span>
                      ))}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${webhook.isActive ? 'bg-green-100 text-green-800 dark:bg-green-400/10 dark:text-green-400' : 'bg-gray-100 text-gray-800 dark:bg-white/10 dark:text-gray-400'}`}>
                      {webhook.isActive ? t('webhooks.active') : t('webhooks.disabled')}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <div className="flex items-center justify-end gap-3">
                      <button title={t('webhooks.test_ping')} onClick={() => handleTest(webhook.id)} className="text-apple-blue hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 transition-colors"><BoltIcon className="w-5 h-5" /></button>
                      <button title={t('webhooks.view_deliveries')} onClick={() => setSelectedEndpointId(selectedEndpointId === webhook.id ? null : webhook.id)}
                        className={`transition-colors ${selectedEndpointId === webhook.id ? 'text-green-600 dark:text-green-400' : 'text-apple-gray-400 hover:text-apple-gray-600 dark:hover:text-gray-300'}`}><ClockIcon className="w-5 h-5" /></button>
                      <button title={t('common.edit')} onClick={() => handleOpenModal(webhook)} className="text-apple-gray-400 hover:text-apple-gray-600 dark:hover:text-gray-300 transition-colors"><PencilSquareIcon className="w-5 h-5" /></button>
                      <button title={t('common.delete')} onClick={() => handleDelete(webhook.id)} className="text-red-600 hover:text-red-700 dark:text-red-500 dark:hover:text-red-400 transition-colors"><TrashIcon className="w-5 h-5" /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Deliveries Panel */}
      <AnimatePresence>
        {selectedEndpointId && (
          <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="card overflow-hidden flex flex-col">
            <div className="px-6 py-4 border-b flex justify-between items-center" style={{ borderColor: 'var(--theme-border)', backgroundColor: 'var(--theme-bg-input)' }}>
              <h3 className="text-lg font-medium" style={{ color: 'var(--theme-text)' }}>{t('webhooks.recent_deliveries')}</h3>
              <button onClick={() => setSelectedEndpointId(null)} className="transition-colors" style={{ color: 'var(--theme-text-muted)' }}>{t('common.close')}</button>
            </div>
            <div className="max-h-[600px] overflow-y-auto w-full p-6">
              {deliveriesLoading ? (
                <div className="text-center py-4" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.loading_deliveries')}</div>
              ) : deliveriesData?.webhookDeliveries?.length === 0 ? (
                <div className="text-center py-8" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.no_deliveries_desc')}</div>
              ) : (
                <div className="space-y-4">
                  {deliveriesData?.webhookDeliveries?.map((d: any) => (
                    <div key={d.id} className="card overflow-hidden">
                      <div className="px-4 py-3 flex items-center justify-between border-b" style={{ borderColor: 'var(--theme-border)', backgroundColor: 'var(--theme-bg-input)' }}>
                        <div className="flex items-center gap-3">
                          {d.status === 'success' ? <CheckCircleIcon className="w-5 h-5 text-green-500" /> : d.status === 'pending' ? <ClockIcon className="w-5 h-5 text-amber-500" /> : <XCircleIcon className="w-5 h-5 text-red-500" />}
                          <div className="flex flex-col">
                            <span className="text-sm font-medium" style={{ color: 'var(--theme-text)' }}>{d.eventType}</span>
                            <span className="text-xs" style={{ color: 'var(--theme-text-muted)' }}>{new Date(d.createdAt).toLocaleString()}</span>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="text-xs font-mono" style={{ color: 'var(--theme-text-muted)' }}>HTTP {d.statusCode || '---'}</span>
                          {d.retryCount > 0 && <span className="text-xs text-amber-500 font-medium">{t('webhooks.retry')} {d.retryCount}</span>}
                        </div>
                      </div>
                      <div className="p-4 grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono">
                        <div>
                          <p className="font-semibold mb-2" style={{ color: 'var(--theme-text-secondary)' }}>{t('webhooks.request_payload')}</p>
                          <div className="p-3 rounded-lg overflow-x-auto" style={{ backgroundColor: 'var(--theme-bg-input)', border: '1px solid var(--theme-border)' }}>
                            <pre className="text-[11px]" style={{ color: 'var(--theme-text-secondary)' }}>{parseJson(d.payload)}</pre>
                          </div>
                        </div>
                        <div>
                          <p className="font-semibold mb-2" style={{ color: 'var(--theme-text-secondary)' }}>{t('webhooks.response_error')}</p>
                          <div className="p-3 rounded-lg overflow-x-auto min-h-[4rem]" style={{ backgroundColor: 'var(--theme-bg-input)', border: '1px solid var(--theme-border)' }}>
                            {d.errorMessage ? (
                              <pre className="text-[11px] text-red-600 dark:text-red-400 whitespace-pre-wrap">{d.errorMessage}</pre>
                            ) : (
                              <pre className="text-[11px] break-all whitespace-pre-wrap" style={{ color: 'var(--theme-text-secondary)' }}>{d.responseBody || t('webhooks.no_response_body')}</pre>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Create/Edit Modal */}
      <AnimatePresence>
        {isModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={() => setIsModalOpen(false)} className="absolute inset-0 bg-gray-500/75 dark:bg-black/80 backdrop-blur-sm transition-opacity" />
            <motion.div initial={{ opacity: 0, scale: 0.95, y: 20 }} animate={{ opacity: 1, scale: 1, y: 0 }} exit={{ opacity: 0, scale: 0.95, y: 20 }} className="relative w-full max-w-lg card overflow-hidden">
              <div className="px-6 py-5 border-b" style={{ borderColor: 'var(--theme-border)' }}>
                <h3 className="text-xl font-semibold" style={{ color: 'var(--theme-text)' }}>{editingWebhook ? t('webhooks.edit_webhook') : t('webhooks.add_webhook')}</h3>
              </div>
              <form onSubmit={handleSubmit} className="p-6 space-y-5">
                <div>
                  <label htmlFor="url" className="block text-sm font-medium" style={{ color: 'var(--theme-text-secondary)' }}>{t('webhooks.payload_url')}</label>
                  <input type="url" id="url" required value={formData.url} onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                    className="mt-2 block w-full rounded-xl px-4 py-2.5 text-sm focus:ring-apple-blue focus:border-apple-blue"
                    style={{ backgroundColor: 'var(--theme-bg-input)', border: '1px solid var(--theme-border)', color: 'var(--theme-text)' }}
                    placeholder={t('webhooks.url_placeholder')} />
                </div>
                <div>
                  <label htmlFor="description" className="block text-sm font-medium" style={{ color: 'var(--theme-text-secondary)' }}>{t('webhooks.description_optional')}</label>
                  <input type="text" id="description" value={formData.description} onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="mt-2 block w-full rounded-xl px-4 py-2.5 text-sm focus:ring-apple-blue focus:border-apple-blue"
                    style={{ backgroundColor: 'var(--theme-bg-input)', border: '1px solid var(--theme-border)', color: 'var(--theme-text)' }}
                    placeholder={t('webhooks.url_placeholder')} />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-2" style={{ color: 'var(--theme-text-secondary)' }}>{t('webhooks.events_to_send')}</label>
                  <div className="space-y-2">
                    {[{ event: 'ping', descKey: 'webhooks.ping_desc' }, { event: 'payment.succeeded', descKey: 'webhooks.payment_desc' }].map(({ event, descKey }) => (
                      <label key={event} className="flex items-center gap-3 p-3 rounded-lg cursor-pointer hover:opacity-80 transition-colors" style={{ backgroundColor: 'var(--theme-bg-input)', border: '1px solid var(--theme-border)' }}>
                        <input type="checkbox" className="rounded border-gray-300 text-apple-blue focus:ring-apple-blue cursor-pointer h-4 w-4 bg-transparent" checked={formData.events.includes(event)}
                          onChange={(e) => { const newEvents = e.target.checked ? [...formData.events, event] : formData.events.filter(ev => ev !== event); setFormData({ ...formData, events: newEvents }); }} />
                        <span className="text-sm font-medium" style={{ color: 'var(--theme-text)' }}>{event}</span>
                        <span className="text-xs ml-auto" style={{ color: 'var(--theme-text-muted)' }}>{t(descKey)}</span>
                      </label>
                    ))}
                  </div>
                </div>
                <div className="flex items-center gap-3 pt-2">
                  <div className="flex h-6 items-center">
                    <input id="isActive" type="checkbox" checked={formData.isActive} onChange={(e) => setFormData({ ...formData, isActive: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-apple-blue focus:ring-apple-blue bg-transparent cursor-pointer" />
                  </div>
                  <div className="text-sm">
                    <label htmlFor="isActive" className="font-medium cursor-pointer" style={{ color: 'var(--theme-text)' }}>{t('webhooks.active_label')}</label>
                    <p className="text-xs mt-0.5" style={{ color: 'var(--theme-text-muted)' }}>{t('webhooks.active_desc')}</p>
                  </div>
                </div>
                <div className="pt-4 flex items-center justify-end gap-3 border-t mt-6" style={{ borderColor: 'var(--theme-border)' }}>
                  <button type="button" onClick={() => setIsModalOpen(false)} className="px-4 py-2.5 rounded-xl text-sm font-medium transition-colors hover:opacity-80" style={{ color: 'var(--theme-text-secondary)' }}>{t('common.cancel')}</button>
                  <button type="submit" className="px-4 py-2.5 rounded-xl text-sm font-medium text-white shadow bg-apple-blue hover:bg-blue-600 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-apple-blue">
                    {editingWebhook ? t('webhooks.save_changes') : t('webhooks.create_webhook')}
                  </button>
                </div>
              </form>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}
