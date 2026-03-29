/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useEffect } from 'react';
import clsx from 'clsx';
import { useQuery as useApolloQuery, useMutation as useApolloMutation } from '@apollo/client/react';
import { useQuery, useMutation } from '@apollo/client/react';
import { FEATURE_GATES_QUERY, UPDATE_FEATURE_GATE } from '@/lib/graphql/operations/featuregates';
import { GET_INTEGRATIONS, UPDATE_INTEGRATION, TEST_LANGFUSE_CONNECTION } from '@/lib/graphql/operations/integrations';
import {
  CheckCircleIcon, XCircleIcon, CloudIcon, SignalIcon, EyeIcon, EyeSlashIcon,
} from '@heroicons/react/24/outline';
import { FormField, TextInput, Toggle } from './FormPrimitives';
import toast from 'react-hot-toast';

/* ── Feature Gates ── */

interface FeatureGateItem {
  name: string;
  enabled: boolean;
  category: string;
  description: string;
  source: string;
}

const categoryOrder = ['security', 'feature', 'observability'];
const categoryLabels: Record<string, string> = {
  security: 'Security',
  feature: 'Features',
  observability: 'Observability',
};

export function FeatureGatesSettingsTab() {
  const { data, loading, refetch } = useApolloQuery<{ featureGates: FeatureGateItem[] }>(FEATURE_GATES_QUERY, {
    fetchPolicy: 'network-only',
  });
  const [updateGate] = useApolloMutation(UPDATE_FEATURE_GATE);

  const gates = data?.featureGates || [];

  const grouped = categoryOrder.reduce<Record<string, FeatureGateItem[]>>((acc, cat) => {
    acc[cat] = gates.filter((g) => g.category === cat);
    return acc;
  }, {});

  const handleToggle = async (name: string, currentValue: boolean) => {
    try {
      await updateGate({ variables: { name, enabled: !currentValue } });
      toast.success(`${name} ${!currentValue ? 'enabled' : 'disabled'}`);
      refetch();
    } catch (err: any) {
      toast.error(err?.message || 'Failed to update gate');
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <p className="text-sm text-apple-gray-500">
        Toggle platform capabilities at runtime. Changes take effect immediately.
      </p>
      {categoryOrder.map((cat) => {
        const items = grouped[cat];
        if (!items || items.length === 0) return null;
        return (
          <div key={cat} className="space-y-3">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-apple-gray-400">
              {categoryLabels[cat]}
            </h3>
            <div className="border border-apple-gray-200 rounded-xl divide-y divide-apple-gray-100">
              {items.map((gate) => (
                <div key={gate.name} className="flex items-center justify-between px-5 py-3.5">
                  <div className="flex-1 min-w-0 pr-4">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-apple-gray-900">{gate.name}</span>
                      {gate.source === 'database' && (
                        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-50 text-blue-600 border border-blue-200">DB</span>
                      )}
                    </div>
                    <p className="text-xs text-apple-gray-500 mt-0.5 truncate">{gate.description}</p>
                  </div>
                  <button type="button" role="switch" aria-checked={gate.enabled} onClick={() => handleToggle(gate.name, gate.enabled)}
                    className={clsx('relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200 flex-shrink-0 cursor-pointer',
                      gate.enabled ? 'bg-apple-blue' : 'bg-apple-gray-300')}>
                    <span className={clsx('inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform duration-200',
                      gate.enabled ? 'translate-x-6' : 'translate-x-1')} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/* ── Integrations ── */

export function IntegrationsSettingsTab() {
  const { data, loading, refetch } = useQuery<any>(GET_INTEGRATIONS, { fetchPolicy: 'cache-and-network' });
  const [updateIntegration] = useMutation(UPDATE_INTEGRATION);
  const [testLangfuse] = useMutation(TEST_LANGFUSE_CONNECTION);

  const integrations = (data?.integrations || []) as any[];

  const handleSave = async (name: string, enabled: boolean, configStr: string) => {
    try {
      JSON.parse(configStr);
      await updateIntegration({ variables: { name, input: { enabled, config: configStr } } });
      toast.success(`${name} configuration saved`);
      refetch();
    } catch (err: any) {
      toast.error(`Failed: ${err.message || 'Invalid JSON'}`);
    }
  };

  const handleTestLangfuse = async (publicKey: string, secretKey: string, host: string) => {
    try {
      const result: any = await testLangfuse({ variables: { publicKey, secretKey, host } });
      return result?.data?.testLangfuseConnection === true;
    } catch {
      return false;
    }
  };

  if (loading) return <div className="flex justify-center py-12"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" /></div>;

  return (
    <div className="space-y-6">
      <p className="text-sm text-apple-gray-500">Configure external logging, tracing, and metrics platforms.</p>
      {integrations.map((ig: any) => (
        ig.name === 'langfuse' ? (
          <LangfuseInlineCard key={ig.id} integration={ig} onSave={handleSave} onTestConnection={handleTestLangfuse} />
        ) : (
          <IntegrationInlineCard key={ig.id} integration={ig} onSave={handleSave} />
        )
      ))}
      {integrations.length === 0 && (
        <p className="text-sm text-apple-gray-400 text-center py-8">No integrations configured yet.</p>
      )}
    </div>
  );
}

/* ── Langfuse Card ── */

function LangfuseInlineCard({ integration, onSave, onTestConnection }: {
  integration: any;
  onSave: (name: string, enabled: boolean, config: string) => void;
  onTestConnection: (publicKey: string, secretKey: string, host: string) => Promise<boolean>;
}) {
  const [enabled, setEnabled] = useState(integration.enabled);
  const [publicKey, setPublicKey] = useState('');
  const [secretKey, setSecretKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('https://cloud.langfuse.com');
  const [showSecret, setShowSecret] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<'idle' | 'success' | 'failed'>('idle');

  useEffect(() => {
    try {
      const cfg = JSON.parse(integration.config);
      if (cfg.publicKey) setPublicKey(cfg.publicKey);
      if (cfg.secretKey) setSecretKey(cfg.secretKey);
      if (cfg.baseUrl) setBaseUrl(cfg.baseUrl);
    } catch { /* ignore */ }
  }, [integration.config]);

  const handleSave = () => {
    const config = JSON.stringify({ publicKey, secretKey, baseUrl });
    onSave('langfuse', enabled, config);
  };

  const handleTest = async () => {
    if (!publicKey || !secretKey || !baseUrl) {
      toast.error('Please fill in all Langfuse fields before testing');
      return;
    }
    setTesting(true);
    setTestResult('idle');
    const ok = await onTestConnection(publicKey, secretKey, baseUrl);
    setTestResult(ok ? 'success' : 'failed');
    if (ok) { toast.success('Langfuse connection successful'); }
    else { toast.error('Langfuse connection failed. Check your credentials and host.'); }
    setTesting(false);
  };

  return (
    <div className="border border-apple-gray-200 rounded-xl p-5 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <SignalIcon className="w-5 h-5 text-orange-500" />
          <h4 className="text-sm font-semibold text-apple-gray-900">Langfuse</h4>
          {enabled ? (
            <span className="flex items-center text-xs font-medium text-emerald-600"><CheckCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Active</span>
          ) : (
            <span className="flex items-center text-xs font-medium text-apple-gray-400"><XCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Off</span>
          )}
        </div>
        <Toggle checked={enabled} onChange={setEnabled} label="" />
      </div>

      <p className="text-xs text-apple-gray-500">
        LLM observability and analytics. Traces, generations, and token usage are automatically reported.
      </p>

      <div className="space-y-3">
        <FormField label="Public Key">
          <TextInput value={publicKey} onChange={setPublicKey} placeholder="pk-lf-..." />
        </FormField>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-apple-gray-700">Secret Key</label>
          <div className="relative">
            <input
              type={showSecret ? 'text' : 'password'}
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
              placeholder="sk-lf-..."
              className="w-full px-3.5 py-2.5 pr-10 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all"
            />
            <button type="button" onClick={() => setShowSecret(!showSecret)}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-apple-gray-400 hover:text-apple-gray-600">
              {showSecret ? <EyeSlashIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
            </button>
          </div>
        </div>

        <FormField label="Base URL">
          <TextInput value={baseUrl} onChange={setBaseUrl} placeholder="https://cloud.langfuse.com" />
        </FormField>
      </div>

      <button onClick={handleTest} disabled={testing}
        className={clsx('w-full py-2.5 rounded-xl text-sm font-medium transition-all',
          testResult === 'success' ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
            : testResult === 'failed' ? 'bg-red-50 text-red-700 border border-red-200'
            : 'bg-apple-gray-50 text-apple-gray-700 border border-apple-gray-200 hover:bg-apple-gray-100',
          testing && 'opacity-60 cursor-wait')}>
        {testing ? (
          <span className="flex items-center justify-center">
            <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />Testing...
          </span>
        ) : testResult === 'success' ? (
          <span className="flex items-center justify-center"><CheckCircleIcon className="w-4 h-4 mr-1.5" /> Connection Successful</span>
        ) : testResult === 'failed' ? (
          <span className="flex items-center justify-center"><XCircleIcon className="w-4 h-4 mr-1.5" /> Connection Failed — Retry</span>
        ) : ('Test Connection')}
      </button>

      <div className="flex justify-end">
        <button onClick={handleSave}
          className="px-4 py-2 bg-apple-blue text-white rounded-xl text-sm font-semibold hover:bg-blue-600 transition-all">Save</button>
      </div>
    </div>
  );
}

/* ── Generic Integration Card ── */

function IntegrationInlineCard({ integration, onSave }: {
  integration: any;
  onSave: (name: string, enabled: boolean, config: string) => void;
}) {
  const [enabled, setEnabled] = useState(integration.enabled);

  const getTemplate = (name: string) => {
    if (name === 'sentry') return '{\n  "dsn": "https://example@sentry.io/123"\n}';
    if (name === 'loki') return '{\n  "endpoint": "http://loki:3100/loki/api/v1/push"\n}';
    return '{\n  \n}';
  };

  const [configStr, setConfigStr] = useState(
    integration.config === '{}' ? getTemplate(integration.name) : JSON.stringify(JSON.parse(integration.config), null, 2)
  );

  return (
    <div className="border border-apple-gray-200 rounded-xl p-5 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <CloudIcon className="w-5 h-5 text-apple-gray-500" />
          <h4 className="text-sm font-semibold text-apple-gray-900 capitalize">{integration.name}</h4>
          {enabled ? (
            <span className="flex items-center text-xs font-medium text-emerald-600"><CheckCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Active</span>
          ) : (
            <span className="flex items-center text-xs font-medium text-apple-gray-400"><XCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Off</span>
          )}
        </div>
        <Toggle checked={enabled} onChange={setEnabled} label="" />
      </div>
      <textarea value={configStr} onChange={(e) => setConfigStr(e.target.value)}
        className="w-full h-28 p-3 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm font-mono text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue resize-none"
        placeholder={getTemplate(integration.name)} />
      <div className="flex justify-end">
        <button onClick={() => onSave(integration.name, enabled, configStr)}
          className="px-4 py-2 bg-apple-blue text-white rounded-xl text-sm font-semibold hover:bg-blue-600 transition-all">Save</button>
      </div>
    </div>
  );
}

export default FeatureGatesSettingsTab;
