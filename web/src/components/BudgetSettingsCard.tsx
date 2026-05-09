/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useMutation, useQuery } from '@apollo/client/react';
import {
  MY_BILLING_QUERY,
  SET_BUDGET,
  DELETE_BUDGET,
} from '@/lib/graphql/operations/billing';
import { useTranslation } from '@/lib/i18n';
import { formatUSD, formatPercent } from '@/lib/format';

// BudgetSettingsCard surfaces the user's monthly spending budget for direct
// editing from the Settings page. Without this UI, only the backend-set values
// were visible — the SET_BUDGET / DELETE_BUDGET mutations existed but had no
// caller. Now the user can configure both the soft alert threshold (used by
// the dashboard banner) and the hard limit (enforced at request time by the
// quota middleware).
function BudgetSettingsCard() {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<any>(MY_BILLING_QUERY);
  const [setBudgetMut, { loading: saving }] = useMutation<any>(SET_BUDGET);
  const [deleteBudgetMut, { loading: deleting }] = useMutation<any>(DELETE_BUDGET);

  const myBudget = data?.myBudget;
  const myBudgetStatus = data?.myBudgetStatus;

  const [monthlyLimit, setMonthlyLimit] = useState<string>('');
  const [alertThreshold, setAlertThreshold] = useState<string>('80');
  const [enforceHardLimit, setEnforceHardLimit] = useState<boolean>(false);
  const [email, setEmail] = useState<string>('');

  useEffect(() => {
    if (myBudget) {
      setMonthlyLimit(myBudget.monthlyLimitUsd != null ? String(myBudget.monthlyLimitUsd) : '');
      // Backend stores AlertThreshold as a fraction (0.8) historically, but
      // the dashboard renders a percent. Normalize both directions.
      const raw = Number(myBudget.alertThreshold ?? 80);
      const pct = raw <= 1 ? raw * 100 : raw;
      setAlertThreshold(String(pct));
      setEnforceHardLimit(Boolean(myBudget.enforceHardLimit));
      setEmail(myBudget.email || '');
    }
  }, [myBudget]);

  const handleSave = async () => {
    const limitNum = parseFloat(monthlyLimit);
    if (!Number.isFinite(limitNum) || limitNum <= 0) {
      toast.error(t('budget.invalid_limit'));
      return;
    }
    const thresholdNum = parseFloat(alertThreshold);
    const thresholdFraction = Number.isFinite(thresholdNum) ? thresholdNum / 100 : 0.8;

    try {
      await setBudgetMut({
        variables: {
          input: {
            monthlyLimitUsd: limitNum,
            alertThreshold: thresholdFraction,
            enforceHardLimit,
            email: email || null,
          },
        },
      });
      toast.success(t('budget.saved'));
      refetch();
    } catch (err: any) {
      toast.error(err?.message || t('budget.save_error'));
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(t('budget.delete_confirm'))) return;
    try {
      await deleteBudgetMut();
      toast.success(t('budget.deleted'));
      setMonthlyLimit('');
      setAlertThreshold('80');
      setEnforceHardLimit(false);
      setEmail('');
      refetch();
    } catch (err: any) {
      toast.error(err?.message || t('budget.delete_error'));
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.15 }}
      className="card max-w-2xl"
    >
      <h2 className="text-lg font-semibold text-apple-gray-900 mb-2">
        {t('budget.title')}
      </h2>
      <p className="text-sm text-apple-gray-500 mb-6">{t('budget.subtitle')}</p>

      {!loading && myBudgetStatus?.budget && (
        <div className="mb-6 p-3 rounded-lg bg-apple-gray-50 dark:bg-[var(--theme-bg-input)] text-sm">
          <div className="flex justify-between">
            <span className="text-apple-gray-500">{t('budget.current_spend')}</span>
            <span className="font-medium">
              {formatUSD(myBudgetStatus.currentSpend || 0)} ({formatPercent(myBudgetStatus.percentUsed || 0)})
            </span>
          </div>
        </div>
      )}

      <div className="space-y-4">
        <div>
          <label htmlFor="monthly-limit" className="label">
            {t('budget.monthly_limit_usd')}
          </label>
          <input
            id="monthly-limit"
            type="number"
            min="0"
            step="0.01"
            value={monthlyLimit}
            onChange={(e) => setMonthlyLimit(e.target.value)}
            className="input"
            placeholder="100.00"
          />
        </div>
        <div>
          <label htmlFor="alert-threshold" className="label">
            {t('budget.alert_threshold')}
          </label>
          <input
            id="alert-threshold"
            type="number"
            min="0"
            max="100"
            step="1"
            value={alertThreshold}
            onChange={(e) => setAlertThreshold(e.target.value)}
            className="input"
          />
          <p className="text-xs text-apple-gray-500 mt-1">{t('budget.alert_threshold_hint')}</p>
        </div>
        <div>
          <label htmlFor="budget-email" className="label">
            {t('budget.notification_email')}
          </label>
          <input
            id="budget-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="input"
            placeholder={t('budget.email_placeholder')}
          />
        </div>
        <div className="flex items-center gap-2 pt-2">
          <input
            id="enforce-hard-limit"
            type="checkbox"
            checked={enforceHardLimit}
            onChange={(e) => setEnforceHardLimit(e.target.checked)}
            className="h-4 w-4 rounded border-apple-gray-300"
          />
          <label htmlFor="enforce-hard-limit" className="text-sm">
            {t('budget.enforce_hard_limit')}
          </label>
        </div>
        <p className="text-xs text-apple-gray-500 -mt-2">
          {t('budget.enforce_hard_limit_hint')}
        </p>
        <div className="pt-4 flex gap-3">
          <button
            onClick={handleSave}
            className="btn btn-primary"
            disabled={saving}
          >
            {saving ? t('common.saving') : t('budget.save')}
          </button>
          {myBudget && (
            <button
              onClick={handleDelete}
              className="btn btn-secondary text-apple-red"
              disabled={deleting}
            >
              {deleting ? t('common.saving') : t('budget.delete')}
            </button>
          )}
        </div>
      </div>
    </motion.div>
  );
}

export default BudgetSettingsCard;
