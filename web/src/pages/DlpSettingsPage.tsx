import { useState, useEffect, useMemo } from 'react';
import { useQuery, useMutation, useLazyQuery } from '@apollo/client/react';
import toast from 'react-hot-toast';
import { ShieldCheckIcon, HandRaisedIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { GET_DLP_CONFIG, UPDATE_DLP_CONFIG, TEST_DLP_REDACTION } from '@/lib/graphql/operations/dlp';
import { MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import type { Organization, Project } from '@/lib/types';

/* eslint-disable @typescript-eslint/no-explicit-any */

export default function DlpSettingsPage() {
  const [saving, setSaving] = useState(false);

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
  
  // Local state for sandbox
  const [testInput, setTestInput] = useState('');
  const [testResult, setTestResult] = useState<any>(null);

  // Queries
  const { data, loading, refetch } = useQuery<any>(GET_DLP_CONFIG, {
    variables: { projectId: currentProjectId },
    skip: !currentProjectId,
    fetchPolicy: 'network-only',
  });

  const [updateDlp] = useMutation<any>(UPDATE_DLP_CONFIG);
  const [testDlp, { loading: testing }] = useLazyQuery<any>(TEST_DLP_REDACTION, {
    fetchPolicy: 'network-only',
  });

  const config = data?.getDlpConfig || null;

  const [customRegexInput, setCustomRegexInput] = useState('');

  const handleToggleEnable = async (enabled: boolean) => {
    try {
      setSaving(true);
      await updateDlp({
        variables: { input: { projectId: currentProjectId, isEnabled: enabled } },
      });
      toast.success(enabled ? 'DLP Enabled' : 'DLP Disabled');
      refetch();
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
      refetch();
    } catch (e: any) {
      toast.error(e.message || 'Failed to update stringery');
    } finally {
      setSaving(false);
    }
  };

  const handleToggleMask = async (field: string, value: boolean) => {
    try {
      await updateDlp({
        variables: { input: { projectId: currentProjectId, [field]: value } },
      });
      refetch();
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
      refetch();
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
      refetch();
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

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  const isEnabled = config?.isEnabled || false;

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900 border-none m-0 p-0">Data Privacy (DLP)</h1>
          <p className="text-sm text-apple-gray-500 mt-1">
            Automatically detect and mask sensitive information in prompt payloads before they reach the provider.
          </p>
        </div>
        
        <div className="flex items-center gap-2">
            <select
              title="Organization"
              value={selectedOrgId}
              onChange={(e) => setSelectedOrgId(e.target.value)}
              className="block w-40 rounded-xl border-apple-gray-200 shadow-sm px-3 py-2 text-sm focus:ring-apple-blue focus:border-apple-blue text-apple-gray-900"
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
              className="block w-40 rounded-xl border-apple-gray-200 shadow-sm px-3 py-2 text-sm focus:ring-apple-blue focus:border-apple-blue text-apple-gray-900"
            >
              <option value="" disabled>Select Project</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
        </div>
      </div>

      {!currentProjectId ? (
        <div className="p-8 text-center text-apple-gray-500">Please select a project first.</div>
      ) : (
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        {/* Left Column: Settings */}
        <div className="lg:col-span-2 space-y-6">
          
          {/* Main Toggle Card */}
          <div className="bg-white rounded-[20px] shadow-sm border border-apple-gray-200 p-6 flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div className={`p-3 rounded-xl flex items-center justify-center ${isEnabled ? 'bg-green-100 text-green-600' : 'bg-apple-gray-100 text-apple-gray-500'}`}>
                <ShieldCheckIcon className="w-6 h-6" />
              </div>
              <div>
                <h3 className="text-lg font-medium text-apple-gray-900">Global Status</h3>
                <p className="text-sm text-apple-gray-500">Enable or disable DLP for your organization.</p>
              </div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" className="sr-only peer" checked={isEnabled} onChange={(e) => handleToggleEnable(e.target.checked)} disabled={saving} />
              <div className="w-11 h-6 bg-apple-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-apple-blue"></div>
            </label>
          </div>

          {/* Strategy Selection */}
          <div className={`bg-white rounded-[20px] shadow-sm border border-apple-gray-200 p-6 transition-opacity ${!isEnabled ? 'opacity-50 pointer-events-none' : ''}`}>
             <h3 className="text-lg font-medium text-apple-gray-900 mb-4">Interception Strategy</h3>
             <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <button
                  onClick={() => handleUpdateStrategy('REDACT')}
                  className={`flex flex-col items-start p-4 rounded-xl border text-left transition-colors ${config?.strategy === 'REDACT' ? 'border-apple-blue bg-apple-blue/5' : 'border-apple-gray-200 hover:border-apple-gray-300'}`}
                >
                  <div className="flex items-center gap-2 mb-2">
                    <ShieldCheckIcon className={`w-5 h-5 ${config?.strategy === 'REDACT' ? 'text-apple-blue' : 'text-apple-gray-500'}`} />
                    <span className="font-medium text-apple-gray-900">Scrub & Redact</span>
                  </div>
                  <p className="text-sm text-apple-gray-500">Mute sensitive entities (e.g. `user@email.com` to `***`) and forward the request.</p>
                </button>
                <button
                  onClick={() => handleUpdateStrategy('BLOCK')}
                  className={`flex flex-col items-start p-4 rounded-xl border text-left transition-colors ${config?.strategy === 'BLOCK' ? 'border-red-500 bg-red-50' : 'border-apple-gray-200 hover:border-apple-gray-300'}`}
                >
                  <div className="flex items-center gap-2 mb-2">
                    <HandRaisedIcon className={`w-5 h-5 ${config?.strategy === 'BLOCK' ? 'text-red-500' : 'text-apple-gray-500'}`} />
                    <span className="font-medium text-apple-gray-900">Hard Block</span>
                  </div>
                  <p className="text-sm text-apple-gray-500">Return HTTP 400 Bad Request immediately if PII is detected.</p>
                </button>
             </div>
          </div>

          {/* Rules Configuration */}
          <div className={`bg-white rounded-[20px] shadow-sm border border-apple-gray-200 overflow-hidden transition-opacity ${!isEnabled ? 'opacity-50 pointer-events-none' : ''}`}>
            <div className="p-6 border-b border-apple-gray-100">
               <h3 className="text-lg font-medium text-apple-gray-900">Active Rules</h3>
               <p className="text-sm text-apple-gray-500 mt-1">Select the PII patterns that should be monitored.</p>
            </div>
            
            <ul className="divide-y divide-apple-gray-100">
              {[
                { field: 'maskEmails', label: 'Email Addresses', desc: 'Detects standard email formats' },
                { field: 'maskPhones', label: 'Phone Numbers', desc: 'Detects US/International phone formats' },
                { field: 'maskCreditCards', label: 'Credit Cards', desc: 'Detects valid 16-digit PANs' },
                { field: 'maskSsn', label: 'Social Security Numbers', desc: 'Detects US SSN patterns' },
                { field: 'maskApiKeys', label: 'API Keys / Secrets', desc: 'Detects sk-..., Bearer tokens, etc.' },
              ].map(rule => (
                <li key={rule.field} className="p-4 flex items-center justify-between hover:bg-apple-gray-50/50">
                  <div>
                    <span className="block font-medium text-apple-gray-900">{rule.label}</span>
                    <span className="block text-xs text-apple-gray-500">{rule.desc}</span>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input type="checkbox" className="sr-only peer" checked={(config as any)?.[rule.field] || false} onChange={(e) => handleToggleMask(rule.field, e.target.checked)} />
                    <div className="w-9 h-5 bg-apple-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-apple-blue"></div>
                  </label>
                </li>
              ))}
            </ul>

            <div className="p-4 bg-apple-gray-50 border-t border-apple-gray-100">
              <label className="block text-sm font-medium text-apple-gray-700 mb-2">Custom RegEx Patterns</label>
              <div className="flex gap-2 mb-3">
                <input 
                  type="text" 
                  value={customRegexInput}
                  onChange={(e) => setCustomRegexInput(e.target.value)}
                  placeholder="e.g. \\b(internal_proj_\\w+)\\b"
                  className="flex-1 rounded-xl border-apple-gray-200 shadow-sm sm:text-sm p-2 border focus:ring-apple-blue focus:border-apple-blue"
                />
                <button onClick={handleAddCustomRegex} className="px-3 bg-apple-gray-100 hover:bg-apple-gray-200 text-apple-gray-700 rounded-xl text-sm font-medium transition-colors">
                  Add
                </button>
              </div>
              <div className="space-y-2">
                {config?.customRegex?.map((regex: string, i: number) => (
                  <div key={i} className="flex items-center justify-between bg-white border border-apple-gray-200 px-3 py-2 rounded-lg text-sm font-mono text-apple-gray-600">
                    <span className="truncate">{regex}</span>
                    <button onClick={() => handleRemoveCustomRegex(i)} className="text-apple-gray-400 hover:text-red-500">
                      <XMarkIcon className="w-5 h-5" />
                    </button>
                  </div>
                ))}
                {!config?.customRegex?.length && (
                  <p className="text-xs text-apple-gray-400">No custom patterns applied.</p>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Right Column: Sandbox */}
        <div className="lg:col-span-1">
          <div className="bg-white rounded-[20px] shadow-sm border border-apple-gray-200 sticky top-6">
            <div className="p-6 border-b border-apple-gray-100 bg-apple-gray-50/50 rounded-t-[20px]">
               <h3 className="text-lg font-medium text-apple-gray-900 flex items-center gap-2">
                 Simulator
               </h3>
               <p className="text-sm text-apple-gray-500">Test your rules against live inputs.</p>
            </div>
            <div className="p-6 space-y-4">
               <div>
                  <textarea 
                    value={testInput}
                    onChange={(e) => setTestInput(e.target.value)}
                    placeholder={'Type sensible info here to test...\n\ne.g. "My email is test@example.com."'}
                    className="w-full h-32 rounded-xl border-apple-gray-200 shadow-sm sm:text-sm p-3 border focus:ring-apple-blue focus:border-apple-blue resize-none"
                  />
               </div>
               <button 
                onClick={handleRunSandbox}
                disabled={testing || !testInput.trim()}
                className="w-full py-2.5 bg-apple-gray-900 hover:bg-apple-gray-800 text-white rounded-xl text-sm font-medium shadow-sm transition-colors disabled:opacity-50"
               >
                 {testing ? 'Testing...' : 'Run Simulation'}
               </button>

               {testResult && (
                 <div className="mt-6 pt-6 border-t border-apple-gray-100">
                    <div className="flex items-center gap-2 mb-3">
                      <span className="text-sm font-medium text-apple-gray-900">Result:</span>
                      {testResult.hasPii ? (
                        <span className="px-2 py-0.5 rounded-full bg-red-100 text-red-700 text-xs font-medium">PII Detected</span>
                      ) : (
                        <span className="px-2 py-0.5 rounded-full bg-green-100 text-green-700 text-xs font-medium">Clean</span>
                      )}
                      
                      {testResult.hasPii && testResult.blocked && (
                        <span className="px-2 py-0.5 rounded-full bg-red-500 text-white text-xs font-medium">Blocked</span>
                      )}
                    </div>
                    
                    <div className="relative">
                      <div className="text-sm font-mono whitespace-pre-wrap bg-apple-gray-50 border border-apple-gray-200 rounded-xl p-3 text-apple-gray-700 min-h-[4rem]">
                         {testResult.scrubbedText}
                      </div>
                    </div>
                 </div>
               )}
            </div>
          </div>
        </div>

      </div>
      )}
    </div>
  );
}
