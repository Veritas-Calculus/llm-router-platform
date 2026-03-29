/* eslint-disable react-refresh/only-export-components */
import { motion } from 'framer-motion';
import { useQuery } from '@apollo/client/react';
import { ExclamationTriangleIcon, ExclamationCircleIcon } from '@heroicons/react/24/outline';
import { SUBSCRIPTION_QUOTA_QUERY } from '@/lib/graphql/operations/billing';
import { API_KEY_RATE_LIMIT_STATUS } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';
import type { ApiKey } from '@/lib/types';

/* ── Constants ── */

export const AVAILABLE_SCOPES_BASE = [
  { id: 'all', labelKey: 'api_keys.scopes_all' },
  { id: 'chat', labelKey: 'api_keys.scopes_chat' },
  { id: 'embeddings', labelKey: 'api_keys.scopes_embeddings' },
  { id: 'images', labelKey: 'api_keys.scopes_images' },
  { id: 'audio', labelKey: 'api_keys.scopes_audio' },
  { id: 'admin', labelKey: 'api_keys.scopes_admin' },
];

const STATUS_BADGE_BASE: Record<string, { labelKey: string; className: string }> = {
  ok: { labelKey: 'api_keys.ok', className: 'bg-green-50 text-green-700 border-green-200' },
  near_limit: { labelKey: 'api_keys.near_limit', className: 'bg-orange-50 text-orange-700 border-orange-200' },
  rate_limited: { labelKey: 'api_keys.rate_limited', className: 'bg-red-50 text-red-700 border-red-200' },
  quota_exceeded: { labelKey: 'api_keys.quota_exceeded', className: 'bg-red-50 text-red-700 border-red-200' },
};

/* ── Utility ── */

interface RawApiKeyData {
  id: string;
  projectId: string;
  channel: string;
  name: string;
  key?: string;
  keyPrefix: string;
  isActive: boolean;
  scopes: string;
  rateLimit: number;
  tokenLimit: number;
  dailyLimit: number;
  createdAt: string;
  lastUsedAt: string;
  expiresAt: string;
}

interface RateLimitStatusData {
  apiKeyRateLimitStatus: {
    status: string;
    rpmCurrent: number;
    rpmLimit: number;
    tpmCurrent: number;
    tpmLimit: number;
    dailyCurrent: number;
    dailyLimit: number;
  };
}

interface SubscriptionQuotaData {
  mySubscription: {
    planName: string;
    tokenLimit: number;
    usedTokens: number;
    quotaPercentage: number;
    isQuotaExceeded: boolean;
  };
}

export function mapApiKey(d: RawApiKeyData): ApiKey {
  return {
    id: d.id, project_id: d.projectId, channel: d.channel, name: d.name, key: d.key || '', key_prefix: d.keyPrefix,
    is_active: d.isActive, scopes: d.scopes, rate_limit: d.rateLimit, token_limit: d.tokenLimit, daily_limit: d.dailyLimit,
    created_at: d.createdAt, last_used_at: d.lastUsedAt, expires_at: d.expiresAt,
  };
}

export function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric',
  });
}

/* ── Rate Limit Components ── */

function RateLimitMiniBar({ current, limit, label }: { current: number; limit: number; label: string }) {
  if (limit <= 0) return <div className="text-[10px] text-apple-gray-400">{label}: Unlimited</div>;
  const pct = Math.min((current / limit) * 100, 100);
  const color = pct >= 100 ? 'bg-red-500' : pct >= 80 ? 'bg-orange-400' : 'bg-green-500';
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-[10px] text-apple-gray-400 w-8 shrink-0">{label}</span>
      <div className="flex-1 h-1.5 bg-apple-gray-100 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color} transition-all duration-300`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-[10px] text-apple-gray-500 w-16 text-right">{current}/{limit}</span>
    </div>
  );
}

export function RateLimitStatusCell({ keyId, isActive }: { keyId: string; isActive: boolean }) {
  const { t } = useTranslation();
  const { data } = useQuery<RateLimitStatusData>(API_KEY_RATE_LIMIT_STATUS, {
    variables: { keyId },
    skip: !isActive,
    pollInterval: 10000,
    fetchPolicy: 'network-only',
  });
  if (!isActive) return null;
  const s = data?.apiKeyRateLimitStatus;
  if (!s) return <span className="text-[10px] text-apple-gray-300">—</span>;
  const badgeBase = STATUS_BADGE_BASE[s.status] || STATUS_BADGE_BASE.ok;
  return (
    <div className="space-y-1.5">
      <span className={`inline-flex px-1.5 py-0.5 rounded-md text-[10px] font-medium border ${badgeBase.className}`}>
        {t(badgeBase.labelKey)}
      </span>
      <RateLimitMiniBar current={s.rpmCurrent} limit={s.rpmLimit} label={t('api_keys.rpm')} />
      <RateLimitMiniBar current={s.tpmCurrent} limit={s.tpmLimit} label={t('api_keys.tpm')} />
      <RateLimitMiniBar current={s.dailyCurrent} limit={s.dailyLimit} label={t('api_keys.daily')} />
    </div>
  );
}

/* ── Subscription Quota Banner ── */

export function SubscriptionQuotaBanner() {
  const { data } = useQuery<SubscriptionQuotaData>(SUBSCRIPTION_QUOTA_QUERY, { fetchPolicy: 'cache-and-network' });
  const sub = data?.mySubscription;
  if (!sub || sub.tokenLimit <= 0) return null;

  const pct = sub.quotaPercentage;
  const isExceeded = sub.isQuotaExceeded;
  const isNear = pct >= 80 && !isExceeded;

  const barColor = isExceeded ? 'bg-red-500' : isNear ? 'bg-orange-400' : 'bg-blue-500';
  const bgColor = isExceeded ? 'bg-red-50 border-red-200' : isNear ? 'bg-orange-50 border-orange-200' : 'bg-blue-50 border-blue-200';
  const textColor = isExceeded ? 'text-red-700' : isNear ? 'text-orange-700' : 'text-blue-700';
  const iconColor = isExceeded ? 'text-red-500' : isNear ? 'text-orange-500' : 'text-blue-500';

  const fmtTokens = (n: number) => n >= 1000000 ? `${(n / 1000000).toFixed(1)}M` : n >= 1000 ? `${(n / 1000).toFixed(1)}K` : `${n}`;

  return (
    <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className={`rounded-apple-lg border p-4 ${bgColor}`}>
      <div className="flex items-center gap-3">
        <ExclamationCircleIcon className={`w-5 h-5 shrink-0 ${iconColor}`} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-1">
            <span className={`text-sm font-medium ${textColor}`}>{sub.planName} Plan -- Token Quota</span>
            <span className={`text-xs font-medium ${textColor}`}>
              {fmtTokens(sub.usedTokens)} / {fmtTokens(sub.tokenLimit)}
              {isExceeded && ' (Exceeded)'}
            </span>
          </div>
          <div className="h-2 bg-white/60 rounded-full overflow-hidden">
            <div className={`h-full rounded-full ${barColor} transition-all duration-500`} style={{ width: `${pct}%` }} />
          </div>
          {isExceeded && (
            <p className="text-xs text-red-600 mt-1">Monthly token limit reached. API requests will be rejected until the next billing period.</p>
          )}
        </div>
      </div>
    </motion.div>
  );
}

/* ── Confirm Modal ── */

export interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText: string;
  confirmColor: 'red' | 'orange';
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

export function ConfirmModal({ isOpen, title, message, confirmText, confirmColor, onConfirm, onCancel, loading }: ConfirmModalProps) {
  if (!isOpen) return null;
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }}
        className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4">
        <div className="flex items-start gap-4">
          <div className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${confirmColor === 'red' ? 'bg-red-100' : 'bg-orange-100'}`}>
            <ExclamationTriangleIcon className={`w-6 h-6 ${confirmColor === 'red' ? 'text-apple-red' : 'text-apple-orange'}`} />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-apple-gray-900">{title}</h3>
            <p className="mt-2 text-sm text-apple-gray-600">{message}</p>
          </div>
        </div>
        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onCancel} className="btn btn-secondary" disabled={loading}>Cancel</button>
          <button onClick={onConfirm} className={`btn ${confirmColor === 'red' ? 'btn-danger' : 'bg-apple-orange text-white hover:opacity-90'}`} disabled={loading}>
            {loading ? 'Processing...' : confirmText}
          </button>
        </div>
      </motion.div>
    </div>
  );
}
