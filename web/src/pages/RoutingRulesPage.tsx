import { useState, useMemo, lazy, Suspense } from 'react';
import { motion } from 'framer-motion';
import {
  PlusIcon,
  TrashIcon,
  PencilIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import { useQuery, useMutation } from '@apollo/client/react';
import {
  GET_ROUTING_RULES,
  CREATE_ROUTING_RULE,
  UPDATE_ROUTING_RULE,
  DELETE_ROUTING_RULE,
} from '@/lib/graphql/operations/routingRules';
import { PROVIDERS_QUERY } from '@/lib/graphql/operations';
import type { RoutingRule, Provider } from '@/lib/types';
import { useTranslation } from '@/lib/i18n';

const VisualRouterPage = lazy(() => import('./VisualRouterPage'));

/* eslint-disable @typescript-eslint/no-explicit-any */

interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

function ConfirmModal({
  isOpen,
  title,
  message,
  confirmText,
  onConfirm,
  onCancel,
  loading,
}: ConfirmModalProps) {
  const { t } = useTranslation();
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
      >
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center bg-red-100">
            <ExclamationTriangleIcon className="w-6 h-6 text-apple-red" />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-apple-gray-900">{title}</h3>
            <p className="mt-2 text-sm text-apple-gray-600">{message}</p>
          </div>
        </div>
        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onCancel} className="btn btn-secondary" disabled={loading}>
            {t('common.cancel')}
          </button>
          <button
            onClick={onConfirm}
            className="btn btn-danger"
            disabled={loading}
          >
            {loading ? t('routing_rules.processing') : confirmText}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

function RoutingRulesPage() {
  const { t } = useTranslation();
  const [viewMode, setViewMode] = useState<'rules' | 'visual'>('rules');
  const { data: rulesData, loading: rulesLoading, refetch: refetchRules } = useQuery<any>(GET_ROUTING_RULES, {
    variables: { page: 1, pageSize: 100 },
  });
  
  const { data: providersData } = useQuery<any>(PROVIDERS_QUERY);

  // Tab bar shared across both views
  const tabBar = (
    <div className="flex items-center gap-1 bg-apple-gray-100 p-1 rounded-xl w-fit border border-apple-gray-200 mb-6">
      <button
        onClick={() => setViewMode('rules')}
        className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
          viewMode === 'rules'
            ? 'bg-white text-apple-blue shadow-sm'
            : 'text-apple-gray-500 hover:text-apple-gray-700'
        }`}
      >
        {t('nav.routing_engine')}
      </button>
      <button
        onClick={() => setViewMode('visual')}
        className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
          viewMode === 'visual'
            ? 'bg-white text-apple-blue shadow-sm'
            : 'text-apple-gray-500 hover:text-apple-gray-700'
        }`}
      >
        {t('nav.visual_router')}
      </button>
    </div>
  );

  const [createRuleMut] = useMutation(CREATE_ROUTING_RULE);
  const [updateRuleMut] = useMutation(UPDATE_ROUTING_RULE);
  const [deleteRuleMut] = useMutation(DELETE_ROUTING_RULE);

  const rules: RoutingRule[] = useMemo(() => rulesData?.routingRules?.data || [], [rulesData]);
  const providers: Provider[] = useMemo(() => providersData?.providers || [], [providersData]);

  const [showModal, setShowModal] = useState(false);
  const [editingRule, setEditingRule] = useState<RoutingRule | null>(null);
  const [saving, setSaving] = useState(false);

  // Form state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [modelPattern, setModelPattern] = useState('');
  const [targetProviderId, setTargetProviderId] = useState('');
  const [fallbackProviderId, setFallbackProviderId] = useState<string>('');
  const [priority, setPriority] = useState(1);
  const [isEnabled, setIsEnabled] = useState(true);
  const fallbackProviders = useMemo(
    () => providers.filter((provider) => provider.id !== targetProviderId),
    [providers, targetProviderId]
  );

  // Delete confirm state
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    ruleId: string;
  }>({ isOpen: false, ruleId: '' });
  const [processingDelay, setProcessingDelete] = useState(false);

  // Visual Router tab
  if (viewMode === 'visual') {
    return (
      <div>
        {tabBar}
        <Suspense fallback={
          <div className="flex items-center justify-center h-64">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
          </div>
        }>
          <VisualRouterPage />
        </Suspense>
      </div>
    );
  }

  const openCreateModal = () => {
    setEditingRule(null);
    setName('');
    setDescription('');
    setModelPattern('');
    setTargetProviderId('');
    setFallbackProviderId('');
    setPriority(rules.length > 0 ? Math.max(...rules.map(r => r.priority)) + 1 : 1);
    setIsEnabled(true);
    setShowModal(true);
  };

  const openEditModal = (rule: RoutingRule) => {
    setEditingRule(rule);
    setName(rule.name);
    setDescription(rule.description || '');
    setModelPattern(rule.modelPattern);
    setTargetProviderId(rule.targetProviderId);
    setFallbackProviderId(rule.fallbackProviderId || '');
    setPriority(rule.priority);
    setIsEnabled(rule.isEnabled);
    setShowModal(true);
  };

  const handleSave = async () => {
    if (!name.trim() || !modelPattern.trim() || !targetProviderId) {
      toast.error(t('routing_rules.fill_required'));
      return;
    }

    setSaving(true);
    const input = {
      name: name.trim(),
      description: description.trim() || null,
      modelPattern: modelPattern.trim(),
      targetProviderId,
      fallbackProviderId: fallbackProviderId || null,
      priority: Number(priority),
      isEnabled,
    };

    try {
      if (editingRule) {
        await updateRuleMut({ variables: { id: editingRule.id, input } });
        toast.success(t('routing_rules.update_success'));
      } else {
        await createRuleMut({ variables: { input } });
        toast.success(t('routing_rules.create_success'));
      }
      await refetchRules();
      setShowModal(false);
    } catch (e: any) {
      toast.error(e.message || t('routing_rules.save_error'));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setProcessingDelete(true);
    try {
      await deleteRuleMut({ variables: { id: confirmModal.ruleId } });
      toast.success(t('routing_rules.delete_success'));
      await refetchRules();
      setConfirmModal({ isOpen: false, ruleId: '' });
    } catch (e: any) {
      toast.error(e.message || t('routing_rules.delete_error'));
    } finally {
      setProcessingDelete(false);
    }
  };

  const handleToggleStatus = async (rule: RoutingRule) => {
    try {
      const input = {
        name: rule.name,
        modelPattern: rule.modelPattern,
        targetProviderId: rule.targetProviderId,
        fallbackProviderId: rule.fallbackProviderId,
        priority: rule.priority,
        isEnabled: !rule.isEnabled,
      };
      await updateRuleMut({ variables: { id: rule.id, input } });
      toast.success(rule.isEnabled ? t('routing_rules.toggle_disabled') : t('routing_rules.toggle_enabled'));
      await refetchRules();
    } catch {
      toast.error(t('routing_rules.toggle_error'));
    }
  };

  if (rulesLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {tabBar}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('routing_rules.title')}</h1>
          <p className="text-apple-gray-500 mt-1">{t('routing_rules.subtitle')}</p>
        </div>
        {rules.length > 0 && (
          <button onClick={openCreateModal} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" />
            {t('routing_rules.create_rule')}
          </button>
        )}
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="card"
      >
        {rules.length === 0 ? (
          <div className="text-center py-16">
            <h3 className="text-lg font-semibold text-apple-gray-900 mb-1">{t('routing_rules.no_rules')}</h3>
            <p className="text-apple-gray-500 text-sm mb-6 max-w-sm mx-auto">
              {t('routing_rules.no_rules_desc')}
            </p>
            <button onClick={openCreateModal} className="btn btn-primary rounded-xl">
              {t('routing_rules.create_first_rule')}
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-apple-gray-200">
              <thead>
                <tr>
                  <th className="table-header">{t('routing_rules.col_name')}</th>
                  <th className="table-header">{t('routing_rules.col_pattern')}</th>
                  <th className="table-header">{t('routing_rules.col_target')}</th>
                  <th className="table-header">{t('routing_rules.col_fallback')}</th>
                  <th className="table-header">{t('routing_rules.col_priority')}</th>
                  <th className="table-header">{t('routing_rules.col_status')}</th>
                  <th className="table-header">{t('routing_rules.col_actions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-apple-gray-100">
                {rules.map((rule) => {
                  const targetName = rule.targetProvider?.name || providers.find(p => p.id === rule.targetProviderId)?.name || t('visual_router.unknown_provider');
                  const fallbackName = rule.fallbackProvider?.name || (rule.fallbackProviderId && providers.find(p => p.id === rule.fallbackProviderId)?.name) || t('routing_rules.fallback_label');
                  
                  return (
                    <tr key={rule.id} className="hover:bg-apple-gray-50">
                      <td className="table-cell font-medium">
                        {rule.name}
                        {rule.description && (
                          <div className="text-xs text-apple-gray-400 font-normal">{rule.description}</div>
                        )}
                      </td>
                      <td className="table-cell">
                        <code className="text-sm bg-apple-gray-100 text-apple-gray-800 px-2 py-1 rounded">
                          {rule.modelPattern}
                        </code>
                      </td>
                      <td className="table-cell">
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
                          {targetName}
                        </span>
                      </td>
                      <td className="table-cell text-apple-gray-500">
                        {rule.fallbackProviderId ? (
                           <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-orange-100 text-orange-800">
                             {fallbackName}
                           </span>
                        ) : t('routing_rules.fallback_label')}
                      </td>
                      <td className="table-cell">
                        {rule.priority}
                      </td>
                      <td className="table-cell">
                        <button
                          onClick={() => handleToggleStatus(rule)}
                          className={`relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                            rule.isEnabled ? 'bg-apple-green' : 'bg-apple-gray-200'
                          }`}
                        >
                          <span
                            className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                              rule.isEnabled ? 'translate-x-4' : 'translate-x-0'
                            }`}
                          />
                        </button>
                      </td>
                      <td className="table-cell">
                        <div className="flex items-center gap-3">
                          <button
                            onClick={() => openEditModal(rule)}
                            className="inline-flex items-center gap-1 text-sm text-apple-blue hover:text-blue-600 transition-colors"
                            title={t('routing_rules.edit_title')}
                          >
                            <PencilIcon className="w-4 h-4" />
                            {t('common.edit')}
                          </button>
                          <button
                            onClick={() => setConfirmModal({ isOpen: true, ruleId: rule.id })}
                            className="inline-flex items-center gap-1 text-sm text-apple-red hover:text-red-600 transition-colors"
                            title={t('routing_rules.delete_title')}
                          >
                            <TrashIcon className="w-4 h-4" />
                            {t('common.delete')}
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </motion.div>

      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">
              {editingRule ? t('routing_rules.edit_rule') : t('routing_rules.create_rule_modal')}
            </h2>
            <div className="space-y-4">
              <div>
                <label className="label">{t('routing_rules.rule_name')}</label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="input"
                  placeholder={t('routing_rules.rule_name_placeholder')}
                />
              </div>

              <div>
                <label className="label">{t('routing_rules.description')}</label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="input"
                  placeholder={t('routing_rules.description_placeholder')}
                />
              </div>

              <div>
                <label className="label">{t('routing_rules.model_pattern')}</label>
                <p className="text-xs text-apple-gray-500 mb-2">{t('routing_rules.model_pattern_hint')}</p>
                <input
                  type="text"
                  value={modelPattern}
                  onChange={(e) => setModelPattern(e.target.value)}
                  className="input font-mono"
                  placeholder={t('routing_rules.model_pattern_placeholder')}
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="label">{t('routing_rules.target_provider')}</label>
                  <select
                    value={targetProviderId}
                    onChange={(e) => {
                      setTargetProviderId(e.target.value);
                      if (fallbackProviderId === e.target.value) {
                        setFallbackProviderId('');
                      }
                    }}
                    className="input"
                  >
                    <option value="">{t('routing_rules.select_primary_provider')}</option>
                    {providers.map(p => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="label">{t('routing_rules.fallback_provider')}</label>
                  <select
                    value={fallbackProviderId}
                    onChange={(e) => setFallbackProviderId(e.target.value)}
                    className="input"
                  >
                    <option value="">{t('routing_rules.fallback_none')}</option>
                    {fallbackProviders.map(p => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="label">{t('routing_rules.priority')}</label>
                  <p className="text-xs text-apple-gray-500 mb-2">{t('routing_rules.priority_hint')}</p>
                  <input
                    type="number"
                    value={priority}
                    onChange={(e) => setPriority(parseInt(e.target.value))}
                    className="input"
                    min="1"
                  />
                </div>

                <div className="flex flex-col justify-end pb-3">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={isEnabled}
                      onChange={(e) => setIsEnabled(e.target.checked)}
                      className="rounded border-apple-gray-300 text-apple-blue focus:ring-apple-blue"
                    />
                    <span className="text-sm text-apple-gray-700">{t('routing_rules.enable_rule')}</span>
                  </label>
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-8">
              <button
                onClick={() => setShowModal(false)}
                className="btn btn-secondary"
              >
                {t('common.cancel')}
              </button>
              <button onClick={handleSave} className="btn btn-primary" disabled={saving}>
                {saving ? t('routing_rules.saving') : t('routing_rules.save_rule')}
              </button>
            </div>
          </motion.div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title={t('routing_rules.delete_modal_title')}
        message={t('routing_rules.delete_modal_message')}
        confirmText={t('routing_rules.delete_confirm_btn')}
        onConfirm={handleDelete}
        onCancel={() => setConfirmModal({ isOpen: false, ruleId: '' })}
        loading={processingDelay}
      />
    </div>
  );
}

export default RoutingRulesPage;
