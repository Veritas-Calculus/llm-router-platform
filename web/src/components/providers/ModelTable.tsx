import { useState } from 'react';
import { useMutation, useQuery } from '@apollo/client/react';
import {
  PlusIcon,
  TrashIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  XCircleIcon,
} from '@heroicons/react/24/outline';
import {
  MODELS_QUERY,
  CREATE_MODEL,
  DELETE_MODEL,
  TOGGLE_MODEL,
  SYNC_PROVIDER_MODELS,
} from '@/lib/graphql/operations/providers';
import ConfirmModal from '@/components/ConfirmModal';
import { useTranslation } from '@/lib/i18n';
import { moneyNumber, type MoneyValue } from '@/lib/format';
import toast from 'react-hot-toast';

interface ModelTableProps {
  providerId: string;
  providerName: string;
}

interface ModelItem {
  id: string;
  name: string;
  displayName: string;
  inputPricePer1k: MoneyValue;
  outputPricePer1k: MoneyValue;
  pricePerSecond?: MoneyValue | null;
  pricePerImage?: MoneyValue | null;
  pricePerMinute?: MoneyValue | null;
  providerInputCostPer1k: MoneyValue;
  providerOutputCostPer1k: MoneyValue;
  providerCostPerSecond?: MoneyValue | null;
  providerCostPerImage?: MoneyValue | null;
  providerCostPerMinute?: MoneyValue | null;
  maxTokens: number;
  isActive: boolean;
}

const initialModelForm = {
  name: '',
  displayName: '',
  inputPricePer1k: 0,
  outputPricePer1k: 0,
  pricePerSecond: 0,
  pricePerImage: 0,
  pricePerMinute: 0,
  providerInputCostPer1k: 0,
  providerOutputCostPer1k: 0,
  providerCostPerSecond: 0,
  providerCostPerImage: 0,
  providerCostPerMinute: 0,
  maxTokens: 4096,
};

const fmtRate = (value?: MoneyValue | null) => `$${moneyNumber(value).toFixed(4)}`;

export default function ModelTable({ providerId, providerName }: ModelTableProps) {
  const { t } = useTranslation();
  const [showAddModal, setShowAddModal] = useState(false);
  const [newModel, setNewModel] = useState(initialModelForm);
  const [confirmModal, setConfirmModal] = useState<{ isOpen: boolean; modelId: string }>({ isOpen: false, modelId: '' });

  const { data, loading, refetch } = useQuery<{ models: ModelItem[] }>(MODELS_QUERY, {
    variables: { providerId },
    fetchPolicy: 'cache-and-network',
  });

  const [createModel, { loading: creating }] = useMutation(CREATE_MODEL, {
    onCompleted: () => { refetch(); setShowAddModal(false); setNewModel(initialModelForm); },
  });

  const [deleteModel, { loading: deleting }] = useMutation(DELETE_MODEL, {
    onCompleted: () => { refetch(); setConfirmModal({ isOpen: false, modelId: '' }); },
  });

  const [toggleModel] = useMutation(TOGGLE_MODEL, { onCompleted: () => refetch() });

  const [syncModels, { loading: syncing }] = useMutation(SYNC_PROVIDER_MODELS, {
    variables: { providerId },
    onCompleted: async (result) => {
      await refetch();
      const count = (result as { syncProviderModels?: ModelItem[] })?.syncProviderModels?.length ?? 0;
      toast.success(
        count > 0
          ? t('providers.sync_models_success', { count })
          : t('providers.sync_models_empty')
      );
    },
    onError: (error) => {
      toast.error(error.message || t('providers.sync_models_failed'));
    },
  });

  const models: ModelItem[] = data?.models ?? [];

  const handleAddModel = async () => {
    if (!newModel.name.trim()) return;
    await createModel({
      variables: {
        providerId,
        input: {
          name: newModel.name.trim(),
          displayName: newModel.displayName.trim() || newModel.name.trim(),
          inputPricePer1k: newModel.inputPricePer1k,
          outputPricePer1k: newModel.outputPricePer1k,
          pricePerSecond: newModel.pricePerSecond,
          pricePerImage: newModel.pricePerImage,
          pricePerMinute: newModel.pricePerMinute,
          providerInputCostPer1k: newModel.providerInputCostPer1k,
          providerOutputCostPer1k: newModel.providerOutputCostPer1k,
          providerCostPerSecond: newModel.providerCostPerSecond,
          providerCostPerImage: newModel.providerCostPerImage,
          providerCostPerMinute: newModel.providerCostPerMinute,
          maxTokens: newModel.maxTokens,
        },
      },
    });
  };

  return (
    <div className="card overflow-x-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h3 className="text-lg font-semibold text-apple-gray-900">{t('providers.models')}</h3>
          <p className="text-sm text-apple-gray-500 mt-1">
            {t('providers.models_desc', { name: providerName })}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => syncModels()}
            disabled={syncing}
            className="btn btn-secondary"
            title={t('providers.sync_models')}
          >
            <ArrowPathIcon className={`w-5 h-5 mr-2 ${syncing ? 'animate-spin' : ''}`} />
            {syncing ? t('providers.syncing') : t('providers.sync_models')}
          </button>
          <button onClick={() => setShowAddModal(true)} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" />
            {t('providers.add_model')}
          </button>
        </div>
      </div>

      {loading && models.length === 0 ? (
        <div className="flex items-center justify-center h-32">
          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-apple-blue" />
        </div>
      ) : models.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-apple-gray-500 mb-4">{t('providers.no_models')}</p>
          <button onClick={() => syncModels()} disabled={syncing} className="btn btn-primary">
            <ArrowPathIcon className={`w-5 h-5 mr-2 ${syncing ? 'animate-spin' : ''}`} />
            {syncing ? t('providers.syncing') : t('providers.sync_models')}
          </button>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-apple-gray-200">
            <thead>
	              <tr>
	                <th className="table-header">{t('providers.model_name')}</th>
	                <th className="table-header">{t('providers.customer_price')}</th>
	                <th className="table-header">{t('providers.provider_cost')}</th>
	                <th className="table-header">{t('providers.max_tokens')}</th>
                <th className="table-header">{t('common.status')}</th>
                <th className="table-header text-right">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {models.map((m) => (
                <tr key={m.id} className="hover:bg-apple-gray-50">
                  <td className="table-cell">
                    <span className="font-medium text-apple-gray-900 block">{m.displayName || m.name}</span>
                    {m.displayName && m.displayName !== m.name && (
                      <code className="text-xs bg-apple-gray-100 px-1 py-0.5 rounded mt-1 inline-block">
                        {m.name}
                      </code>
                    )}
	                  </td>
	                  <td className="table-cell text-sm">
	                    <span className="text-apple-gray-700">{fmtRate(m.inputPricePer1k)}</span>
	                    <span className="text-apple-gray-400 text-xs"> in</span>
	                    <span className="text-apple-gray-300 mx-1">/</span>
	                    <span className="text-apple-gray-700">{fmtRate(m.outputPricePer1k)}</span>
	                    <span className="text-apple-gray-400 text-xs"> out</span>
	                  </td>
	                  <td className="table-cell text-sm">
	                    <span className="text-apple-gray-700">{fmtRate(m.providerInputCostPer1k)}</span>
	                    <span className="text-apple-gray-400 text-xs"> in</span>
	                    <span className="text-apple-gray-300 mx-1">/</span>
	                    <span className="text-apple-gray-700">{fmtRate(m.providerOutputCostPer1k)}</span>
	                    <span className="text-apple-gray-400 text-xs"> out</span>
	                  </td>
                  <td className="table-cell text-sm text-apple-gray-500">
                    {m.maxTokens.toLocaleString()}
                  </td>
                  <td className="table-cell">
                    <button
                      onClick={() => toggleModel({ variables: { id: m.id } })}
                      className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${m.isActive
                        ? 'bg-green-100 text-apple-green hover:bg-green-200'
                        : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
                        }`}
                    >
                      {m.isActive ? (
                        <><CheckCircleIcon className="w-3.5 h-3.5" /> Active</>
                      ) : (
                        <><XCircleIcon className="w-3.5 h-3.5" /> Inactive</>
                      )}
                    </button>
                  </td>
                  <td className="table-cell text-right">
                    <button
                      onClick={() => setConfirmModal({ isOpen: true, modelId: m.id })}
                      className="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg text-apple-gray-400 hover:text-apple-red hover:bg-red-50 transition-colors text-sm"
                      title={t('common.delete')}
                    >
                      <TrashIcon className="w-4 h-4" />
                      {t('common.delete')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="text-xs text-apple-gray-400 mt-3 text-right">
            {models.length} {t('providers.models_count')}
          </div>
        </div>
      )}

      {/* Add Model Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-3xl mx-4 max-h-[90vh] overflow-y-auto">
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">{t('providers.add_model')}</h2>
            <div className="space-y-4">
              <div>
                <label className="label">{t('providers.model_name')}</label>
                <input
                  type="text"
                  value={newModel.name}
                  onChange={(e) => setNewModel((prev) => ({ ...prev, name: e.target.value }))}
                  className="input"
                  placeholder="gpt-4o"
                />
              </div>
              <div>
                <label className="label">{t('providers.display_name')}</label>
                <input
                  type="text"
                  value={newModel.displayName}
                  onChange={(e) => setNewModel((prev) => ({ ...prev, displayName: e.target.value }))}
                  className="input"
                  placeholder="GPT-4o"
                />
              </div>
              <div>
                <p className="text-sm font-medium text-apple-gray-700 mb-2">{t('providers.customer_price')}</p>
                <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                  <div>
                    <label className="label">{t('providers.input_price')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.inputPricePer1k}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, inputPricePer1k: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.output_price')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.outputPricePer1k}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, outputPricePer1k: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.price_per_second')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.pricePerSecond}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, pricePerSecond: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.price_per_image')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.pricePerImage}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, pricePerImage: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.price_per_minute')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.pricePerMinute}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, pricePerMinute: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                </div>
              </div>
              <div>
                <p className="text-sm font-medium text-apple-gray-700 mb-2">{t('providers.provider_cost')}</p>
                <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                  <div>
                    <label className="label">{t('providers.input_cost')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.providerInputCostPer1k}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, providerInputCostPer1k: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.output_cost')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.providerOutputCostPer1k}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, providerOutputCostPer1k: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.cost_per_second')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.providerCostPerSecond}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, providerCostPerSecond: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.cost_per_image')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.providerCostPerImage}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, providerCostPerImage: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">{t('providers.cost_per_minute')}</label>
                    <input
                      type="number" step="0.0001" min="0"
                      value={newModel.providerCostPerMinute}
                      onChange={(e) => setNewModel((prev) => ({ ...prev, providerCostPerMinute: parseFloat(e.target.value) || 0 }))}
                      className="input"
                    />
                  </div>
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div>
                  <label className="label">{t('providers.max_tokens')}</label>
                  <input
                    type="number" min="1"
                    value={newModel.maxTokens}
                    onChange={(e) => setNewModel((prev) => ({ ...prev, maxTokens: parseInt(e.target.value) || 4096 }))}
                    className="input"
                  />
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowAddModal(false)} className="btn btn-secondary">{t('common.cancel')}</button>
              <button onClick={handleAddModel} className="btn btn-primary" disabled={creating}>
                {creating ? t('common.adding') : t('providers.add_model')}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title={t('providers.delete_model')}
        message={t('providers.delete_model_confirm')}
        confirmText={t('common.delete')}
        confirmColor="red"
        onConfirm={async () => {
          await deleteModel({ variables: { id: confirmModal.modelId } });
        }}
        onCancel={() => setConfirmModal({ isOpen: false, modelId: '' })}
        loading={deleting}
      />
    </div>
  );
}
