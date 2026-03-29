/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useMemo, useEffect, useCallback } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import {
  GET_WEBHOOKS, CREATE_WEBHOOK_ENDPOINT, UPDATE_WEBHOOK_ENDPOINT,
  DELETE_WEBHOOK_ENDPOINT, TEST_WEBHOOK_ENDPOINT, GET_WEBHOOK_DELIVERIES,
} from '@/lib/graphql/operations/webhooks';
import { MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import type { Organization, Project } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';
import toast from 'react-hot-toast';

export interface WebhookFormData {
  url: string;
  description: string;
  events: string[];
  isActive: boolean;
}

export function useWebhooks() {
  const { t } = useTranslation();

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

  // Modal + editing
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<any>(null);
  const [selectedEndpointId, setSelectedEndpointId] = useState<string | null>(null);

  // Queries
  const { data, loading, refetch } = useQuery<any>(GET_WEBHOOKS, { variables: { projectId: selectedProjectId }, skip: !selectedProjectId });
  const { data: deliveriesData, loading: deliveriesLoading } = useQuery<any>(GET_WEBHOOK_DELIVERIES, { variables: { endpointId: selectedEndpointId, limit: 50 }, skip: !selectedEndpointId, pollInterval: 5000 });

  // Mutations
  const [createWebhook] = useMutation(CREATE_WEBHOOK_ENDPOINT, {
    onCompleted: (data: any) => {
      toast.success(t('webhooks.created_success'));
      if (data.createWebhookEndpoint.secret) {
        toast((_t) => (
          <div className="flex flex-col">
            <span className="font-medium text-amber-500">{t('webhooks.save_secret')}</span>
            <span className="text-xs text-gray-400 mt-1 break-all bg-gray-900/50 p-2 rounded border border-gray-700">{data.createWebhookEndpoint.secret}</span>
            <span className="text-xs mt-1">{t('webhooks.secret_not_shown')}</span>
          </div>
        ), { duration: 10000 });
      }
      refetch(); setIsModalOpen(false);
    },
    onError: (error: any) => toast.error(error.message),
  });

  const [updateWebhook] = useMutation(UPDATE_WEBHOOK_ENDPOINT, {
    onCompleted: () => { toast.success(t('webhooks.updated_success')); refetch(); setIsModalOpen(false); setEditingWebhook(null); },
    onError: (error: any) => toast.error(error.message),
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
    if (webhook) {
      setEditingWebhook(webhook);
      setFormData({ url: webhook.url, description: webhook.description || '', events: webhook.events || [], isActive: webhook.isActive });
    } else {
      setEditingWebhook(null);
      setFormData({ url: '', description: '', events: ['ping'], isActive: true });
    }
    setIsModalOpen(true);
  }, []);

  const handleSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedProjectId) return;
    if (editingWebhook) {
      updateWebhook({ variables: { id: editingWebhook.id, input: { url: formData.url, description: formData.description, events: formData.events, isActive: formData.isActive } } });
    } else {
      createWebhook({ variables: { input: { projectId: selectedProjectId, url: formData.url, description: formData.description, events: formData.events } } });
    }
  }, [selectedProjectId, editingWebhook, formData, updateWebhook, createWebhook]);

  const handleDelete = useCallback((id: string) => {
    if (window.confirm(t('webhooks.delete_confirm'))) deleteWebhook({ variables: { id } });
  }, [deleteWebhook, t]);

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
  };
}
