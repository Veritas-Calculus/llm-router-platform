import {
  CheckCircleIcon,
  XCircleIcon,
  TrashIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';
import { moneyNumber, type MoneyValue } from '@/lib/format';
import { useTranslation } from '@/lib/i18n';

/**
 * One row inside ModelTable. We accept the union of fields used by every
 * kind-specific sub-table so the parent can pass the same shape to all of
 * them.
 */
export interface ModelGroupItem {
  id: string;
  name: string;
  displayName: string;
  modelKind: ModelKind;
  inputPricePer1k: MoneyValue;
  outputPricePer1k: MoneyValue;
  providerInputCostPer1k: MoneyValue;
  providerOutputCostPer1k: MoneyValue;
  pricePerImage?: MoneyValue | null;
  providerCostPerImage?: MoneyValue | null;
  contextWindow?: number | null;
  maxOutputTokens?: number | null;
  maxTokens: number; // legacy / fallback
  catalogWarnings?: string | null;
  isActive: boolean;
}

export type ModelKind = 'CHAT' | 'EMBEDDING' | 'IMAGE' | 'STT' | 'TTS' | 'RERANK' | 'UNKNOWN';

const fmtRate = (value?: MoneyValue | null) => `$${moneyNumber(value).toFixed(4)}`;

const KIND_TITLES: Record<ModelKind, { en: string; zh: string }> = {
  CHAT: { en: 'Chat', zh: '对话' },
  EMBEDDING: { en: 'Embedding', zh: '向量化' },
  IMAGE: { en: 'Image', zh: '图像生成' },
  STT: { en: 'Speech-to-Text (STT)', zh: '语音转写 (STT)' },
  TTS: { en: 'Text-to-Speech (TTS)', zh: '语音合成 (TTS)' },
  RERANK: { en: 'Reranker', zh: '重排序' },
  UNKNOWN: { en: 'Other', zh: '其他' },
};

interface NameCellProps {
  item: ModelGroupItem;
}

function NameCell({ item }: NameCellProps) {
  const { t } = useTranslation();
  const hasWarning = Boolean(item.catalogWarnings && item.catalogWarnings.trim());
  return (
    <td className="table-cell">
      <span className="font-medium text-apple-gray-900 dark:text-gray-100 block">
        {item.displayName || item.name}
      </span>
      {item.displayName && item.displayName !== item.name && (
        <code className="text-xs bg-apple-gray-100 dark:bg-white/5 px-1 py-0.5 rounded mt-1 inline-block">
          {item.name}
        </code>
      )}
      {hasWarning && (
        <div
          className="mt-1 flex items-center gap-1 text-[11px] text-amber-600"
          title={item.catalogWarnings ?? ''}
        >
          <ExclamationTriangleIcon className="w-3.5 h-3.5 shrink-0" />
          <span>{t('providers.warning_badge')}</span>
        </div>
      )}
    </td>
  );
}

interface StatusCellProps {
  isActive: boolean;
  onToggle: () => void;
}

function StatusCell({ isActive, onToggle }: StatusCellProps) {
  return (
    <td className="table-cell">
      <button
        onClick={onToggle}
        className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${
          isActive
            ? 'bg-green-100 text-apple-green hover:bg-green-200'
            : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
        }`}
      >
        {isActive ? (
          <>
            <CheckCircleIcon className="w-3.5 h-3.5" /> Active
          </>
        ) : (
          <>
            <XCircleIcon className="w-3.5 h-3.5" /> Inactive
          </>
        )}
      </button>
    </td>
  );
}

interface ActionsCellProps {
  onDelete: () => void;
}

function ActionsCell({ onDelete }: ActionsCellProps) {
  const { t } = useTranslation();
  return (
    <td className="table-cell text-right">
      <button
        onClick={onDelete}
        className="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg text-apple-gray-400 hover:text-apple-red hover:bg-red-50 transition-colors text-sm"
        title={t('common.delete')}
      >
        <TrashIcon className="w-4 h-4" />
        {t('common.delete')}
      </button>
    </td>
  );
}

interface ModelKindGroupProps {
  kind: ModelKind;
  items: ModelGroupItem[];
  onToggle: (id: string) => void;
  onDelete: (id: string) => void;
}

/**
 * Renders one group of models, picking the right columns for the kind.
 * Each kind has its own column set because:
 *   * Chat needs in/out prices and a context-window vs max-output split
 *     (audit M-07).
 *   * Embedding is a single price-per-token, no output side.
 *   * STT/TTS pricing is per-minute and orthogonal — the catalog only
 *     shows name + status (audit M-08).
 *   * Image is per-image pricing.
 */
export default function ModelKindGroup({ kind, items, onToggle, onDelete }: ModelKindGroupProps) {
  const { t, locale } = useTranslation();
  if (items.length === 0) return null;

  // Pick a friendly group title. We fall back to the English label when
  // the locale-specific copy is missing — this avoids losing the group
  // header during incremental i18n migration.
  const titleEntry = KIND_TITLES[kind];
  const titleStr =
    locale === 'zh-CN' && titleEntry.zh ? titleEntry.zh : titleEntry.en;

  return (
    <section className="mt-6 first:mt-0">
      <h4 className="text-sm font-semibold text-apple-gray-700 dark:text-gray-200 mb-2">
        {titleStr}{' '}
        <span className="text-apple-gray-400 dark:text-gray-500 font-normal">({items.length})</span>
      </h4>
      <div className="overflow-x-auto rounded-2xl border border-apple-gray-100 dark:border-white/10">
        <table className="min-w-full divide-y divide-apple-gray-200 dark:divide-white/10">
          <thead>
            <tr className="bg-apple-gray-50 dark:bg-white/5">
              <th className="table-header">{t('providers.model_name')}</th>
              {kind === 'CHAT' && (
                <>
                  <th className="table-header">{t('providers.customer_price')}</th>
                  <th className="table-header">{t('providers.provider_cost')}</th>
                  <th className="table-header">{t('providers.context_window')}</th>
                  <th className="table-header">{t('providers.max_output_tokens')}</th>
                </>
              )}
              {kind === 'EMBEDDING' && (
                <>
                  <th className="table-header">{t('providers.customer_price_single')}</th>
                  <th className="table-header">{t('providers.provider_cost_single')}</th>
                  <th className="table-header">{t('providers.context_window')}</th>
                </>
              )}
              {kind === 'IMAGE' && (
                <>
                  <th className="table-header">{t('providers.price_per_image')}</th>
                  <th className="table-header">{t('providers.cost_per_image')}</th>
                </>
              )}
              {(kind === 'STT' || kind === 'TTS' || kind === 'RERANK' || kind === 'UNKNOWN') && (
                <th className="table-header" colSpan={2}>
                  {t('providers.audio_pricing_hint')}
                </th>
              )}
              <th className="table-header">{t('common.status')}</th>
              <th className="table-header text-right">{t('common.actions')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-apple-gray-100 dark:divide-white/10">
            {items.map((m) => (
              <tr key={m.id} className="hover:bg-apple-gray-50 dark:hover:bg-white/5">
                <NameCell item={m} />
                {kind === 'CHAT' && (
                  <>
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
                    <td className="table-cell text-sm text-apple-gray-700">
                      {(m.contextWindow || m.maxTokens || 0).toLocaleString()}
                    </td>
                    <td className="table-cell text-sm text-apple-gray-500">
                      {m.maxOutputTokens ? m.maxOutputTokens.toLocaleString() : '—'}
                    </td>
                  </>
                )}
                {kind === 'EMBEDDING' && (
                  <>
                    <td className="table-cell text-sm text-apple-gray-700">
                      {fmtRate(m.inputPricePer1k)}
                    </td>
                    <td className="table-cell text-sm text-apple-gray-700">
                      {fmtRate(m.providerInputCostPer1k)}
                    </td>
                    <td className="table-cell text-sm text-apple-gray-700">
                      {(m.contextWindow || m.maxTokens || 0).toLocaleString()}
                    </td>
                  </>
                )}
                {kind === 'IMAGE' && (
                  <>
                    <td className="table-cell text-sm text-apple-gray-700">
                      {fmtRate(m.pricePerImage)}
                    </td>
                    <td className="table-cell text-sm text-apple-gray-700">
                      {fmtRate(m.providerCostPerImage)}
                    </td>
                  </>
                )}
                {(kind === 'STT' || kind === 'TTS' || kind === 'RERANK' || kind === 'UNKNOWN') && (
                  <td className="table-cell text-sm text-apple-gray-400 italic" colSpan={2}>
                    {t('providers.no_pricing_columns')}
                  </td>
                )}
                <StatusCell isActive={m.isActive} onToggle={() => onToggle(m.id)} />
                <ActionsCell onDelete={() => onDelete(m.id)} />
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
