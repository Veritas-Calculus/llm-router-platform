import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { PlusIcon, XMarkIcon, ChevronLeftIcon } from '@heroicons/react/24/outline';
import ProviderList from '@/components/providers/ProviderList';
import ProviderInfoCard from '@/components/providers/ProviderInfoCard';
import ApiKeyTable from '@/components/providers/ApiKeyTable';
import ModelTable from '@/components/providers/ModelTable';
import LocalProviderCard from '@/components/providers/LocalProviderCard';
import { useProviders } from '@/hooks/useProviders';
import { useTranslation } from '@/lib/i18n';
import toast from 'react-hot-toast';

/* eslint-disable @typescript-eslint/no-explicit-any */

const PROVIDER_PRESETS: { name: string; label: string; baseUrl: string; requiresApiKey: boolean }[] = [
  { name: 'openai',    label: 'OpenAI',        baseUrl: 'https://api.openai.com/v1',       requiresApiKey: true },
  { name: 'anthropic', label: 'Anthropic',      baseUrl: 'https://api.anthropic.com',       requiresApiKey: true },
  { name: 'google',    label: 'Google Gemini',  baseUrl: 'https://generativelanguage.googleapis.com/v1beta', requiresApiKey: true },
  { name: 'deepseek',  label: 'DeepSeek',       baseUrl: 'https://api.deepseek.com',        requiresApiKey: true },
  { name: 'mistral',   label: 'Mistral AI',     baseUrl: 'https://api.mistral.ai/v1',       requiresApiKey: true },
  { name: 'ollama',    label: 'Ollama',         baseUrl: 'http://localhost:11434/v1',        requiresApiKey: false },
  { name: 'lmstudio',  label: 'LM Studio',      baseUrl: 'http://localhost:1234/v1',         requiresApiKey: false },
  { name: 'vllm',      label: 'vLLM',           baseUrl: 'http://localhost:8000/v1',         requiresApiKey: false },
];

/** Generate a unique provider name by appending a numeric suffix if needed */
function uniqueProviderName(baseName: string, existingNames: string[]): string {
  if (!existingNames.includes(baseName)) return baseName;
  let i = 2;
  while (existingNames.includes(`${baseName}-${i}`)) i++;
  return `${baseName}-${i}`;
}

/** Known local-inference provider types (matched by prefix) */
const LOCAL_PROVIDER_TYPES = ['ollama', 'lmstudio', 'vllm'];

function isLocalProviderType(name: string): boolean {
  return LOCAL_PROVIDER_TYPES.some(t => name === t || name.startsWith(`${t}-`));
}

function AddProviderModal({
  open,
  onClose,
  onSubmit,
  existingNames,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: { name: string; baseUrl: string; requiresApiKey: boolean }) => Promise<void>;
  existingNames: string[];
}) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState('');
  const [customName, setCustomName] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [requiresApiKey, setRequiresApiKey] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const isCustom = selected === '__custom__';

  const handleSelectChange = (value: string) => {
    setSelected(value);
    if (value === '__custom__') {
      setCustomName('');
      setBaseUrl('');
      setRequiresApiKey(true);
    } else {
      const preset = PROVIDER_PRESETS.find(p => p.name === value);
      if (preset) {
        setBaseUrl(preset.baseUrl);
        setRequiresApiKey(preset.requiresApiKey);
      }
    }
  };

  // Auto-generate unique name if the preset already exists
  const rawName = isCustom ? customName.trim().toLowerCase() : selected;
  const finalName = rawName ? uniqueProviderName(rawName, existingNames) : '';
  const canSubmit = finalName && baseUrl.trim() && !submitting;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      await onSubmit({ name: finalName, baseUrl: baseUrl.trim(), requiresApiKey });
      setSelected('');
      setCustomName('');
      setBaseUrl('');
      setRequiresApiKey(true);
      onClose();
    } catch (err: any) {
      toast.error(err?.message || 'Failed to create provider');
    } finally {
      setSubmitting(false);
    }
  };

  if (!open) return null;

  return (
    <AnimatePresence>
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
          onClick={onClose}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 10 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 10 }}
            transition={{ type: 'spring', duration: 0.3 }}
            className="card w-full max-w-md mx-4 p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-semibold text-apple-gray-900">
                {t('providers.addProvider')}
              </h2>
              <button onClick={onClose} className="p-1 rounded-lg hover:bg-apple-gray-100 transition-colors">
                <XMarkIcon className="w-5 h-5 text-apple-gray-500" />
              </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              {/* Provider selector */}
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                  {t('providers.name')}
                </label>
                <select
                  value={selected}
                  onChange={(e) => handleSelectChange(e.target.value)}
                  className="w-full px-3 py-2.5 rounded-apple border border-apple-gray-200 bg-white text-apple-gray-900 focus:outline-none focus:ring-2 focus:ring-apple-blue/40 focus:border-apple-blue transition-colors appearance-none"
                  required
                  autoFocus
                >
                  <option value="" disabled>{t('providers.selectProvider')}</option>
                  {PROVIDER_PRESETS.map(p => (
                    <option key={p.name} value={p.name}>{p.label}</option>
                  ))}
                  <option value="__custom__">{t('providers.customProvider')}</option>
                </select>
                {/* Show auto-generated name hint when a duplicate is detected */}
                {!isCustom && finalName && finalName !== selected && (
                  <p className="mt-1 text-xs text-apple-blue">
                    {t('providers.willBeCreatedAs', { name: finalName })}
                  </p>
                )}
              </div>

              {/* Custom name input — only for custom providers */}
              {isCustom && (
                <div>
                  <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                    {t('providers.customName')}
                  </label>
                  <input
                    type="text"
                    value={customName}
                    onChange={(e) => setCustomName(e.target.value)}
                    placeholder="my-custom-provider"
                    className="w-full px-3 py-2 rounded-apple border border-apple-gray-200 bg-white text-apple-gray-900 focus:outline-none focus:ring-2 focus:ring-apple-blue/40 focus:border-apple-blue transition-colors"
                    required
                  />
                </div>
              )}

              {/* Base URL */}
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                  {t('providers.baseUrl')}
                </label>
                <input
                  type="url"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="https://api.openai.com/v1"
                  className="w-full px-3 py-2 rounded-apple border border-apple-gray-200 bg-white text-apple-gray-900 focus:outline-none focus:ring-2 focus:ring-apple-blue/40 focus:border-apple-blue transition-colors"
                  required
                />
              </div>

              {/* Requires API Key */}
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="requiresApiKey"
                  checked={requiresApiKey}
                  onChange={(e) => setRequiresApiKey(e.target.checked)}
                  className="rounded"
                />
                <label htmlFor="requiresApiKey" className="text-sm text-apple-gray-700">
                  {t('providers.requiresApiKey')}
                </label>
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-apple-gray-700 bg-apple-gray-100 rounded-apple hover:bg-apple-gray-200 transition-colors"
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="submit"
                  disabled={!canSubmit}
                  className="px-4 py-2 text-sm font-medium text-white bg-apple-blue rounded-apple hover:bg-apple-blue/90 transition-colors disabled:opacity-50"
                >
                  {submitting ? t('common.saving') : t('common.create')}
                </button>
              </div>
            </form>
          </motion.div>
        </motion.div>
    </AnimatePresence>
  );
}

function ProvidersPage() {
  const { t } = useTranslation();
  const [showAddModal, setShowAddModal] = useState(false);
  const [isMobileListVisible, setIsMobileListVisible] = useState(true);
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
    handleCreateProvider,
    handleDeleteProvider,
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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">
            {t('providers.title')}
          </h1>
          <p className="text-apple-gray-500 mt-1">
            {t('providers.subtitle')}
          </p>
        </div>
        {providers.length > 0 && (
          <button
            onClick={() => setShowAddModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-apple-blue rounded-apple hover:bg-apple-blue/90 transition-colors shadow-sm"
          >
            <PlusIcon className="w-4 h-4" />
            {t('providers.addProvider')}
          </button>
        )}
      </div>

      {providers.length === 0 ? (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="card p-12 text-center"
        >
          <div className="mx-auto w-16 h-16 rounded-full bg-apple-gray-100 flex items-center justify-center mb-4">
            <PlusIcon className="w-8 h-8 text-apple-gray-400" />
          </div>
          <h3 className="text-lg font-semibold text-apple-gray-900 mb-2">
            {t('providers.emptyTitle')}
          </h3>
          <p className="text-apple-gray-500 mb-6 max-w-md mx-auto">
            {t('providers.emptyDescription')}
          </p>
          <button
            onClick={() => setShowAddModal(true)}
            className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-medium text-white bg-apple-blue rounded-apple hover:bg-apple-blue/90 transition-colors shadow-sm"
          >
            <PlusIcon className="w-4 h-4" />
            {t('providers.addFirstProvider')}
          </button>
        </motion.div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          <div className={isMobileListVisible ? 'block lg:block' : 'hidden lg:block'}>
            <ProviderList
              providers={providers}
              selectedProvider={selectedProvider}
              onSelect={(p: any) => { setSelectedProvider(p); setIsMobileListVisible(false); }}
              onToggle={handleToggleProvider}
            />
          </div>

          <motion.div
            initial={{ opacity: 0, x: 10 }}
            animate={{ opacity: 1, x: 0 }}
            className={`lg:col-span-3 ${!isMobileListVisible ? 'block' : 'hidden lg:block'}`}
          >
            {selectedProvider && (
              <div className="space-y-6">
                <button 
                  onClick={() => setIsMobileListVisible(true)} 
                  className="lg:hidden flex items-center gap-1.5 px-3 py-1.5 -ml-3 text-sm font-medium text-apple-gray-600 hover:text-apple-gray-900 transition-colors"
                >
                  <ChevronLeftIcon className="w-5 h-5" />
                  Back to Providers
                </button>
                <ProviderInfoCard
                  provider={selectedProvider}
                  proxies={proxies}
                  healthStatus={healthStatus}
                  testing={testing}
                  savingProxy={savingProxy}
                  onTestConnection={handleTestConnection}
                  onToggleProxy={handleToggleProxy}
                  onProxyChange={handleProxyChange}
                  onDeleteProvider={handleDeleteProvider}
                />

                {isLocalProviderType(selectedProvider.name) && (
                  <LocalProviderCard
                    provider={selectedProvider}
                    onToggleRequiresApiKey={handleToggleRequiresApiKey}
                    onSaveEndpoint={handleSaveEndpoint}
                  />
                )}

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

                <ModelTable
                  providerId={selectedProvider.id}
                  providerName={selectedProvider.name}
                />
              </div>
            )}
          </motion.div>
        </div>
      )}

      <AddProviderModal
        open={showAddModal}
        onClose={() => setShowAddModal(false)}
        onSubmit={handleCreateProvider}
        existingNames={providers.map((p: any) => p.name)}
      />
    </div>
  );
}

export default ProvidersPage;
