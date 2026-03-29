/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useMemo, useEffect, useCallback } from 'react';
import { useQuery, useMutation, useLazyQuery } from '@apollo/client/react';
import { GET_DLP_CONFIG, UPDATE_DLP_CONFIG, TEST_DLP_REDACTION } from '@/lib/graphql/operations/dlp';
import { MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import type { Organization, Project } from '@/lib/types';
import { useAuthStore } from '@/stores/authStore';
import toast from 'react-hot-toast';
import { detectActivePreset }  from './DlpConstants';
import type { PolicyPreset } from './DlpConstants';

export function useDlpSettings() {
  const [saving, setSaving] = useState(false);
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === 'admin';

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
  const [currentProjectId, setCurrentProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      if (!currentProjectId || !projects.find(p => p.id === currentProjectId)) setCurrentProjectId(projects[0].id);
    } else if (projects.length === 0 && currentProjectId) setCurrentProjectId('');
  }, [projects, currentProjectId]);

  // Local state
  const [testInput, setTestInput] = useState('');
  const [testResult, setTestResult] = useState<any>(null);
  const [customRegexInput, setCustomRegexInput] = useState('');

  // Queries
  const { data, loading } = useQuery<any>(GET_DLP_CONFIG, { variables: { projectId: currentProjectId }, skip: !currentProjectId, fetchPolicy: 'network-only' });
  const [updateDlp] = useMutation<any>(UPDATE_DLP_CONFIG, { refetchQueries: [{ query: GET_DLP_CONFIG, variables: { projectId: currentProjectId } }], awaitRefetchQueries: true });
  const [testDlp, { loading: testing }] = useLazyQuery<any>(TEST_DLP_REDACTION, { fetchPolicy: 'network-only' });

  const config = data?.getDlpConfig || null;
  const activePresetId = detectActivePreset(config);
  const isEnabled = config?.isEnabled ?? false;

  const applyPreset = useCallback(async (preset: PolicyPreset) => {
    try {
      setSaving(true);
      await updateDlp({ variables: { input: { projectId: currentProjectId, ...preset.config } } });
      toast.success(`Applied "${preset.name}" policy`);
    } catch (e: any) { toast.error(e.message || 'Failed to apply policy'); }
    finally { setSaving(false); }
  }, [currentProjectId, updateDlp]);

  const handleToggleEnable = useCallback(async (enabled: boolean) => {
    try { setSaving(true); await updateDlp({ variables: { input: { projectId: currentProjectId, isEnabled: enabled } } }); toast.success(enabled ? 'DLP Enabled' : 'DLP Disabled'); }
    catch (e: any) { toast.error(e.message || 'Failed to update DLP settings'); }
    finally { setSaving(false); }
  }, [currentProjectId, updateDlp]);

  const handleUpdateStrategy = useCallback(async (strategy: 'REDACT' | 'BLOCK') => {
    try { setSaving(true); await updateDlp({ variables: { input: { projectId: currentProjectId, strategy } } }); toast.success('Strategy updated'); }
    catch (e: any) { toast.error(e.message || 'Failed to update strategy'); }
    finally { setSaving(false); }
  }, [currentProjectId, updateDlp]);

  const handleToggleMask = useCallback(async (field: string, value: boolean) => {
    try { await updateDlp({ variables: { input: { projectId: currentProjectId, [field]: value } } }); }
    catch (e: any) { toast.error(e.message || 'Failed to update rule'); }
  }, [currentProjectId, updateDlp]);

  const handleAddCustomRegex = useCallback(async () => {
    if (!customRegexInput.trim()) return;
    try {
      const newArray = [...(config?.customRegex || []), customRegexInput.trim()];
      await updateDlp({ variables: { input: { projectId: currentProjectId, customRegex: newArray } } });
      setCustomRegexInput(''); toast.success('Custom pattern added');
    } catch (e: any) { toast.error(e.message || 'Failed to add custom rule'); }
  }, [customRegexInput, config, currentProjectId, updateDlp]);

  const handleRemoveCustomRegex = useCallback(async (index: number) => {
    const newArray = [...(config?.customRegex || [])]; newArray.splice(index, 1);
    try { await updateDlp({ variables: { input: { projectId: currentProjectId, customRegex: newArray } } }); }
    catch (e: any) { toast.error(e.message || 'Failed to remove custom rule'); }
  }, [config, currentProjectId, updateDlp]);

  const handleRunSandbox = useCallback(async () => {
    if (!testInput.trim()) return;
    try { const { data } = await testDlp({ variables: { projectId: currentProjectId, input: testInput } }); setTestResult(data?.testDlpRedaction); }
    catch (e: any) { toast.error(e.message || 'Sandbox test failed'); }
  }, [testInput, currentProjectId, testDlp]);

  const handlePublishToAllProjects = useCallback(async () => {
    if (!config) return;
    try {
      setSaving(true);
      const promises = projects.filter(p => p.id !== currentProjectId).map(p =>
        updateDlp({ variables: { input: { projectId: p.id, isEnabled: config.isEnabled, strategy: config.strategy, maskEmails: config.maskEmails, maskPhones: config.maskPhones, maskCreditCards: config.maskCreditCards, maskSsn: config.maskSsn, maskApiKeys: config.maskApiKeys, customRegex: config.customRegex || [] } } })
      );
      await Promise.all(promises); toast.success(`Policy published to ${promises.length} other project(s)`);
    } catch (e: any) { toast.error(e.message || 'Failed to publish policy'); }
    finally { setSaving(false); }
  }, [config, projects, currentProjectId, updateDlp]);

  return {
    saving, isAdmin,
    orgs, selectedOrgId, setSelectedOrgId,
    projects, currentProjectId, setCurrentProjectId,
    loading, config, activePresetId, isEnabled,
    testInput, setTestInput, testResult, testing,
    customRegexInput, setCustomRegexInput,
    applyPreset, handleToggleEnable, handleUpdateStrategy, handleToggleMask,
    handleAddCustomRegex, handleRemoveCustomRegex, handleRunSandbox, handlePublishToAllProjects,
  };
}
