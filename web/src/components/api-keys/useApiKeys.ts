/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useMemo, useEffect, useCallback } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { MY_API_KEYS, MY_ORGANIZATIONS, MY_PROJECTS, CREATE_API_KEY, REVOKE_API_KEY, DELETE_API_KEY, UPDATE_PROJECT } from '@/lib/graphql/operations';
import type { ApiKey, Organization, Project } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';
import { mapApiKey, AVAILABLE_SCOPES_BASE } from './ApiKeyComponents';
import toast from 'react-hot-toast';

export function useApiKeys() {
  const { t } = useTranslation();
  const AVAILABLE_SCOPES = useMemo(() => AVAILABLE_SCOPES_BASE.map(s => ({ ...s, label: t(s.labelKey) })), [t]);

  // Organization state
  const { data: orgData } = useQuery<any>(MY_ORGANIZATIONS);
  const orgs: Organization[] = useMemo(() => orgData?.myOrganizations || [], [orgData]);
  const [selectedOrgId, setSelectedOrgId] = useState<string>('');

  useEffect(() => {
    if (orgs.length > 0 && !selectedOrgId) setSelectedOrgId(orgs[0].id);
  }, [orgs, selectedOrgId]);

  // Project state
  const { data: projData } = useQuery<any>(MY_PROJECTS, { variables: { orgId: selectedOrgId }, skip: !selectedOrgId });
  const projects: Project[] = useMemo(() => projData?.myProjects || [], [projData]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      if (!selectedProjectId || !projects.find(p => p.id === selectedProjectId)) setSelectedProjectId(projects[0].id);
    } else if (projects.length === 0 && selectedProjectId) setSelectedProjectId('');
  }, [projects, selectedProjectId]);

  // API Keys
  const { data, loading, refetch } = useQuery<any>(MY_API_KEYS, { variables: { projectId: selectedProjectId }, skip: !selectedProjectId });
  const apiKeys: ApiKey[] = useMemo(() => (data?.myApiKeys || []).map(mapApiKey), [data]);

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showQuickGuide, setShowQuickGuide] = useState(false);
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

  // Project Settings
  const [isProjectSettingsOpen, setIsProjectSettingsOpen] = useState(false);
  const [projectWhiteListedIps, setProjectWhiteListedIps] = useState('');
  const [updatingProject, setUpdatingProject] = useState(false);

  // Confirm modal
  const [confirmModal, setConfirmModal] = useState<{ isOpen: boolean; type: 'revoke' | 'delete'; keyId: string }>({ isOpen: false, type: 'revoke', keyId: '' });
  const [processing, setProcessing] = useState(false);

  const handleCreate = useCallback(async () => {
    if (!newKeyName.trim()) { toast.error(t('api_keys.enter_name')); return; }
    if (!selectedProjectId) { toast.error(t('api_keys.select_project')); return; }
    setCreating(true);
    try {
      const scopeStr = selectedScopes.includes('all') ? 'all' : selectedScopes.join(',');
      const variables: any = { projectId: selectedProjectId, name: newKeyName.trim(), scopes: scopeStr };
      if (newKeyRateLimit) variables.rateLimit = parseInt(newKeyRateLimit, 10);
      if (newKeyTokenLimit) variables.tokenLimit = parseInt(newKeyTokenLimit, 10);
      const { data: result } = await createKeyMut({ variables });
      const key = mapApiKey((result as any)?.createApiKey);
      setCreatedKey(key);
      setShowCreateModal(false);
      await refetch();
      setNewKeyName(''); setSelectedScopes(['all']); setNewKeyRateLimit(''); setNewKeyTokenLimit('');
      toast.success(t('api_keys.created_success'));
    } catch (e: any) {
      toast.error(e.message || t('api_keys.create_error'));
    } finally { setCreating(false); }
  }, [newKeyName, selectedProjectId, selectedScopes, newKeyRateLimit, newKeyTokenLimit, createKeyMut, refetch, t]);

  const openRevokeModal = (id: string) => setConfirmModal({ isOpen: true, type: 'revoke', keyId: id });
  const openDeleteModal = (id: string) => setConfirmModal({ isOpen: true, type: 'delete', keyId: id });
  const closeConfirmModal = () => setConfirmModal({ isOpen: false, type: 'revoke', keyId: '' });

  const handleConfirmAction = useCallback(async () => {
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
    } finally { setProcessing(false); }
  }, [confirmModal, selectedProjectId, revokeKeyMut, deleteKeyMut, refetch, t]);

  const copyToClipboard = async (text: string) => {
    try { await navigator.clipboard.writeText(text); toast.success(t('common.copied_clipboard')); }
    catch { toast.error(t('common.copy_failed')); }
  };

  const openProjectSettings = () => {
    const p = projects.find(x => x.id === selectedProjectId);
    if (p) { setProjectWhiteListedIps(p.whiteListedIps || ''); setIsProjectSettingsOpen(true); }
  };

  const saveProjectSettings = async () => {
    setUpdatingProject(true);
    try {
      await updateProjectMut({ variables: { id: selectedProjectId, input: { whiteListedIps: projectWhiteListedIps.trim() } } });
      toast.success("Project settings updated");
      setIsProjectSettingsOpen(false);
    } catch (e: any) {
      toast.error(e.message || "Failed to update settings");
    } finally { setUpdatingProject(false); }
  };

  return {
    t,
    AVAILABLE_SCOPES,
    // Org/Project
    orgs, selectedOrgId, setSelectedOrgId,
    projects, selectedProjectId, setSelectedProjectId,
    // Keys
    apiKeys, loading,
    // Create
    showCreateModal, setShowCreateModal, newKeyName, setNewKeyName,
    selectedScopes, setSelectedScopes, newKeyRateLimit, setNewKeyRateLimit,
    newKeyTokenLimit, setNewKeyTokenLimit, createdKey, setCreatedKey, creating, handleCreate,
    // Quick guide
    showQuickGuide, setShowQuickGuide,
    // Actions
    openRevokeModal, openDeleteModal, closeConfirmModal, handleConfirmAction,
    confirmModal, processing, copyToClipboard,
    // Project settings
    isProjectSettingsOpen, setIsProjectSettingsOpen, projectWhiteListedIps, setProjectWhiteListedIps,
    updatingProject, openProjectSettings, saveProjectSettings,
  };
}
