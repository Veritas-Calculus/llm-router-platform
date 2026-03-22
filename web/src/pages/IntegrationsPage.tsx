import { useState, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { GET_INTEGRATIONS, UPDATE_INTEGRATION } from '@/lib/graphql/operations/integrations';
import { CloudIcon, CheckCircleIcon, XCircleIcon } from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';

export default function IntegrationsPage() {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const { data, loading, refetch } = useQuery<any>(GET_INTEGRATIONS, {
    fetchPolicy: 'cache-and-network',
  });
  
  const [updateIntegration] = useMutation(UPDATE_INTEGRATION);

  const integrations = useMemo(() => data?.integrations || [], [data]);

  const handleUpdate = async (name: string, enabled: boolean, configStr: string) => {
    try {
      // Validate JSON before sending payload
      JSON.parse(configStr);
      
      await updateIntegration({
        variables: {
          name,
          input: {
            enabled,
            config: configStr,
          }
        }
      });
      toast.success(`${name} configuration saved successfully`);
      refetch();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      toast.error(`Failed to update ${name}: ${err.message || 'Invalid JSON'}`);
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
          <IntegrationCard 
            key={integration.id} 
            integration={integration} 
            onSave={handleUpdate} 
          />
        ))}
      </div>
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function IntegrationCard({ integration, onSave }: { integration: any, onSave: (name: string, enabled: boolean, config: string) => void }) {
  const [enabled, setEnabled] = useState(integration.enabled);
  const [configStr, setConfigStr] = useState(integration.config === '{}' ? '{\n  \n}' : JSON.stringify(JSON.parse(integration.config), null, 2));

  // Pre-fill templates based on integration name if empty
  const getTemplate = (name: string) => {
    if (name === 'sentry') return '{\n  "dsn": "https://example@sentry.io/123"\n}';
    if (name === 'loki') return '{\n  "endpoint": "http://loki:3100/loki/api/v1/push"\n}';
    if (name === 'langfuse') return '{\n  "publicKey": "",\n  "secretKey": "",\n  "baseUrl": ""\n}';
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
