import { useState, useMemo, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  PlusIcon,
  TrashIcon,
  ClipboardIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
  ExclamationCircleIcon,
  KeyIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { useQuery, useMutation } from '@apollo/client/react';
import { MY_API_KEYS, MY_ORGANIZATIONS, MY_PROJECTS, CREATE_API_KEY, REVOKE_API_KEY, DELETE_API_KEY, UPDATE_PROJECT, API_KEY_RATE_LIMIT_STATUS } from '@/lib/graphql/operations';
import { SUBSCRIPTION_QUOTA_QUERY } from '@/lib/graphql/operations/billing';
import type { ApiKey, Organization, Project } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';

const AVAILABLE_SCOPES = [
  { id: 'all', label: t('api_keys.scopes_all') },
  { id: 'chat', label: t('api_keys.scopes_chat') },
  { id: 'embeddings', label: t('api_keys.scopes_embeddings') },
  { id: 'images', label: t('api_keys.scopes_images') },
  { id: 'audio', label: 'Audio & TTS' },
  { id: 'admin', label: 'Admin (Mgmt API)' },
];

/* eslint-disable @typescript-eslint/no-explicit-any */

const STATUS_BADGE: Record<string, { label: string; className: string }> = {
  ok: { label: t('api_keys.ok'), className: 'bg-green-50 text-green-700 border-green-200' },
  near_limit: { label: t('api_keys.near_limit'), className: 'bg-orange-50 text-orange-700 border-orange-200' },
  rate_limited: { label: t('api_keys.rate_limited'), className: 'bg-red-50 text-red-700 border-red-200' },
  quota_exceeded: { label: t('api_keys.quota_exceeded'), className: 'bg-red-50 text-red-700 border-red-200' },
};

function RateLimitMiniBar({ current, limit, label }: { current: number; limit: number; label: string }) {
  const { t } = useTranslation();
  if (limit <= 0) return <div className="text-[10px] text-apple-gray-400">{label}: Unlimited</div>;
  const pct = Math.min((current / limit) * 100, 100);
  const color = pct >= 100 ? 'bg-red-500' : pct >= 80 ? 'bg-orange-400' : 'bg-green-500';
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-[10px] text-apple-gray-400 w-8 shrink-0">{label}</span>
      <div className="flex-1 h-1.5 bg-apple-gray-100 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color} transition-all duration-300`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-[10px] text-apple-gray-500 w-16 text-right">{current}/{limit}</span>
    </div>
  );
}

function RateLimitStatusCell({ keyId, isActive }: { keyId: string; isActive: boolean }) {
  const { data } = useQuery<any>(API_KEY_RATE_LIMIT_STATUS, {
    variables: { keyId },
    skip: !isActive,
    pollInterval: 10000,
    fetchPolicy: 'network-only',
  });
  if (!isActive) return null;
  const s = data?.apiKeyRateLimitStatus;
  if (!s) return <span className="text-[10px] text-apple-gray-300">—</span>;
  const badge = STATUS_BADGE[s.status] || STATUS_BADGE.ok;
  return (
    <div className="space-y-1.5">
      <span className={`inline-flex px-1.5 py-0.5 rounded-md text-[10px] font-medium border ${badge.className}`}>
        {badge.label}
      </span>
      <RateLimitMiniBar current={s.rpmCurrent} limit={s.rpmLimit} label={t('api_keys.rpm')} />
      <RateLimitMiniBar current={s.tpmCurrent} limit={s.tpmLimit} label={t('api_keys.tpm')} />
      <RateLimitMiniBar current={s.dailyCurrent} limit={s.dailyLimit} label={t('api_keys.daily')} />
    </div>
  );
}

function SubscriptionQuotaBanner() {
  const { data } = useQuery<any>(SUBSCRIPTION_QUOTA_QUERY, { fetchPolicy: 'cache-and-network' });
  const sub = data?.mySubscription;
  if (!sub || sub.tokenLimit <= 0) return null;

  const pct = sub.quotaPercentage;
  const isExceeded = sub.isQuotaExceeded;
  const isNear = pct >= 80 && !isExceeded;

  const barColor = isExceeded ? 'bg-red-500' : isNear ? 'bg-orange-400' : 'bg-blue-500';
  const bgColor = isExceeded ? 'bg-red-50 border-red-200' : isNear ? 'bg-orange-50 border-orange-200' : 'bg-blue-50 border-blue-200';
  const textColor = isExceeded ? 'text-red-700' : isNear ? 'text-orange-700' : 'text-blue-700';
  const iconColor = isExceeded ? 'text-red-500' : isNear ? 'text-orange-500' : 'text-blue-500';

  const fmtTokens = (n: number) => n >= 1000000 ? `${(n / 1000000).toFixed(1)}M` : n >= 1000 ? `${(n / 1000).toFixed(1)}K` : `${n}`;

  return (
    <motion.div
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      className={`rounded-apple-lg border p-4 ${bgColor}`}
    >
      <div className="flex items-center gap-3">
        <ExclamationCircleIcon className={`w-5 h-5 shrink-0 ${iconColor}`} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-1">
            <span className={`text-sm font-medium ${textColor}`}>
              {sub.planName} Plan -- Token Quota
            </span>
            <span className={`text-xs font-medium ${textColor}`}>
              {fmtTokens(sub.usedTokens)} / {fmtTokens(sub.tokenLimit)}
              {isExceeded && ' (Exceeded)'}
            </span>
          </div>
          <div className="h-2 bg-white/60 rounded-full overflow-hidden">
            <div className={`h-full rounded-full ${barColor} transition-all duration-500`} style={{ width: `${pct}%` }} />
          </div>
          {isExceeded && (
            <p className="text-xs text-red-600 mt-1">
              Monthly token limit reached. API requests will be rejected until the next billing period.
            </p>
          )}
        </div>
      </div>
    </motion.div>
  );
}

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
        className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
      >
        <div className="flex items-start gap-4">
          <div
            className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${confirmColor === 'red' ? 'bg-red-100' : 'bg-orange-100'
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

function mapApiKey(d: any): ApiKey {
  return {
    id: d.id, project_id: d.projectId, channel: d.channel, name: d.name, key: d.key || '', key_prefix: d.keyPrefix,
    is_active: d.isActive, scopes: d.scopes, rate_limit: d.rateLimit, token_limit: d.tokenLimit, daily_limit: d.dailyLimit,
    created_at: d.createdAt, last_used_at: d.lastUsedAt, expires_at: d.expiresAt,
  };
}

function ApiKeysPage() {
  // Organization state
  const { data: orgData } = useQuery<any>(MY_ORGANIZATIONS);
  const orgs: Organization[] = useMemo(() => orgData?.myOrganizations || [], [orgData]);
  const [selectedOrgId, setSelectedOrgId] = useState<string>('');

  useEffect(() => {
    if (orgs.length > 0 && !selectedOrgId) {
      setSelectedOrgId(orgs[0].id);
    }
  }, [orgs, selectedOrgId]);

  // Project state
  const { data: projData } = useQuery<any>(MY_PROJECTS, {
    variables: { orgId: selectedOrgId },
    skip: !selectedOrgId,
  });
  const projects: Project[] = useMemo(() => projData?.myProjects || [], [projData]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      // If we swapped orgs and the old project doesn't exist in the new org
      if (!selectedProjectId || !projects.find(p => p.id === selectedProjectId)) {
        setSelectedProjectId(projects[0].id);
      }
    } else if (projects.length === 0 && selectedProjectId) {
      setSelectedProjectId('');
    }
  }, [projects, selectedProjectId]);

  const { data, loading, refetch } = useQuery<any>(MY_API_KEYS, {
    variables: { projectId: selectedProjectId },
    skip: !selectedProjectId,
  });
  const apiKeys: ApiKey[] = useMemo(() => (data?.myApiKeys || []).map(mapApiKey), [data]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<string[]>(['all']);
  const [newKeyRateLimit, setNewKeyRateLimit] = useState<string>('');
  const [newKeyTokenLimit, setNewKeyTokenLimit] = useState<string>('');
  const [createdKey, setCreatedKey] = useState<ApiKey | null>(null);
  const [creating, setCreating] = useState(false);
  const [createKeyMut] = useMutation(CREATE_API_KEY);
  const [revokeKeyMut] = useMutation(REVOKE_API_KEY);
  const [deleteKeyMut] = useMutation(DELETE_API_KEY);
  const [updateProjectMut] = useMutation(UPDATE_PROJECT);

  // Project Settings Modal State
  const [isProjectSettingsOpen, setIsProjectSettingsOpen] = useState(false);
  const [projectWhiteListedIps, setProjectWhiteListedIps] = useState('');
  const [updatingProject, setUpdatingProject] = useState(false);

  // Confirm modal state
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    type: 'revoke' | 'delete';
    keyId: string;
  }>({ isOpen: false, type: 'revoke', keyId: '' });
  const [processing, setProcessing] = useState(false);

  const handleCreate = async () => {
    if (!newKeyName.trim()) {
      toast.error(t('api_keys.enter_name'));
      return;
    }

    if (!selectedProjectId) {
      toast.error(t('api_keys.select_project'));
      return;
    }

    setCreating(true);
    try {
      const scopeStr = selectedScopes.includes('all') ? 'all' : selectedScopes.join(',');
      
      const variables: any = { 
        projectId: selectedProjectId, 
        name: newKeyName.trim(), 
        scopes: scopeStr 
      };
      if (newKeyRateLimit) variables.rateLimit = parseInt(newKeyRateLimit, 10);
      if (newKeyTokenLimit) variables.tokenLimit = parseInt(newKeyTokenLimit, 10);

      const { data: result } = await createKeyMut({
        variables
      });
      const key = mapApiKey((result as any)?.createApiKey);
      setCreatedKey(key);
      setShowCreateModal(false);
      await refetch();
      setNewKeyName('');
      setSelectedScopes(['all']);
      setNewKeyRateLimit('');
      setNewKeyTokenLimit('');
      toast.success(t('api_keys.created_success'));
    } catch (e: any) {
      toast.error(e.message || t('api_keys.create_error'));
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
        await revokeKeyMut({ variables: { projectId: selectedProjectId, id: keyId } });
        toast.success(t('api_keys.revoked_success'));
      } else {
        await deleteKeyMut({ variables: { projectId: selectedProjectId, id: keyId } });
        toast.success(t('api_keys.deleted_success'));
      }
      await refetch();
      closeConfirmModal();
    } catch {
      toast.error(type === 'revoke' ? t('api_keys.revoke_error') : t('api_keys.delete_error'));
    } finally {
      setProcessing(false);
    }
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(t('common.copied_clipboard'));
    } catch {
      toast.error(t('common.copy_failed'));
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
    <div className="space-y-6">
      <SubscriptionQuotaBanner />
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('api_keys.title')}</h1>
          <p className="text-apple-gray-500 mt-1">{t('api_keys.subtitle')}</p>
          
          <div className="mt-4 flex gap-4 items-end">
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-apple-gray-500">{t('common.organization')}</label>
              <select
                value={selectedOrgId}
                onChange={(e) => setSelectedOrgId(e.target.value)}
                className="input py-2 pl-3 pr-8 min-w-[220px]"
              >
                {orgs.map(org => (
                  <option key={org.id} value={org.id}>{org.name}</option>
                ))}
              </select>
            </div>
            
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-apple-gray-500">{t('common.project')}</label>
              <select
                value={selectedProjectId}
                onChange={(e) => setSelectedProjectId(e.target.value)}
                className="input py-2 pl-3 pr-8 min-w-[220px]"
                disabled={!projects.length}
              >
                {projects.length === 0 && <option value="">{t('common.no_projects')}</option>}
                {projects.map(proj => (
                  <option key={proj.id} value={proj.id}>{proj.name}</option>
                ))}
              </select>
            </div>
            {selectedProjectId && (
              <button
                onClick={() => {
                  const p = projects.find(x => x.id === selectedProjectId);
                  if (p) {
                    setProjectWhiteListedIps(p.whiteListedIps || '');
                    setIsProjectSettingsOpen(true);
                  }
                }}
                className="btn btn-secondary px-3"
                title={t('api_keys.project_settings')}
              >
                Settings
              </button>
            )}
          </div>
        </div>
        {apiKeys.length > 0 && (
          <button 
            onClick={() => setShowCreateModal(true)} 
            className="btn btn-primary"
            disabled={!selectedProjectId}
          >
            <PlusIcon className="w-5 h-5 mr-2" />
            Create API Key
          </button>
        )}
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
              <div className="flex items-center gap-2 bg-[var(--theme-bg-input)] rounded-apple border border-apple-gray-200 p-3">
                <code className="text-sm text-apple-gray-900 flex-1 break-all">
                  {createdKey.key}
                </code>
                <button
                  onClick={() => copyToClipboard(createdKey.key)}
                  className="btn btn-ghost p-2"
                  title={t('api_keys.copy_clipboard')}
                >
                  <ClipboardIcon className="w-5 h-5" />
                </button>
              </div>
            </div>
            <button onClick={() => setCreatedKey(null)} className="text-apple-gray-400 hover:text-apple-gray-600">
              <span className="sr-only">{t('common.dismiss')}</span>
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
          <div className="text-center py-16">
            <div className="w-16 h-16 bg-blue-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
              <KeyIcon className="w-8 h-8 text-apple-blue" />
            </div>
            <h3 className="text-lg font-semibold text-apple-gray-900 mb-1">{t('api_keys.no_keys')}</h3>
            <p className="text-apple-gray-500 text-sm mb-6 max-w-sm mx-auto">
              Create an API key to start routing requests through the LLM Router.
            </p>
            <button onClick={() => setShowCreateModal(true)} className="btn btn-primary rounded-xl">
              Create your first API key
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
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
                    <td className="table-cell">
                      <code className="text-sm bg-apple-gray-100 px-2 py-1 rounded">
                        {key.key_prefix}...
                      </code>
                    </td>
                    <td className="table-cell">
                      <span className={key.is_active ? 'badge-success' : 'badge-error'}>
                        {key.is_active ? t('common.active') : t('common.revoked')}
                      </span>
                    </td>
                    <td className="table-cell">
                      <div className="flex flex-wrap gap-1">
                        {key.scopes === 'all' ? (
                          <span className="badge-purple">{t('common.all')}</span>
                        ) : (
                          key.scopes?.split(',').map((s: string) => (
                            <span key={s} className="px-2 py-0.5 rounded-full bg-apple-gray-100 text-apple-gray-600 text-xs border border-apple-gray-200">
                              {s}
                            </span>
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
                            title={t('api_keys.revoke_key')}
                          >
                            <XCircleIcon className="w-5 h-5" />
                          </button>
                        )}
                        <button
                          onClick={() => openDeleteModal(key.id)}
                          className="text-apple-red hover:text-red-600 transition-colors"
                          title={t('api_keys.delete_key')}
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
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
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
                        <input
                          type="checkbox"
                          className="w-4 h-4 text-apple-blue border-apple-gray-300 rounded focus:ring-apple-blue transition-all"
                          checked={isChecked || isDisabled}
                          disabled={isDisabled}
                          onChange={(e) => {
                            if (scope.id === 'all') {
                              setSelectedScopes(e.target.checked ? ['all'] : []);
                            } else {
                              if (e.target.checked) {
                                setSelectedScopes([...selectedScopes.filter(s => s !== 'all'), scope.id]);
                              } else {
                                setSelectedScopes(selectedScopes.filter(s => s !== scope.id));
                              }
                            }
                          }}
                        />
                      </div>
                      <div className="flex-1">
                        <p className="text-sm font-medium text-apple-gray-900">{scope.label}</p>
                      </div>
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4 mb-6">
              <div>
                <label htmlFor="rateLimit" className="label">
                  Requests/Min (RPM)
                </label>
                <input
                  type="number"
                  id="rateLimit"
                  value={newKeyRateLimit}
                  onChange={(e) => setNewKeyRateLimit(e.target.value)}
                  className="input mt-1 block w-full"
                  placeholder="Unlimited (1000)"
                />
              </div>
              <div>
                <label htmlFor="tokenLimit" className="label">
                  Tokens/Min (TPM)
                </label>
                <input
                  type="number"
                  id="tokenLimit"
                  value={newKeyTokenLimit}
                  onChange={(e) => setNewKeyTokenLimit(e.target.value)}
                  className="input mt-1 block w-full"
                  placeholder="Unlimited (0)"
                />
              </div>
            </div>

            <div className="flex justify-end gap-3">
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewKeyName('');
                  setSelectedScopes(['all']);
                  setNewKeyRateLimit('');
                  setNewKeyTokenLimit('');
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

      {/* Project Settings Modal */}
      {isProjectSettingsOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4"
          >
            <h3 className="text-xl font-semibold text-apple-gray-900 mb-6">Project Settings</h3>
            
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                  API IP Whitelist
                </label>
                <textarea
                  value={projectWhiteListedIps}
                  onChange={(e) => setProjectWhiteListedIps(e.target.value)}
                  placeholder="e.g. 192.168.1.1, 10.0.0.0/24 (comma separated)"
                  rows={4}
                  className="input w-full font-mono text-sm"
                />
                <p className="mt-2 text-xs text-apple-gray-500 max-w">
                  Restrict API key usage to specific IP addresses or CIDR blocks. Leave empty to allow any IP. <strong>Note:</strong> This takes effect for all API keys in this project.
                </p>
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-8">
              <button
                onClick={() => setIsProjectSettingsOpen(false)}
                className="btn btn-secondary"
                disabled={updatingProject}
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  setUpdatingProject(true);
                  try {
                    await updateProjectMut({
                      variables: {
                        id: selectedProjectId,
                        input: { whiteListedIps: projectWhiteListedIps.trim() }
                      }
                    });
                    toast.success("Project settings updated");
                    setIsProjectSettingsOpen(false);
                    // This updates the local cache automatically because of Apollo cache normalization if id matches
                  } catch (e: any) {
                    toast.error(e.message || "Failed to update settings");
                  } finally {
                    setUpdatingProject(false);
                  }
                }}
                className="btn btn-primary"
                disabled={updatingProject}
              >
                {updatingProject ? 'Saving...' : 'Save Settings'}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default ApiKeysPage;
