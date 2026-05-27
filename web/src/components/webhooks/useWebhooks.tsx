/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useMemo, useEffect, useCallback } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { CombinedGraphQLErrors } from '@apollo/client/errors';
import {
  GET_WEBHOOKS, CREATE_WEBHOOK_ENDPOINT, UPDATE_WEBHOOK_ENDPOINT,
  DELETE_WEBHOOK_ENDPOINT, TEST_WEBHOOK_ENDPOINT, GET_WEBHOOK_DELIVERIES,
} from '@/lib/graphql/operations/webhooks';
import { MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import type { Organization, Project } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';
import { useAuthStore } from '@/stores/authStore';
import { useAuthHydrated } from '@/hooks/useAuthHydrated';
import { useVisibilityAwarePolling } from '@/hooks/useVisibilityAwarePolling';
import toast from 'react-hot-toast';

export interface WebhookFormData {
  url: string;
  description: string;
  events: string[];
  isActive: boolean;
}

// H-05: shape of the field-keyed inline-error map surfaced by the
// webhook hook to the page. Mirrors the LoginPage convention so the
// dialog can render <p role="alert"> next to each input. Server keys
// the field via extensions.field — we forward those keys verbatim.
type FieldErrors = Partial<Record<'url' | 'events' | 'description' | 'projectId' | 'id', string>>;

// Pull every VALIDATION-coded GraphQL error out of an Apollo
// onError payload and project it into a {field: message} map. Errors
// without an extensions.field land under '_form' (not currently
// rendered, but kept so we never silently drop a server message).
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
    if (code === 'VALIDATION' && field) {
      // Only map fields the form actually knows how to display; unknown
      // keys still get surfaced via otherMessages as a last resort.
      if (field === 'url' || field === 'events' || field === 'description' || field === 'projectId' || field === 'id') {
        fieldErrors[field as keyof FieldErrors] = gqlError.message;
        continue;
      }
    }
    if (code !== 'VALIDATION') {
      otherMessages.push(gqlError.message);
    } else {
      // VALIDATION without a usable field — bubble to the user via toast
      // so they aren't left guessing.
      otherMessages.push(gqlError.message);
    }
  }
  return { fieldErrors, otherMessages };
}

export function useWebhooks() {
  const { t } = useTranslation();

  // Organization state — sourced from the persisted auth store so
  // OrgSwitcher selections propagate here without page-local drift.
  const selectedOrgId = useAuthStore((s) => s.selectedOrgId) ?? '';
  const setSelectedOrgId = useAuthStore((s) => s.setSelectedOrgId);
  const hydrated = useAuthHydrated();

  const { data: orgData } = useQuery<any>(MY_ORGANIZATIONS);
  const orgs: Organization[] = useMemo(() => orgData?.myOrganizations || [], [orgData]);

  useEffect(() => {
    if (!hydrated) return;
    if (orgs.length > 0 && !selectedOrgId) setSelectedOrgId(orgs[0].id);
  }, [hydrated, orgs, selectedOrgId, setSelectedOrgId]);

  // Project state
  const { data: projData } = useQuery<any>(MY_PROJECTS, { variables: { orgId: selectedOrgId }, skip: !selectedOrgId });
  const projects: Project[] = useMemo(() => projData?.myProjects || [], [projData]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      if (!selectedProjectId || !projects.find(p => p.id === selectedProjectId)) setSelectedProjectId(projects[0].id);
    } else if (projects.length === 0 && selectedProjectId) setSelectedProjectId('');
  }, [projects, selectedProjectId]);

  // Modal + editing
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<any>(null);
  const [selectedEndpointId, setSelectedEndpointId] = useState<string | null>(null);
  // H-05: field-keyed inline errors. Populated by the create/update
  // mutation onError handlers and cleared whenever the user edits the
  // matching field (see clearFieldError below) or closes the dialog.
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  // Queries
  const { data, loading, refetch } = useQuery<any>(GET_WEBHOOKS, { variables: { projectId: selectedProjectId }, skip: !selectedProjectId });
  const deliveriesPollMs = 5000;
  const deliveriesQuery = useQuery<any>(GET_WEBHOOK_DELIVERIES, { variables: { endpointId: selectedEndpointId, limit: 50 }, skip: !selectedEndpointId, pollInterval: deliveriesPollMs });
  useVisibilityAwarePolling(deliveriesQuery, deliveriesPollMs);
  const { data: deliveriesData, loading: deliveriesLoading } = deliveriesQuery;

  // handleMutationError is the shared onError sink for the
  // create/update mutations. It pulls VALIDATION errors with a known
  // field into the inline state and toasts anything left over so the
  // user always gets some feedback. The Apollo errorLink already
  // suppresses VALIDATION toasts globally, so we own the surfacing
  // here.
  const handleMutationError = useCallback((error: unknown) => {
    const { fieldErrors: next, otherMessages } = collectValidationErrors(error);
    if (Object.keys(next).length > 0) {
      setFieldErrors(next);
    }
    // Only toast if we couldn't pin the error to a field — pinned
    // errors render inline next to the offending input.
    if (Object.keys(next).length === 0 && otherMessages.length > 0) {
      toast.error(otherMessages.join('\n'));
    }
  }, []);

  // Mutations
  const [createWebhook] = useMutation(CREATE_WEBHOOK_ENDPOINT, {
    onCompleted: (data: any) => {
      setFieldErrors({});
      toast.success(t('webhooks.created_success'));
      if (data.createWebhookEndpoint.secret) {
        toast((_t) => (
          <div className="flex flex-col">
            <span className="font-medium text-amber-500">{t('webhooks.save_secret')}</span>
            <span className="text-xs text-apple-gray-600 dark:text-gray-400 mt-1 break-all bg-apple-gray-100 dark:bg-gray-900/50 p-2 rounded border border-apple-gray-200 dark:border-gray-700 font-mono">{data.createWebhookEndpoint.secret}</span>
            <span className="text-xs mt-1">{t('webhooks.secret_not_shown')}</span>
          </div>
        ), { duration: 10000 });
      }
      refetch(); setIsModalOpen(false);
    },
    onError: handleMutationError,
  });

  const [updateWebhook] = useMutation(UPDATE_WEBHOOK_ENDPOINT, {
    onCompleted: () => { setFieldErrors({}); toast.success(t('webhooks.updated_success')); refetch(); setIsModalOpen(false); setEditingWebhook(null); },
    onError: handleMutationError,
  });

  const [deleteWebhook] = useMutation(DELETE_WEBHOOK_ENDPOINT, {
    onCompleted: () => { toast.success(t('webhooks.deleted_success')); if (selectedEndpointId === editingWebhook?.id) setSelectedEndpointId(null); refetch(); },
    onError: (error: any) => toast.error(error.message),
  });

  const [testWebhook] = useMutation(TEST_WEBHOOK_ENDPOINT, {
    onCompleted: () => toast.success(t('webhooks.test_success')),
    onError: (error: any) => toast.error(error.message),
  });

  // Form data
  const [formData, setFormData] = useState<WebhookFormData>({ url: '', description: '', events: ['ping', 'payment.succeeded'], isActive: true });

  const handleOpenModal = useCallback((webhook: any = null) => {
    // H-05: opening the modal must always reset any stale field errors
    // — otherwise a previous failed submit (e.g. private URL) would
    // show a red message under a freshly-cleared input.
    setFieldErrors({});
    if (webhook) {
      setEditingWebhook(webhook);
      setFormData({ url: webhook.url, description: webhook.description || '', events: webhook.events || [], isActive: webhook.isActive });
    } else {
      setEditingWebhook(null);
      setFormData({ url: '', description: '', events: ['ping'], isActive: true });
    }
    setIsModalOpen(true);
  }, []);

  // clearFieldError lets input change handlers wipe just the inline
  // error tied to that input as the user starts typing — mirrors the
  // LoginPage UX where the red message vanishes once the user begins
  // editing.
  const clearFieldError = useCallback((field: keyof FieldErrors) => {
    setFieldErrors((prev) => {
      if (!(field in prev)) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
  }, []);

  const handleSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedProjectId) return;
    // Always reset before a new round-trip — server failures will
    // repopulate this from onError.
    setFieldErrors({});
    if (editingWebhook) {
      updateWebhook({ variables: { id: editingWebhook.id, input: { url: formData.url, description: formData.description, events: formData.events, isActive: formData.isActive } } });
    } else {
      createWebhook({ variables: { input: { projectId: selectedProjectId, url: formData.url, description: formData.description, events: formData.events, isActive: formData.isActive } } });
    }
  }, [selectedProjectId, editingWebhook, formData, updateWebhook, createWebhook]);

  const handleDelete = useCallback((id: string) => {
    if (window.confirm(t('webhooks.delete_confirm'))) {
      if (selectedEndpointId === id) setSelectedEndpointId(null);
      deleteWebhook({ variables: { id } });
    }
  }, [deleteWebhook, selectedEndpointId, t]);

  const handleTest = useCallback((id: string) => {
    setSelectedEndpointId(id);
    testWebhook({ variables: { id } });
  }, [testWebhook]);

  const parseJson = (str: string) => {
    try { return JSON.stringify(JSON.parse(str), null, 2); }
    catch { return str; }
  };

  return {
    t,
    orgs, selectedOrgId, setSelectedOrgId,
    projects, selectedProjectId, setSelectedProjectId,
    isModalOpen, setIsModalOpen, editingWebhook,
    selectedEndpointId, setSelectedEndpointId,
    data, loading, deliveriesData, deliveriesLoading,
    formData, setFormData,
    handleOpenModal, handleSubmit, handleDelete, handleTest,
    parseJson,
    // H-05: inline error surface.
    fieldErrors,
    clearFieldError,
  };
}
