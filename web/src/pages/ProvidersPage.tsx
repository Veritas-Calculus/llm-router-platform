import { motion } from 'framer-motion';
import ProviderList from '@/components/providers/ProviderList';
import ProviderInfoCard from '@/components/providers/ProviderInfoCard';
import ApiKeyTable from '@/components/providers/ApiKeyTable';
import LocalProviderCard from '@/components/providers/LocalProviderCard';
import { useProviders } from '@/hooks/useProviders';

function ProvidersPage() {
  const {
    providers,
    selectedProvider,
    setSelectedProvider,
    apiKeys,
    proxies,
    loading,
    testing,
    healthStatus,
    savingProxy,
    handleToggleProvider,
    handleTestConnection,
    handleToggleProxy,
    handleProxyChange,
    handleToggleRequiresApiKey,
    handleSaveEndpoint,
    handleAddKey,
    handleUpdateKey,
    handleToggleKey,
    handleDeleteKey,
  } = useProviders();

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Providers</h1>
        <p className="text-apple-gray-500 mt-1">Manage LLM providers and their API keys</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <ProviderList
          providers={providers}
          selectedProvider={selectedProvider}
          onSelect={setSelectedProvider}
          onToggle={handleToggleProvider}
        />

        <motion.div
          initial={{ opacity: 0, x: 10 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-3"
        >
          {selectedProvider && (
            <div className="space-y-6">
              <ProviderInfoCard
                provider={selectedProvider}
                proxies={proxies}
                healthStatus={healthStatus}
                testing={testing}
                savingProxy={savingProxy}
                onTestConnection={handleTestConnection}
                onToggleProxy={handleToggleProxy}
                onProxyChange={handleProxyChange}
              />

              {selectedProvider.requires_api_key && (
                <ApiKeyTable
                  providerName={selectedProvider.name}
                  apiKeys={apiKeys}
                  onAddKey={handleAddKey}
                  onUpdateKey={handleUpdateKey}
                  onToggleKey={handleToggleKey}
                  onDeleteKey={handleDeleteKey}
                />
              )}

              {['ollama', 'lmstudio', 'vllm'].includes(selectedProvider.name) && (
                <LocalProviderCard
                  provider={selectedProvider}
                  onToggleRequiresApiKey={handleToggleRequiresApiKey}
                  onSaveEndpoint={handleSaveEndpoint}
                />
              )}
            </div>
          )}
        </motion.div>
      </div>
    </div>
  );
}

export default ProvidersPage;
