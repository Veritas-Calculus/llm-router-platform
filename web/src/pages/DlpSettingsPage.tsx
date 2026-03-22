import { useState, useEffect, useMemo } from 'react';
import { useQuery, useMutation, useLazyQuery } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { ShieldCheckIcon, HandRaisedIcon, XMarkIcon, SparklesIcon, LockClosedIcon, AdjustmentsHorizontalIcon, CheckCircleIcon, EnvelopeIcon, DevicePhoneMobileIcon, CreditCardIcon, IdentificationIcon, KeyIcon, BeakerIcon } from '@heroicons/react/24/outline';
import { GET_DLP_CONFIG, UPDATE_DLP_CONFIG, TEST_DLP_REDACTION } from '@/lib/graphql/operations/dlp';
import { MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import type { Organization, Project } from '@/lib/types';
import { useAuthStore } from '@/stores/authStore';

/* eslint-disable @typescript-eslint/no-explicit-any */

// ── Policy Preset Definitions ──────────────────────────────────────
interface PolicyPreset {
  id: string;
  name: string;
  description: string;
  icon: React.ReactNode;
  color: string;
  borderColor: string;
  bgColor: string;
  config: {
    isEnabled: boolean;
    strategy: 'REDACT' | 'BLOCK';
    maskEmails: boolean;
    maskPhones: boolean;
    maskCreditCards: boolean;
    maskSsn: boolean;
    maskApiKeys: boolean;
  };
}

const POLICY_PRESETS: PolicyPreset[] = [
  {
    id: 'relaxed',
    name: 'Relaxed',
    description: 'Minimal protection — only masks credit cards and SSNs. Suitable for internal development environments.',
    icon: <SparklesIcon className="w-5 h-5" />,
    color: 'text-blue-500',
    borderColor: 'border-blue-500',
    bgColor: 'bg-blue-500/10',
    config: {
      isEnabled: true,
      strategy: 'REDACT',
      maskEmails: false,
      maskPhones: false,
      maskCreditCards: true,
      maskSsn: true,
      maskApiKeys: false,
    },
  },
  {
    id: 'standard',
    name: 'Standard',
    description: 'Balanced protection — redacts emails, phones, cards, and SSNs. Recommended for most production workloads.',
    icon: <ShieldCheckIcon className="w-5 h-5" />,
    color: 'text-green-500',
    borderColor: 'border-green-500',
    bgColor: 'bg-green-500/10',
    config: {
      isEnabled: true,
      strategy: 'REDACT',
      maskEmails: true,
      maskPhones: true,
      maskCreditCards: true,
      maskSsn: true,
      maskApiKeys: false,
    },
  },
  {
    id: 'strict',
    name: 'Strict',
    description: 'Maximum protection — blocks any request containing PII. Use for regulated industries (HIPAA, PCI-DSS).',
    icon: <LockClosedIcon className="w-5 h-5" />,
    color: 'text-red-500',
    borderColor: 'border-red-500',
    bgColor: 'bg-red-500/10',
    config: {
      isEnabled: true,
      strategy: 'BLOCK',
      maskEmails: true,
      maskPhones: true,
      maskCreditCards: true,
      maskSsn: true,
      maskApiKeys: true,
    },
  },
  {
    id: 'custom',
    name: 'Custom',
    description: 'Fine-tune each rule individually. Full control over strategy, patterns, and custom regex.',
    icon: <AdjustmentsHorizontalIcon className="w-5 h-5" />,
    color: 'text-purple-500',
    borderColor: 'border-purple-500',
    bgColor: 'bg-purple-500/10',
    config: {
      isEnabled: true,
      strategy: 'REDACT',
      maskEmails: true,
      maskPhones: true,
      maskCreditCards: true,
      maskSsn: true,
      maskApiKeys: true,
    },
  },
];

// ── Detect which preset matches the current config ──────────────
function detectActivePreset(config: any): string {
  if (!config?.isEnabled) return 'none';
  for (const preset of POLICY_PRESETS) {
    if (preset.id === 'custom') continue;
    const p = preset.config;
    if (
      config.strategy === p.strategy &&
      config.maskEmails === p.maskEmails &&
      config.maskPhones === p.maskPhones &&
      config.maskCreditCards === p.maskCreditCards &&
      config.maskSsn === p.maskSsn &&
      config.maskApiKeys === p.maskApiKeys
    ) {
      return preset.id;
    }
  }
  return 'custom';
}

// ── PII Rule Definitions ────────────────────────────────────────
const PII_RULES = [
  { field: 'maskEmails', label: 'Email Addresses', desc: 'user@example.com, admin@corp.org', icon: EnvelopeIcon },
  { field: 'maskPhones', label: 'Phone Numbers', desc: '+1 (555) 123-4567, 138-0000-0000', icon: DevicePhoneMobileIcon },
  { field: 'maskCreditCards', label: 'Credit Card Numbers', desc: '4111-1111-1111-1111 (16-digit PANs)', icon: CreditCardIcon },
  { field: 'maskSsn', label: 'Social Security Numbers', desc: '123-45-6789 (US SSN format)', icon: IdentificationIcon },
  { field: 'maskApiKeys', label: 'API Keys & Secrets', desc: 'sk-..., Bearer tokens, AWS keys', icon: KeyIcon },
];

export default function DlpSettingsPage() {
  const [saving, setSaving] = useState(false);
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === 'admin';

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
  const [currentProjectId, setCurrentProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      if (!currentProjectId || !projects.find(p => p.id === currentProjectId)) {
        setCurrentProjectId(projects[0].id);
      }
    } else if (projects.length === 0 && currentProjectId) {
      setCurrentProjectId('');
    }
  }, [projects, currentProjectId]);
  
  // Local state
  const [testInput, setTestInput] = useState('');
  const [testResult, setTestResult] = useState<any>(null);
  const [customRegexInput, setCustomRegexInput] = useState('');

  // Queries
  const { data, loading } = useQuery<any>(GET_DLP_CONFIG, {
    variables: { projectId: currentProjectId },
    skip: !currentProjectId,
    fetchPolicy: 'network-only',
  });

  const [updateDlp] = useMutation<any>(UPDATE_DLP_CONFIG, {
    refetchQueries: [{ query: GET_DLP_CONFIG, variables: { projectId: currentProjectId } }],
    awaitRefetchQueries: true,
  });
  const [testDlp, { loading: testing }] = useLazyQuery<any>(TEST_DLP_REDACTION, {
    fetchPolicy: 'network-only',
  });

  const config = data?.getDlpConfig || null;
  const activePresetId = detectActivePreset(config);
  const isEnabled = config?.isEnabled ?? false;

  // ── Handlers ──────────────────────────────────────────────────
  const applyPreset = async (preset: PolicyPreset) => {
    try {
      setSaving(true);
      await updateDlp({
        variables: {
          input: {
            projectId: currentProjectId,
            isEnabled: preset.config.isEnabled,
            strategy: preset.config.strategy,
            maskEmails: preset.config.maskEmails,
            maskPhones: preset.config.maskPhones,
            maskCreditCards: preset.config.maskCreditCards,
            maskSsn: preset.config.maskSsn,
            maskApiKeys: preset.config.maskApiKeys,
          },
        },
      });
      toast.success(`Applied "${preset.name}" policy`);
    } catch (e: any) {
      toast.error(e.message || 'Failed to apply policy');
    } finally {
      setSaving(false);
    }
  };

  const handleToggleEnable = async (enabled: boolean) => {
    try {
      setSaving(true);
      await updateDlp({
        variables: { input: { projectId: currentProjectId, isEnabled: enabled } },
      });
      toast.success(enabled ? 'DLP Enabled' : 'DLP Disabled');
    } catch (e: any) {
      toast.error(e.message || 'Failed to update DLP settings');
    } finally {
      setSaving(false);
    }
  };

  const handleUpdateStrategy = async (strategy: 'REDACT' | 'BLOCK') => {
    try {
      setSaving(true);
      await updateDlp({
        variables: { input: { projectId: currentProjectId, strategy } },
      });
      toast.success('Strategy updated');
    } catch (e: any) {
      toast.error(e.message || 'Failed to update strategy');
    } finally {
      setSaving(false);
    }
  };

  const handleToggleMask = async (field: string, value: boolean) => {
    try {
      await updateDlp({
        variables: { input: { projectId: currentProjectId, [field]: value } },
      });
    } catch (e: any) {
      toast.error(e.message || 'Failed to update rule');
    }
  };

  const handleAddCustomRegex = async () => {
    if (!customRegexInput.trim()) return;
    try {
      const newArray = [...(config?.customRegex || []), customRegexInput.trim()];
      await updateDlp({
        variables: { input: { projectId: currentProjectId, customRegex: newArray } },
      });
      setCustomRegexInput('');
      toast.success('Custom pattern added');
    } catch (e: any) {
      toast.error(e.message || 'Failed to add custom rule');
    }
  };

  const handleRemoveCustomRegex = async (index: number) => {
    const newArray = [...(config?.customRegex || [])];
    newArray.splice(index, 1);
    try {
      await updateDlp({
        variables: { input: { projectId: currentProjectId, customRegex: newArray } },
      });
    } catch (e: any) {
      toast.error(e.message || 'Failed to remove custom rule');
    }
  };

  const handleRunSandbox = async () => {
    if (!testInput.trim()) return;
    try {
      const { data } = await testDlp({
        variables: { projectId: currentProjectId, input: testInput },
      });
      setTestResult(data?.testDlpRedaction);
    } catch (e: any) {
      toast.error(e.message || 'Sandbox test failed');
    }
  };

  const handlePublishToAllProjects = async () => {
    if (!config) return;
    try {
      setSaving(true);
      const promises = projects
        .filter(p => p.id !== currentProjectId)
        .map(p =>
          updateDlp({
            variables: {
              input: {
                projectId: p.id,
                isEnabled: config.isEnabled,
                strategy: config.strategy,
                maskEmails: config.maskEmails,
                maskPhones: config.maskPhones,
                maskCreditCards: config.maskCreditCards,
                maskSsn: config.maskSsn,
                maskApiKeys: config.maskApiKeys,
                customRegex: config.customRegex || [],
              },
            },
          })
        );
      await Promise.all(promises);
      toast.success(`Policy published to ${promises.length} other project(s)`);
    } catch (e: any) {
      toast.error(e.message || 'Failed to publish policy');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--theme-color-primary)]" />
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-[var(--theme-text-primary)] border-none m-0 p-0">
            Data Privacy (DLP)
          </h1>
          <p className="text-sm text-[var(--theme-text-tertiary)] mt-1 max-w-xl">
            Automatically detect and mask sensitive information in prompt payloads before they reach the LLM provider. 
            Protect PII, financial data, and secrets in real time.
          </p>
        </div>
        
        <div className="flex items-center gap-2">
          <select
            title="Organization"
            value={selectedOrgId}
            onChange={(e) => setSelectedOrgId(e.target.value)}
            className="block w-40 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm px-3 py-2 text-sm focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)]"
          >
            <option value="" disabled>Select Org</option>
            {orgs.map((o) => (
              <option key={o.id} value={o.id}>{o.name}</option>
            ))}
          </select>
          <select
            title="Project"
            value={currentProjectId}
            onChange={(e) => setCurrentProjectId(e.target.value)}
            className="block w-40 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm px-3 py-2 text-sm focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)]"
          >
            <option value="" disabled>Select Project</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        </div>
      </div>

      {!currentProjectId ? (
        <div className="p-12 text-center text-[var(--theme-text-tertiary)] card">
          <ShieldCheckIcon className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p>Please select a project to configure DLP policies.</p>
        </div>
      ) : (
      <>
        {/* ── Global Toggle ─────────────────────────────────── */}
        <div className="card p-6 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className={`p-3 rounded-2xl flex items-center justify-center transition-colors ${isEnabled ? 'bg-green-500/15 text-green-500' : 'bg-[var(--theme-bg-subtle)] text-[var(--theme-text-tertiary)]'}`}>
              <ShieldCheckIcon className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-lg font-semibold text-[var(--theme-text-primary)]">Protection Status</h3>
              <p className="text-sm text-[var(--theme-text-secondary)]">
                {isEnabled 
                  ? <>Active — All API requests are being scanned. Currently using <strong className="text-[var(--theme-text-primary)]">{activePresetId === 'none' ? 'Disabled' : activePresetId.charAt(0).toUpperCase() + activePresetId.slice(1)}</strong> policy.</>
                  : 'Disabled — No PII scanning is active for this project.'
                }
              </p>
            </div>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" className="sr-only peer" checked={isEnabled} onChange={(e) => handleToggleEnable(e.target.checked)} disabled={saving} />
            <div className="w-11 h-6 bg-[var(--theme-bg-subtle)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-green-500"></div>
          </label>
        </div>

        {/* ── Policy Presets ────────────────────────────────── */}
        <div>
          <h2 className="text-lg font-semibold text-[var(--theme-text-primary)] mb-1">Quick Presets</h2>
          <p className="text-sm text-[var(--theme-text-tertiary)] mb-4">Select a preset to quickly configure your DLP policy, or choose "Custom" for full control.</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {POLICY_PRESETS.map(preset => {
              const isActive = activePresetId === preset.id;
              return (
                <button
                  key={preset.id}
                  onClick={() => applyPreset(preset)}
                  disabled={saving}
                  className={`relative flex flex-col items-start p-5 rounded-2xl border-2 text-left transition-all duration-200 hover:shadow-md disabled:opacity-50 ${
                    isActive
                      ? `${preset.borderColor} ${preset.bgColor} shadow-sm`
                      : 'border-[var(--theme-border-default)] hover:border-[var(--theme-border-hover)] bg-[var(--theme-bg-surface)]'
                  }`}
                >
                  {isActive && (
                    <CheckCircleIcon className={`absolute top-3 right-3 w-5 h-5 ${preset.color}`} />
                  )}
                  <div className={`p-2.5 rounded-xl mb-3 ${isActive ? preset.bgColor : 'bg-[var(--theme-bg-subtle)]'}`}>
                    <span className={isActive ? preset.color : 'text-[var(--theme-text-tertiary)]'}>{preset.icon}</span>
                  </div>
                  <span className={`text-base font-semibold mb-1 ${isActive ? preset.color : 'text-[var(--theme-text-primary)]'}`}>
                    {preset.name}
                  </span>
                  <p className="text-xs text-[var(--theme-text-tertiary)] leading-relaxed">
                    {preset.description}
                  </p>
                </button>
              );
            })}
          </div>
        </div>

        {/* ── Main Content Grid ─────────────────────────────── */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          
          {/* Left Column: Settings */}
          <div className="lg:col-span-2 space-y-6">
            
            {/* Strategy Selection */}
            <div className={`card p-6 transition-opacity ${!isEnabled ? 'opacity-40 pointer-events-none' : ''}`}>
              <h3 className="text-base font-semibold text-[var(--theme-text-primary)] mb-1">Interception Strategy</h3>
              <p className="text-sm text-[var(--theme-text-tertiary)] mb-4">Choose how the system responds when PII is detected in a request.</p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <button
                  onClick={() => handleUpdateStrategy('REDACT')}
                  className={`flex items-start gap-3 p-4 rounded-xl border-2 text-left transition-all ${
                    config?.strategy === 'REDACT'
                      ? 'border-green-500 bg-green-500/10'
                      : 'border-[var(--theme-border-default)] hover:border-[var(--theme-border-hover)]'
                  }`}
                >
                  <ShieldCheckIcon className={`w-5 h-5 mt-0.5 flex-shrink-0 ${config?.strategy === 'REDACT' ? 'text-green-500' : 'text-[var(--theme-text-tertiary)]'}`} />
                  <div>
                    <span className="font-semibold text-[var(--theme-text-primary)] block">Scrub & Redact</span>
                    <span className="text-xs text-[var(--theme-text-tertiary)]">Replace sensitive data with *** and forward the request.</span>
                  </div>
                </button>
                <button
                  onClick={() => handleUpdateStrategy('BLOCK')}
                  className={`flex items-start gap-3 p-4 rounded-xl border-2 text-left transition-all ${
                    config?.strategy === 'BLOCK'
                      ? 'border-red-500 bg-red-500/10'
                      : 'border-[var(--theme-border-default)] hover:border-[var(--theme-border-hover)]'
                  }`}
                >
                  <HandRaisedIcon className={`w-5 h-5 mt-0.5 flex-shrink-0 ${config?.strategy === 'BLOCK' ? 'text-red-500' : 'text-[var(--theme-text-tertiary)]'}`} />
                  <div>
                    <span className="font-semibold text-[var(--theme-text-primary)] block">Hard Block</span>
                    <span className="text-xs text-[var(--theme-text-tertiary)]">Reject the entire request with HTTP 400 if PII is found.</span>
                  </div>
                </button>
              </div>
            </div>

            {/* PII Rules Configuration */}
            <div className={`card overflow-hidden transition-opacity ${!isEnabled ? 'opacity-40 pointer-events-none' : ''}`}>
              <div className="p-5 border-b border-[var(--theme-border-default)]">
                <h3 className="text-base font-semibold text-[var(--theme-text-primary)]">Detection Rules</h3>
                <p className="text-sm text-[var(--theme-text-tertiary)] mt-0.5">Toggle which PII patterns should be scanned in every request.</p>
              </div>
              
              <ul className="divide-y divide-[var(--theme-border-default)]">
                {PII_RULES.map(rule => (
                  <li key={rule.field} className="px-5 py-4 flex items-center justify-between hover:bg-[var(--theme-bg-subtle)] transition-colors">
                    <div className="flex items-center gap-3">
                      <rule.icon className="w-5 h-5" style={{ color: 'var(--theme-text-secondary)' }} />
                      <div>
                        <span className="block text-sm font-medium text-[var(--theme-text-primary)]">{rule.label}</span>
                        <span className="block text-xs text-[var(--theme-text-tertiary)]">{rule.desc}</span>
                      </div>
                    </div>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input type="checkbox" className="sr-only peer" checked={(config as any)?.[rule.field] || false} onChange={(e) => handleToggleMask(rule.field, e.target.checked)} />
                      <div className="w-10 h-[22px] bg-[var(--theme-bg-subtle)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-[18px] after:w-[18px] after:transition-all peer-checked:bg-green-500"></div>
                    </label>
                  </li>
                ))}
              </ul>

              {/* Custom RegEx */}
              <div className="p-5 bg-[var(--theme-bg-subtle)] border-t border-[var(--theme-border-default)]">
                <label className="block text-sm font-medium text-[var(--theme-text-secondary)] mb-2">Custom RegEx Patterns</label>
                <div className="flex gap-2 mb-3">
                  <input 
                    type="text" 
                    value={customRegexInput}
                    onChange={(e) => setCustomRegexInput(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleAddCustomRegex()}
                    placeholder="e.g. \b(internal_proj_\w+)\b"
                    className="flex-1 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm sm:text-sm p-2.5 focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)]"
                  />
                  <button onClick={handleAddCustomRegex} className="px-4 py-2 bg-[var(--theme-bg-surface)] hover:bg-[var(--theme-bg-hover)] text-[var(--theme-text-secondary)] rounded-xl text-sm font-medium transition-colors border border-[var(--theme-border-default)]">
                    Add
                  </button>
                </div>
                <div className="space-y-2">
                  {config?.customRegex?.map((regex: string, i: number) => (
                    <div key={i} className="flex items-center justify-between bg-[var(--theme-bg-surface)] border border-[var(--theme-border-default)] px-3 py-2 rounded-lg text-sm font-mono text-[var(--theme-text-secondary)]">
                      <span className="truncate">{regex}</span>
                      <button onClick={() => handleRemoveCustomRegex(i)} className="text-[var(--theme-text-tertiary)] hover:text-red-500 ml-2 flex-shrink-0">
                        <XMarkIcon className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                  {!config?.customRegex?.length && (
                    <p className="text-xs text-[var(--theme-text-tertiary)]">No custom patterns applied. Press Enter or click Add to create one.</p>
                  )}
                </div>
              </div>
            </div>

            {/* Admin: Publish Policy */}
            {isAdmin && projects.length > 1 && (
              <div className="card p-5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-xl bg-purple-500/10">
                      <SparklesIcon className="w-5 h-5 text-purple-500" />
                    </div>
                    <div>
                      <h3 className="text-sm font-semibold text-[var(--theme-text-primary)]">Publish Policy to All Projects</h3>
                      <p className="text-xs text-[var(--theme-text-tertiary)]">
                        Copy the current DLP configuration to {projects.length - 1} other project(s) in this organization.
                      </p>
                    </div>
                  </div>
                  <button
                    onClick={handlePublishToAllProjects}
                    disabled={saving}
                    className="px-4 py-2 bg-purple-500 hover:bg-purple-600 text-white rounded-xl text-sm font-medium shadow-sm transition-colors disabled:opacity-50"
                  >
                    {saving ? 'Publishing...' : 'Publish'}
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Right Column: Simulator */}
          <div className="lg:col-span-1">
            <div className="card sticky top-6 overflow-hidden">
              <div className="p-5 border-b border-[var(--theme-border-default)] bg-[var(--theme-bg-subtle)]">
                <h3 className="text-base font-semibold text-[var(--theme-text-primary)] flex items-center gap-2">
                  <BeakerIcon className="w-5 h-5 inline-block mr-1" style={{ color: 'var(--theme-text-secondary)' }} /> Simulator
                </h3>
                <p className="text-xs text-[var(--theme-text-tertiary)] mt-0.5">Test your rules against sample inputs in real time.</p>
              </div>
              <div className="p-5 space-y-4">
                <div>
                  <textarea 
                    value={testInput}
                    onChange={(e) => setTestInput(e.target.value)}
                    placeholder={'Paste or type text to test...\n\ne.g. "My email is john@acme.com and my SSN is 123-45-6789"'}
                    className="w-full h-32 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm text-sm p-3 focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)] resize-none placeholder:text-[var(--theme-text-tertiary)]"
                  />
                </div>
                <button 
                  onClick={handleRunSandbox}
                  disabled={testing || !testInput.trim() || !isEnabled}
                  className="w-full py-2.5 bg-[var(--theme-text-primary)] text-[var(--theme-bg-surface)] rounded-xl text-sm font-medium shadow-sm transition-all hover:opacity-90 disabled:opacity-40"
                >
                  {testing ? 'Scanning...' : 'Run Simulation'}
                </button>

                {testResult && (
                  <div className="pt-4 border-t border-[var(--theme-border-default)] space-y-3">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-[var(--theme-text-primary)]">Result:</span>
                      {testResult.hasPii ? (
                        <span className="px-2.5 py-0.5 rounded-full bg-red-500/15 text-red-500 text-xs font-semibold">PII Detected</span>
                      ) : (
                        <span className="px-2.5 py-0.5 rounded-full bg-green-500/15 text-green-500 text-xs font-semibold">Clean</span>
                      )}
                      {testResult.hasPii && testResult.blocked && (
                        <span className="px-2.5 py-0.5 rounded-full bg-red-500 text-white text-xs font-semibold">Blocked</span>
                      )}
                    </div>
                    
                    <div className="text-sm font-mono whitespace-pre-wrap bg-[var(--theme-bg-subtle)] border border-[var(--theme-border-default)] rounded-xl p-3 text-[var(--theme-text-secondary)] min-h-[4rem] leading-relaxed">
                      {testResult.scrubbedText}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>

        </div>
      </>
      )}
    </div>
  );
}
