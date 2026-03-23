import { useState, useMemo, useEffect } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { GET_INTEGRATIONS, UPDATE_INTEGRATION, TEST_LANGFUSE_CONNECTION } from '@/lib/graphql/operations/integrations';
import { CloudIcon, CheckCircleIcon, XCircleIcon, SignalIcon, EyeIcon, EyeSlashIcon } from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { useTranslation } from '@/lib/i18n';

export default function IntegrationsPage() {
  const { t } = useTranslation();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const { data, loading, refetch } = useQuery<any>(GET_INTEGRATIONS, {
    fetchPolicy: 'cache-and-network',
  });
  
  const [updateIntegration] = useMutation(UPDATE_INTEGRATION);
  const [testLangfuse] = useMutation(TEST_LANGFUSE_CONNECTION);

  const integrations = useMemo(() => data?.integrations || [], [data]);

  const handleUpdate = async (name: string, enabled: boolean, configStr: string) => {
    try {
      JSON.parse(configStr);
      await updateIntegration({
        variables: { name, input: { enabled, config: configStr } }
      });
      toast.success(`${name} configuration saved successfully`);
      refetch();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      toast.error(`Failed to update ${name}: ${err.message || 'Invalid JSON'}`);
    }
  };

  const handleTestLangfuse = async (publicKey: string, secretKey: string, host: string) => {
    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const result: any = await testLangfuse({ variables: { publicKey, secretKey, host } });
      return result?.data?.testLangfuseConnection === true;
    } catch {
      return false;
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Integrations</h1>
        <p className="text-apple-gray-500 mt-1">Configure external logging, tracing, and metric endpoints</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
        {integrations.map((integration: any) => (
          integration.name === 'langfuse' ? (
            <LangfuseCard
              key={integration.id}
              integration={integration}
              onSave={handleUpdate}
              onTestConnection={handleTestLangfuse}
            />
          ) : (
            <IntegrationCard
              key={integration.id}
              integration={integration}
              onSave={handleUpdate}
            />
          )
        ))}
      </div>
    </div>
  );
}

// ── Langfuse Card ────────────────────────────────────────────────────────────

function LangfuseCard({ integration, onSave, onTestConnection }: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
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
    } catch {
      // Ignore
    }
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
    if (ok) {
      toast.success('Langfuse connection successful');
    } else {
      toast.error('Langfuse connection failed. Check your credentials and host.');
    }
    setTesting(false);
  };

  return (
    <div className="card flex flex-col h-full border border-apple-gray-200">
      <div className="p-6 flex-1 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-orange-50 to-amber-50 flex items-center justify-center border border-orange-100 shadow-sm">
              <SignalIcon className="w-6 h-6 text-orange-500" />
            </div>
            <div>
              <h3 className="text-lg font-medium text-apple-gray-900">Langfuse</h3>
              <div className="flex items-center mt-0.5">
                {enabled ? (
                  <span className="flex items-center text-xs font-medium text-emerald-600">
                    <CheckCircleIcon className="w-3.5 h-3.5 mr-1" /> Active
                  </span>
                ) : (
                  <span className="flex items-center text-xs font-medium text-apple-gray-400">
                    <XCircleIcon className="w-3.5 h-3.5 mr-1" /> Disabled
                  </span>
                )}
              </div>
            </div>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" className="sr-only peer" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-apple-blue"></div>
          </label>
        </div>

        <p className="text-sm text-apple-gray-500 mb-4">
          LLM observability and analytics. Traces, generations, and token usage are automatically reported.
        </p>

        {/* Form Fields */}
        <div className="space-y-3 flex-1">
          <div>
            <label className="block text-sm font-medium text-apple-gray-700 mb-1">Public Key</label>
            <input
              type="text"
              value={publicKey}
              onChange={(e) => setPublicKey(e.target.value)}
              placeholder="pk-lf-..."
              className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm font-mono text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-apple-gray-700 mb-1">Secret Key</label>
            <div className="relative">
              <input
                type={showSecret ? 'text' : 'password'}
                value={secretKey}
                onChange={(e) => setSecretKey(e.target.value)}
                placeholder="sk-lf-..."
                className="w-full px-3 py-2 pr-10 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm font-mono text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue"
              />
              <button
                type="button"
                onClick={() => setShowSecret(!showSecret)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-apple-gray-400 hover:text-apple-gray-600"
              >
                {showSecret ? <EyeSlashIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
              </button>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-apple-gray-700 mb-1">Base URL</label>
            <input
              type="text"
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="https://cloud.langfuse.com"
              className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue"
            />
          </div>
        </div>

        {/* Test Connection */}
        <div className="mt-4">
          <button
            onClick={handleTest}
            disabled={testing}
            className={`w-full py-2 px-4 rounded-lg text-sm font-medium transition-all ${
              testResult === 'success'
                ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                : testResult === 'failed'
                ? 'bg-red-50 text-red-700 border border-red-200'
                : 'bg-apple-gray-50 text-apple-gray-700 border border-apple-gray-200 hover:bg-apple-gray-100'
            } ${testing ? 'opacity-60 cursor-wait' : ''}`}
          >
            {testing ? (
              <span className="flex items-center justify-center">
                <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />
                Testing...
              </span>
            ) : testResult === 'success' ? (
              <span className="flex items-center justify-center">
                <CheckCircleIcon className="w-4 h-4 mr-1.5" /> Connection Successful
              </span>
            ) : testResult === 'failed' ? (
              <span className="flex items-center justify-center">
                <XCircleIcon className="w-4 h-4 mr-1.5" /> Connection Failed — Retry
              </span>
            ) : (
              'Test Connection'
            )}
          </button>
        </div>

        {/* Save */}
        <div className="mt-4 flex justify-end pt-4 border-t border-apple-gray-100">
          <button
            onClick={handleSave}
            className="btn btn-primary py-2 px-6 shadow-sm font-medium"
          >
            Save Changes
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Generic Integration Card ─────────────────────────────────────────────────

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function IntegrationCard({ integration, onSave }: { integration: any, onSave: (name: string, enabled: boolean, config: string) => void }) {
  const [enabled, setEnabled] = useState(integration.enabled);
  const [configStr, setConfigStr] = useState(integration.config === '{}' ? '{\n  \n}' : JSON.stringify(JSON.parse(integration.config), null, 2));

  const getTemplate = (name: string) => {
    if (name === 'sentry') return '{\n  "dsn": "https://example@sentry.io/123"\n}';
    if (name === 'loki') return '{\n  "endpoint": "http://loki:3100/loki/api/v1/push"\n}';
    return '{\n  \n}';
  };

  const handleReset = () => {
    setConfigStr(getTemplate(integration.name));
  };

  return (
    <div className="card flex flex-col h-full border border-apple-gray-200">
      <div className="p-6 flex-1 flex flex-col">
        <div className="flex items-center justify-between mx-b-4 mb-4">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-apple-gray-50 flex items-center justify-center border border-apple-gray-100 shadow-sm">
              <CloudIcon className="w-6 h-6 text-apple-gray-500" />
            </div>
            <div>
              <h3 className="text-lg font-medium text-apple-gray-900 capitalize">{integration.name}</h3>
              <div className="flex items-center mt-0.5">
                {enabled ? (
                  <span className="flex items-center text-xs font-medium text-emerald-600">
                    <CheckCircleIcon className="w-3.5 h-3.5 mr-1" /> Active
                  </span>
                ) : (
                  <span className="flex items-center text-xs font-medium text-apple-gray-400">
                    <XCircleIcon className="w-3.5 h-3.5 mr-1" /> Disabled
                  </span>
                )}
              </div>
            </div>
          </div>
          
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={enabled} 
              onChange={(e) => setEnabled(e.target.checked)} 
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-apple-blue"></div>
          </label>
        </div>

        <div className="flex-1 mt-4">
          <label className="block text-sm font-medium text-apple-gray-700 mb-2">Configuration (JSON)</label>
          <textarea
            value={configStr}
            onChange={(e) => setConfigStr(e.target.value)}
            className="w-full h-40 p-3 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm font-mono text-apple-gray-700 shadow-inner focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue resize-none"
            placeholder={getTemplate(integration.name)}
          />
        </div>

        <div className="mt-6 flex items-center justify-between pt-4 border-t border-apple-gray-100">
          <button 
            onClick={handleReset}
            className="text-sm font-medium text-apple-gray-500 hover:text-apple-gray-700 transition-colors"
          >
            Insert Template
          </button>
          <button
            onClick={() => onSave(integration.name, enabled, configStr)}
            className="btn btn-primary py-2 px-6 shadow-sm font-medium"
          >
            Save Changes
          </button>
        </div>
      </div>
    </div>
  );
}
