/* eslint-disable @typescript-eslint/no-explicit-any */
import { useMemo, useState, useEffect, useCallback } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { CombinedGraphQLErrors } from '@apollo/client/errors';
import { MY_API_KEYS, MY_ORGANIZATIONS, MY_PROJECTS, CREATE_API_KEY, REVOKE_API_KEY, DELETE_API_KEY, UPDATE_PROJECT } from '@/lib/graphql/operations';
import type { ApiKey } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';
import { useAuthStore } from '@/stores/authStore';
import { useAuthHydrated } from '@/hooks/useAuthHydrated';
import { mapApiKey, AVAILABLE_SCOPES_BASE } from './ApiKeyComponents';
import toast from 'react-hot-toast';

// FU-1: shape of the field-keyed inline-error map surfaced by the
// hook to the create-key dialog. Mirrors the WebhooksPage / LoginPage
// convention so the dialog can render <p role="alert"> under each input.
// Server keys the field via extensions.field — we forward those keys
// verbatim. `name` is the field the L-06 server validator exercises
// today; the others are wired for forward-compat with the API key
// rate-limit / scopes validators.
type FieldErrors = Partial<Record<'name' | 'rateLimit' | 'tokenLimit' | 'dailyLimit' | 'scopes', string>>;

const KNOWN_API_KEY_FIELDS: ReadonlySet<keyof FieldErrors> = new Set(['name', 'rateLimit', 'tokenLimit', 'dailyLimit', 'scopes']);

// Pull every VALIDATION-coded GraphQL error out of an Apollo onError
// payload and project it into a {field: message} map. Unknown fields
// (or VALIDATION without a field) land in otherMessages so the caller
// can still toast them rather than silently dropping the server reason.
function collectValidationErrors(error: unknown): { fieldErrors: FieldErrors; otherMessages: string[] } {
  const fieldErrors: FieldErrors = {};
  const otherMessages: string[] = [];
  if (!CombinedGraphQLErrors.is(error)) {
    if (error instanceof Error) otherMessages.push(error.message);
    return { fieldErrors, otherMessages };
  }
  for (const gqlError of error.errors) {
    const code = (gqlError.extensions?.code as string) || '';
    const field = (gqlError.extensions?.field as string) || '';
    if (code === 'VALIDATION' && field && KNOWN_API_KEY_FIELDS.has(field as keyof FieldErrors)) {
      fieldErrors[field as keyof FieldErrors] = gqlError.message;
      continue;
    }
    otherMessages.push(gqlError.message);
  }
  return { fieldErrors, otherMessages };
}

const parsePolicyList = (value: string) => Array.from(new Set(
  value
    .split(/[,\n]/)
    .map(item => item.trim())
    .filter(Boolean),
));

const DEFAULT_API_KEY_SCOPES = ['chat'];

// L-06: mirror of the server-side regex in apikey.resolvers.go. Soft check
// only — the backend is the source of truth and will return a typed
// VALIDATION error with field=name if the client mirror drifts. The
// （ / ） escapes are fullwidth parens, which appear naturally
// in Chinese / Japanese key names.
const API_KEY_NAME_PATTERN = /^[\p{L}\p{N}\s_.\-()（）]{1,64}$/u;

export function useApiKeys() {
  const { t } = useTranslation();
  const AVAILABLE_SCOPES = useMemo(() => AVAILABLE_SCOPES_BASE.map(s => ({ ...s, label: t(s.labelKey) })), [t]);

  // Organization state — sourced from the persisted auth store so it
  // stays in sync with OrgSwitcher across every page.
  const selectedOrgId = useAuthStore((s) => s.selectedOrgId) ?? '';
  const setSelectedOrgId = useAuthStore((s) => s.setSelectedOrgId);
  const hydrated = useAuthHydrated();

  const { data: orgData } = useQuery(MY_ORGANIZATIONS);
  // Apollo 4.2 infers the row shape from the typed document; downstream
  // callers (OrgSwitcher select, project picker) only touch `id` / `name`
  // so we deliberately keep the inferred shape rather than coerce to the
  // legacy snake_case Organization type.
  const orgs = useMemo(() => orgData?.myOrganizations || [], [orgData]);

  useEffect(() => {
    if (!hydrated) return;
    if (orgs.length > 0 && !selectedOrgId) setSelectedOrgId(orgs[0].id);
  }, [hydrated, orgs, selectedOrgId, setSelectedOrgId]);

  // Project state — same inference rationale as orgs above.
  const { data: projData } = useQuery(MY_PROJECTS, { variables: { orgId: selectedOrgId }, skip: !selectedOrgId });
  const projects = useMemo(() => projData?.myProjects || [], [projData]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      if (!selectedProjectId || !projects.find(p => p.id === selectedProjectId)) setSelectedProjectId(projects[0].id);
    } else if (projects.length === 0 && selectedProjectId) setSelectedProjectId('');
  }, [projects, selectedProjectId]);

  // API Keys
  const { data, loading, refetch } = useQuery(MY_API_KEYS, { variables: { projectId: selectedProjectId }, skip: !selectedProjectId });
  const apiKeys: ApiKey[] = useMemo(() => (data?.myApiKeys || []).map(mapApiKey), [data]);

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showQuickGuide, setShowQuickGuide] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<string[]>(DEFAULT_API_KEY_SCOPES);
  const [newAllowedModels, setNewAllowedModels] = useState('');
  const [newAllowedProviders, setNewAllowedProviders] = useState('');
  const [newKeyRateLimit, setNewKeyRateLimit] = useState<string>('');
  const [newKeyTokenLimit, setNewKeyTokenLimit] = useState<string>('');
  const [createdKey, setCreatedKey] = useState<ApiKey | null>(null);
  const [creating, setCreating] = useState(false);
  // FU-1: field-keyed inline errors. Populated by the create-key
  // mutation onError handler and cleared whenever the user edits the
  // matching input (see clearFieldError below) or re-opens the dialog.
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
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
    const trimmedName = newKeyName.trim();
    if (!trimmedName) { toast.error(t('api_keys.enter_name')); return; }
    // L-06: soft client-side mirror of the server-side regex; the server
    // returns a typed VALIDATION error with field=name if this slips past
    // (e.g. when the client mirror drifts) — that response is rendered
    // inline by the dialog via fieldErrors.name. Surface the same hint
    // inline here so the UX is consistent regardless of where the check
    // fires.
    if (!API_KEY_NAME_PATTERN.test(trimmedName)) {
      setFieldErrors({ name: t('api_keys.invalid_name') });
      return;
    }
    if (!selectedProjectId) { toast.error(t('api_keys.select_project')); return; }
    // FU-1: always clear stale inline errors before a new round-trip —
    // server failures will repopulate this via the onError path below.
    setFieldErrors({});
    setCreating(true);
    try {
      const scopeStr = selectedScopes.includes('all') ? 'all' : selectedScopes.join(',');
      const variables: any = { projectId: selectedProjectId, name: newKeyName.trim(), scopes: scopeStr };
      if (newKeyRateLimit) variables.rateLimit = parseInt(newKeyRateLimit, 10);
      if (newKeyTokenLimit) variables.tokenLimit = parseInt(newKeyTokenLimit, 10);
      const allowedModels = parsePolicyList(newAllowedModels);
      const allowedProviders = parsePolicyList(newAllowedProviders);
      if (allowedModels.length > 0) variables.allowedModels = allowedModels;
      if (allowedProviders.length > 0) variables.allowedProviders = allowedProviders;
      // Apollo's global errorPolicy is 'all', so GraphQL errors land in
      // result.error rather than being thrown. We forward that into the
      // catch site to keep the field-error wiring centralized.
      const result = await createKeyMut({ variables });
      if (result.error) throw result.error;
      const key = mapApiKey((result.data as any)?.createApiKey);
      setCreatedKey(key);
      setShowCreateModal(false);
      await refetch();
      setNewKeyName(''); setSelectedScopes(DEFAULT_API_KEY_SCOPES); setNewAllowedModels(''); setNewAllowedProviders(''); setNewKeyRateLimit(''); setNewKeyTokenLimit('');
      toast.success(t('api_keys.created_success'));
    } catch (e: any) {
      // FU-1: project any VALIDATION error with a known extensions.field
      // onto the inline-error map so the dialog renders <p role=alert>
      // under the offending input. Anything we can't pin to a field
      // falls back to a toast — matches the WebhooksPage policy.
      const { fieldErrors: next, otherMessages } = collectValidationErrors(e);
      if (Object.keys(next).length > 0) {
        setFieldErrors(next);
      } else if (otherMessages.length > 0) {
        toast.error(otherMessages.join('\n'));
      } else {
        toast.error(e.message || t('api_keys.create_error'));
      }
    } finally { setCreating(false); }
  }, [newKeyName, selectedProjectId, selectedScopes, newKeyRateLimit, newKeyTokenLimit, newAllowedModels, newAllowedProviders, createKeyMut, refetch, t]);

  // FU-1: lets the dialog's input change handlers wipe just the inline
  // error tied to that field as the user starts editing — mirrors the
  // LoginPage / WebhooksPage UX where the red message vanishes once the
  // user begins typing a fix.
  const clearFieldError = useCallback((field: keyof FieldErrors) => {
    setFieldErrors((prev) => {
      if (!(field in prev)) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
  }, []);

  // Reset fieldErrors whenever the create dialog opens, so stale errors
  // from a prior failed attempt don't leak across opens.
  useEffect(() => {
    if (showCreateModal) setFieldErrors({});
  }, [showCreateModal]);

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
    selectedScopes, setSelectedScopes, newAllowedModels, setNewAllowedModels,
    newAllowedProviders, setNewAllowedProviders, newKeyRateLimit, setNewKeyRateLimit,
    newKeyTokenLimit, setNewKeyTokenLimit, createdKey, setCreatedKey, creating, handleCreate,
    // FU-1: inline error surface for the create dialog.
    fieldErrors, clearFieldError,
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
