import { useState } from 'react';
import { CheckCircleIcon } from '@heroicons/react/24/outline';
import { Provider } from '@/lib/api';

interface LocalProviderCardProps {
  provider: Provider;
  onToggleRequiresApiKey: () => void;
  onSaveEndpoint: (url: string) => Promise<void>;
}

export default function LocalProviderCard({
  provider,
  onToggleRequiresApiKey,
  onSaveEndpoint,
}: LocalProviderCardProps) {
  const [editingEndpoint, setEditingEndpoint] = useState(false);
  const [endpointValue, setEndpointValue] = useState(provider.base_url);
  const [savingEndpoint, setSavingEndpoint] = useState(false);

  const handleSave = async () => {
    if (!endpointValue.trim()) return;
    setSavingEndpoint(true);
    try {
      await onSaveEndpoint(endpointValue.trim());
      setEditingEndpoint(false);
    } finally {
      setSavingEndpoint(false);
    }
  };

  const handleCancel = () => {
    setEditingEndpoint(false);
    setEndpointValue(provider.base_url);
  };

  const displayName = provider.name === 'ollama' ? 'Ollama' : provider.name === 'vllm' ? 'vLLM' : 'LM Studio';
  const placeholder = provider.name === 'ollama' ? 'http://localhost:11434' : provider.name === 'vllm' ? 'http://localhost:8000/v1' : 'http://localhost:1234/v1';

  return (
    <div className="card mt-6">
      <div className="flex items-center gap-3 mb-4">
        <div className="flex-shrink-0 w-10 h-10 bg-apple-blue/10 rounded-full flex items-center justify-center">
          <CheckCircleIcon className="w-6 h-6 text-apple-blue" />
        </div>
        <div>
          <h3 className="text-lg font-semibold text-apple-gray-900">Local Provider</h3>
          <p className="text-sm text-apple-gray-500">
            {displayName} runs locally. You can configure it to require an API key.
          </p>
        </div>
      </div>

      {/* Requires API Key Toggle */}
      <div className="mt-4 pt-4 border-t border-apple-gray-100">
        <div className="flex items-center justify-between mb-2">
          <div>
            <h4 className="text-sm font-medium text-apple-gray-900">Require API Key</h4>
            <p className="text-xs text-apple-gray-500 mt-0.5">
              Require API keys to route requests to this provider
            </p>
          </div>
          <button
            onClick={onToggleRequiresApiKey}
            className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${provider.requires_api_key ? 'bg-apple-blue' : 'bg-apple-gray-200'
              }`}
          >
            <span
              className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${provider.requires_api_key ? 'translate-x-5' : 'translate-x-0'
                }`}
            />
          </button>
        </div>
      </div>

      {/* Endpoint Configuration */}
      <div className="border-t border-apple-gray-100 pt-4 mt-2">
        <div className="flex items-center justify-between mb-2">
          <label className="text-sm font-medium text-apple-gray-900">
            Endpoint URL
          </label>
          {!editingEndpoint && (
            <button
              onClick={() => setEditingEndpoint(true)}
              className="text-sm text-apple-blue hover:text-blue-600 transition-colors"
            >
              Edit
            </button>
          )}
        </div>
        {editingEndpoint ? (
          <div className="space-y-3">
            <input
              type="text"
              value={endpointValue}
              onChange={(e) => setEndpointValue(e.target.value)}
              className="input w-full"
              placeholder={placeholder}
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={handleCancel}
                className="btn btn-secondary text-sm"
                disabled={savingEndpoint}
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                className="btn btn-primary text-sm"
                disabled={savingEndpoint}
              >
                {savingEndpoint ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        ) : (
          <p className="text-sm text-apple-gray-600 font-mono bg-apple-gray-50 px-3 py-2 rounded-apple">
            {provider.base_url}
          </p>
        )}
      </div>
    </div>
  );
}
