import { useState, useMemo, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  CheckIcon,
  ArrowPathIcon,
  SparklesIcon,
  RocketLaunchIcon,
  BuildingOffice2Icon,
  CreditCardIcon,
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon,
  DocumentDuplicateIcon,
  GiftIcon,
  PlusIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation, useLazyQuery } from '@apollo/client/react';
import { PLANS_QUERY, MY_BILLING_QUERY, CHANGE_PLAN, CREATE_CHECKOUT_SESSION } from '@/lib/graphql/operations';
import { MY_BALANCE } from '@/lib/graphql/operations/auth';
import { MY_REDEEM_HISTORY, REDEEM_CODE_MUTATION } from '@/lib/graphql/operations/redeem';
import { useTranslation } from '@/lib/i18n';
import { useAuthStore } from '@/stores/authStore';
import { formatUSD, moneyNumber } from '@/lib/format';
import toast from 'react-hot-toast';
import RechargeModal from '@/components/RechargeModal';

/* eslint-disable @typescript-eslint/no-explicit-any */

const getErrorMessage = (error: unknown) => {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return '';
};

function SubscriptionPage() {
  const { t } = useTranslation();
  const { user, updateUser } = useAuthStore();
  const { data: plansData, loading: plansLoading } = useQuery<any>(PLANS_QUERY);
  const { data: billingData, loading: billingLoading, refetch: refetchBilling } = useQuery<any>(MY_BILLING_QUERY);
  const { data: redeemHistoryData, loading: redeemHistoryLoading, refetch: refetchRedeemHistory } = useQuery<any>(MY_REDEEM_HISTORY);
  const [refetchBalance] = useLazyQuery<any>(MY_BALANCE, { fetchPolicy: 'network-only' });
  const [changePlanMut] = useMutation(CHANGE_PLAN);
  const [createCheckoutSession] = useMutation(CREATE_CHECKOUT_SESSION);
  const [redeemMut] = useMutation(REDEEM_CODE_MUTATION);
  const [processingId, setProcessingId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'plans' | 'orders' | 'redeems'>('plans');
  const [redeemCode, setRedeemCode] = useState('');
  const [redeeming, setRedeeming] = useState(false);
  const [isRechargeModalOpen, setIsRechargeModalOpen] = useState(false);
  const loading = plansLoading || billingLoading || redeemHistoryLoading;

  // Refresh balance + billing on:
  //   1. Returning to the tab after a Stripe redirect (window focus event).
  //   2. Landing on this page with ?payment=success (Stripe success URL).
  // The user's balance lives in zustand auth state, so we explicitly fetch
  // ME { balance } and merge it back. Without this, the balance card stays
  // stale until the user logs out and back in.
  useEffect(() => {
    const refresh = async () => {
      try {
        const result = await refetchBalance();
        const fresh = (result.data as any)?.me;
        if (fresh && user) {
          updateUser({ ...user, balance: fresh.balance });
        }
      } catch {
        // best-effort — the existing user.balance stays as is
      }
      refetchBilling();
      refetchRedeemHistory();
    };

    // URL-based refresh on first mount.
    const params = new URLSearchParams(window.location.search);
    if (params.get('payment') === 'success' || params.get('checkout') === 'success') {
      refresh();
      toast.success(t('subscription.payment_success'));
      // Clean the URL so a manual reload doesn't re-toast.
      params.delete('payment');
      params.delete('checkout');
      params.delete('order_no');
      const newSearch = params.toString();
      window.history.replaceState(
        {},
        '',
        window.location.pathname + (newSearch ? `?${newSearch}` : '')
      );
    }

    // Focus-based refresh (covers Stripe redirect-back without success param).
    window.addEventListener('focus', refresh);
    return () => window.removeEventListener('focus', refresh);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const plans = useMemo(() => (plansData?.plans || []) as any[], [plansData]);
  const subscription = billingData?.mySubscription as any;
  const orders = useMemo(() => (billingData?.myOrders || []) as any[], [billingData]);
  const redeemHistory = useMemo(() => (redeemHistoryData?.myRedeemHistory || []) as any[], [redeemHistoryData]);
  const activePlans = useMemo(() => plans.filter((p: any) => p.isActive), [plans]);
  const currentPlan = useMemo(
    () => activePlans.find((p: any) => p.id === subscription?.planId),
    [activePlans, subscription?.planId]
  );
  const currentPlanPrice = moneyNumber(currentPlan?.priceMonth);

  const refreshUserBalance = async () => {
    try {
      const result = await refetchBalance();
      const fresh = (result.data as any)?.me;
      if (fresh && user) {
        updateUser({ ...user, balance: fresh.balance });
      }
      return moneyNumber(fresh?.balance ?? user?.balance);
    } catch {
      return moneyNumber(user?.balance);
    }
  };

  const planChangeErrorMessage = (error: unknown, fallback: string) => {
    const message = getErrorMessage(error);
    if (message.includes('insufficient balance')) {
      return t('subscription.insufficient_balance');
    }
    if (message.includes('payments are currently disabled')) {
      return t('subscription.payment_disabled');
    }
    if (message.includes('plan is not available')) {
      return t('subscription.plan_unavailable');
    }
    if (message.includes('plan not found')) {
      return t('subscription.plan_not_found');
    }
    if (message.includes('checkout session missing url')) {
      return t('subscription.checkout_missing');
    }
    return message || fallback;
  };

  const changePlanWithBalance = async (planId: string) => {
    const { data, error } = await changePlanMut({
      variables: { planId },
      refetchQueries: [{ query: MY_BILLING_QUERY }],
      awaitRefetchQueries: true,
    });
    if (error) throw error;
    if (!(data as any)?.changePlan) throw new Error('plan change missing subscription');
    await refreshUserBalance();
  };

  const handleChangePlan = async (plan: any) => {
    try {
      setProcessingId(plan.id);
      const priceMonth = moneyNumber(plan.priceMonth);
      if (priceMonth > 0) {
        const balance = await refreshUserBalance();
        if (balance >= priceMonth) {
          await changePlanWithBalance(plan.id);
          toast.success(t('subscription.change_success'));
          return;
        }

        const { data, error } = await createCheckoutSession({ variables: { planId: plan.id } });
        if (error) throw error;
        const url = (data as any)?.createCheckoutSession?.url;
        if (!url) throw new Error('checkout session missing url');
        window.location.href = url;
        return;
      }

      await changePlanWithBalance(plan.id);
      toast.success(t('subscription.change_success'));
    } catch (error) {
      const fallback = moneyNumber(plan.priceMonth) > 0 ? t('subscription.checkout_error') : t('subscription.change_error');
      toast.error(planChangeErrorMessage(error, fallback));
    } finally {
      setProcessingId(null);
    }
  };

  const handleRedeem = async () => {
    if (!redeemCode.trim()) return;
    try {
      setRedeeming(true);
      const { data, error } = await redeemMut({ variables: { code: redeemCode.trim() } });
      if (error) throw error;
      const result = (data as any)?.redeemCode;
      if (result?.success) {
        toast.success(result.message || t('redeem.success_msg'));
        setRedeemCode('');
        await Promise.all([refreshUserBalance(), refetchBilling(), refetchRedeemHistory()]);
      } else {
        toast.error(result?.message || t('redeem.error_msg'));
      }
    } catch {
      toast.error(t('redeem.error_msg'));
    } finally {
      setRedeeming(false);
    }
  };

  const getPlanIcon = (name: string) => {
    switch (name.toLowerCase()) {
      case 'free': return <SparklesIcon className="w-5 h-5 text-apple-gray-500 dark:text-apple-gray-300" />;
      case 'pro': return <RocketLaunchIcon className="w-5 h-5 text-apple-blue" />;
      case 'enterprise': return <BuildingOffice2Icon className="w-5 h-5 text-purple-600 dark:text-purple-400" />;
      default: return <SparklesIcon className="w-5 h-5 text-apple-blue" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const config: Record<string, { bg: string; text: string; icon: any; label: string }> = {
      paid: { bg: 'bg-green-100 dark:bg-green-500/15', text: 'text-green-800 dark:text-green-300', icon: CheckCircleIcon, label: t('subscription.status_paid') },
      pending: { bg: 'bg-orange-100 dark:bg-orange-500/15', text: 'text-orange-800 dark:text-orange-300', icon: ClockIcon, label: t('subscription.status_pending') },
      failed: { bg: 'bg-red-100 dark:bg-red-500/15', text: 'text-red-800 dark:text-red-300', icon: XCircleIcon, label: t('subscription.status_failed') },
    };
    const s = config[status] || { bg: 'bg-gray-100 dark:bg-white/10', text: 'text-gray-800 dark:text-apple-gray-300', icon: ClockIcon, label: status };
    const Icon = s.icon;
    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${s.bg} ${s.text}`}>
        <Icon className="w-3 h-3 mr-1" />
        {s.label}
      </span>
    );
  };

  const formatTokens = (n: number) => {
    if (n === 0) return '∞';
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
    return String(n);
  };

  if (loading) {
    return (
      <div className="flex justify-center py-24">
        <ArrowPathIcon className="w-8 h-8 text-apple-blue animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-7xl space-y-5 pb-8">
      <div>
        <h1 className="text-2xl font-bold text-apple-gray-900 dark:text-white">{t('subscription.title')}</h1>
        <p className="mt-1 text-sm text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.subtitle')}</p>
      </div>

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-[1fr_1.25fr_1fr]">
        <div className="rounded-2xl border border-apple-gray-200 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-50 dark:bg-blue-500/15">
              <CreditCardIcon className="h-5 w-5 text-apple-blue" />
            </div>
            <div className="min-w-0">
              <p className="text-xs font-medium text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.current_plan')}</p>
              <p className="mt-0.5 truncate text-lg font-semibold text-apple-gray-900 dark:text-white">
                {subscription?.planName || t('subscription.no_plan')}
              </p>
            </div>
          </div>
        </div>

        <div className="rounded-2xl border border-apple-gray-200 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-green-50 dark:bg-green-500/15">
                <SparklesIcon className="h-5 w-5 text-green-600 dark:text-green-400" />
              </div>
              <div className="min-w-0">
                <p className="text-xs font-medium text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.balance')}</p>
                <p className="mt-0.5 truncate text-xl font-semibold text-apple-gray-900 dark:text-white">
                  {formatUSD(user?.balance ?? 0)}
                </p>
              </div>
            </div>
            <button
              onClick={() => setIsRechargeModalOpen(true)}
              className="inline-flex h-10 shrink-0 items-center justify-center gap-1.5 rounded-xl bg-green-500 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-green-600 active:bg-green-700"
              title={t('subscription.top_up')}
            >
              <PlusIcon className="h-4 w-4" />
              {t('subscription.top_up')}
            </button>
          </div>
        </div>

        <div className="rounded-2xl border border-apple-gray-200 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-purple-50 dark:bg-purple-500/15">
              <ClockIcon className="h-5 w-5 text-purple-600 dark:text-purple-400" />
            </div>
            <div className="min-w-0">
              <p className="text-xs font-medium text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.period')}</p>
              <p className="mt-0.5 truncate text-lg font-semibold text-apple-gray-900 dark:text-white">
                {subscription?.currentPeriodEnd
                  ? new Date(subscription.currentPeriodEnd).toLocaleDateString()
                  : '--'}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="inline-flex max-w-full overflow-x-auto rounded-2xl border border-apple-gray-200 bg-apple-gray-100 p-1 dark:border-white/10 dark:bg-white/5">
        <button
          onClick={() => setActiveTab('plans')}
          className={`h-10 shrink-0 rounded-xl px-5 text-sm font-semibold transition-all ${
            activeTab === 'plans'
              ? 'bg-white text-apple-blue shadow-sm dark:bg-[#2C2C2E]'
              : 'text-apple-gray-500 hover:text-apple-gray-700 dark:text-apple-gray-400 dark:hover:text-white'
          }`}
        >
          {t('subscription.tab_plans')}
        </button>
        <button
          onClick={() => setActiveTab('orders')}
          className={`h-10 shrink-0 rounded-xl px-5 text-sm font-semibold transition-all ${
            activeTab === 'orders'
              ? 'bg-white text-apple-blue shadow-sm dark:bg-[#2C2C2E]'
              : 'text-apple-gray-500 hover:text-apple-gray-700 dark:text-apple-gray-400 dark:hover:text-white'
          }`}
        >
          {t('subscription.tab_orders')}
        </button>
        <button
          onClick={() => setActiveTab('redeems')}
          className={`h-10 shrink-0 rounded-xl px-5 text-sm font-semibold transition-all ${
            activeTab === 'redeems'
              ? 'bg-white text-apple-blue shadow-sm dark:bg-[#2C2C2E]'
              : 'text-apple-gray-500 hover:text-apple-gray-700 dark:text-apple-gray-400 dark:hover:text-white'
          }`}
        >
          {t('subscription.tab_redeems')}
        </button>
      </div>

      {activeTab === 'plans' && (
        <>
          {activePlans.length === 0 ? (
            <div className="rounded-2xl border border-apple-gray-200 bg-white p-10 text-center shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
              <CreditCardIcon className="mx-auto h-10 w-10 text-apple-gray-300 dark:text-apple-gray-500" />
              <h3 className="mt-3 text-base font-semibold text-apple-gray-900 dark:text-white">{t('subscription.no_plan')}</h3>
            </div>
          ) : (
            <div className={activePlans.length === 1
              ? 'grid max-w-md grid-cols-1 gap-4'
              : 'grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'}
            >
              {activePlans.map((plan: any) => {
                const isCurrent = subscription?.planId === plan.id;
                const features = (plan.features || '').split(',').map((f: string) => f.trim()).filter(Boolean);
                const priceMonth = moneyNumber(plan.priceMonth);
                const isUpgrade = priceMonth > currentPlanPrice;

                return (
                  <motion.div
                    key={plan.id}
                    whileHover={isCurrent ? undefined : { y: -3 }}
                    className={`relative overflow-hidden rounded-2xl border bg-white p-5 shadow-sm transition-colors dark:bg-[#1C1C1E] ${
                      isCurrent
                        ? 'border-apple-blue shadow-[0_0_0_1px_rgba(0,122,255,0.45)]'
                        : 'border-apple-gray-200 hover:border-apple-gray-300 dark:border-white/10 dark:hover:border-white/20'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-apple-gray-100 dark:bg-white/10">
                        {getPlanIcon(plan.name)}
                      </div>
                      {isCurrent && (
                        <span className="rounded-full bg-apple-blue px-2.5 py-1 text-xs font-semibold text-white">
                          {t('subscription.current')}
                        </span>
                      )}
                    </div>

                    <div className="mt-4">
                      <h3 className="text-xl font-semibold text-apple-gray-900 dark:text-white">{plan.name}</h3>
                      {plan.description && (
                        <p className="mt-1 text-sm leading-5 text-apple-gray-500 dark:text-apple-gray-400">
                          {plan.description}
                        </p>
                      )}
                    </div>

                    <div className="mt-5 flex items-end gap-1">
                      <span className="text-4xl font-semibold tracking-normal text-apple-gray-900 dark:text-white">
                        {formatUSD(plan.priceMonth)}
                      </span>
                      <span className="pb-1 text-sm text-apple-gray-500 dark:text-apple-gray-400">
                        {t('subscription.per_month')}
                      </span>
                    </div>

                    <div className="mt-5 grid grid-cols-2 gap-2">
                      <div className="rounded-xl border border-apple-gray-200 bg-apple-gray-50 p-3 dark:border-white/10 dark:bg-white/5">
                        <p className="text-xs text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.token_limit')}</p>
                        <p className="mt-1 text-sm font-semibold text-apple-gray-900 dark:text-white">{formatTokens(plan.tokenLimit)}</p>
                      </div>
                      <div className="rounded-xl border border-apple-gray-200 bg-apple-gray-50 p-3 dark:border-white/10 dark:bg-white/5">
                        <p className="text-xs text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.rate_limit_label')}</p>
                        <p className="mt-1 text-sm font-semibold text-apple-gray-900 dark:text-white">{plan.rateLimit}</p>
                      </div>
                    </div>

                    {features.length > 0 && (
                      <ul className="mt-5 space-y-2 border-t border-apple-gray-100 pt-4 dark:border-white/10">
                        {features.map((f: string, i: number) => (
                          <li key={i} className="flex items-start text-sm">
                            <CheckIcon className="mr-2 mt-0.5 h-4 w-4 shrink-0 text-apple-green" />
                            <span className="text-apple-gray-600 dark:text-apple-gray-300">{f}</span>
                          </li>
                        ))}
                      </ul>
                    )}

                    <button
                      onClick={() => handleChangePlan(plan)}
                      disabled={isCurrent || !!processingId}
                      className={`mt-5 flex h-11 w-full items-center justify-center gap-2 rounded-xl px-4 text-sm font-semibold transition-all ${
                        isCurrent
                          ? 'cursor-default bg-apple-gray-100 text-apple-gray-500 dark:bg-white/10 dark:text-apple-gray-400'
                          : 'bg-apple-blue text-white shadow-sm hover:bg-blue-600 active:scale-[0.98]'
                      }`}
                    >
                      {processingId === plan.id ? (
                        <ArrowPathIcon className="h-5 w-5 animate-spin" />
                      ) : isCurrent ? (
                        t('subscription.subscribed')
                      ) : isUpgrade ? (
                        t('subscription.upgrade')
                      ) : (
                        t('subscription.downgrade')
                      )}
                    </button>
                  </motion.div>
                );
              })}
            </div>
          )}

          <div className="rounded-2xl border border-apple-gray-200 bg-white p-5 shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-50 dark:bg-blue-500/15">
                  <GiftIcon className="h-5 w-5 text-apple-blue" />
                </div>
                <div className="min-w-0">
                  <h2 className="text-base font-semibold text-apple-gray-900 dark:text-white">{t('subscription.redeem_code')}</h2>
                  <p className="mt-0.5 text-sm text-apple-gray-500 dark:text-apple-gray-400">{t('redeem.input_desc')}</p>
                </div>
              </div>
              <div className="flex w-full flex-col gap-3 sm:flex-row lg:max-w-xl">
                <input
                  type="text"
                  value={redeemCode}
                  onChange={(e) => setRedeemCode(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleRedeem()}
                  placeholder={t('subscription.redeem_placeholder')}
                  className="h-11 min-w-0 flex-1 rounded-xl border border-apple-gray-200 bg-white px-4 text-sm font-mono tracking-wider text-apple-gray-900 transition-shadow focus:border-transparent focus:outline-none focus:ring-2 focus:ring-apple-blue dark:border-white/10 dark:bg-[#111113] dark:text-white"
                />
                <button
                  onClick={handleRedeem}
                  disabled={!redeemCode.trim() || redeeming}
                  className="inline-flex h-11 items-center justify-center gap-2 rounded-xl bg-apple-blue px-5 text-sm font-semibold text-white transition-all hover:bg-blue-600 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {redeeming ? <ArrowPathIcon className="h-4 w-4 animate-spin" /> : <GiftIcon className="h-4 w-4" />}
                  {t('subscription.redeem_btn')}
                </button>
              </div>
            </div>
          </div>
        </>
      )}

      {activeTab === 'orders' && (
        <div className="overflow-hidden rounded-2xl border border-apple-gray-200 bg-white shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
          {orders.length === 0 ? (
            <div className="p-12 text-center">
              <CreditCardIcon className="mx-auto mb-4 h-12 w-12 text-apple-gray-300 dark:text-apple-gray-500" />
              <h3 className="text-lg font-medium text-apple-gray-900 dark:text-white">{t('subscription.no_orders')}</h3>
              <p className="mt-1 text-sm text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.no_orders_desc')}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-apple-gray-200 dark:divide-white/10">
                <thead className="bg-apple-gray-50 dark:bg-white/5">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.order_info')}</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('common.status')}</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.amount')}</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('common.created_at')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-apple-gray-200 bg-white dark:divide-white/10 dark:bg-[#1C1C1E]">
                  {orders.map((order: any) => (
                    <tr key={order.id} className="transition-colors hover:bg-apple-gray-50 dark:hover:bg-white/5">
                      <td className="whitespace-nowrap px-6 py-4">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-mono text-apple-gray-900 dark:text-white">
                            {order.id.slice(0, 8)}...
                          </span>
                          <button
                            onClick={() => {
                              navigator.clipboard.writeText(order.id);
                              toast.success(t('common.copied'));
                            }}
                            className="text-apple-gray-400 transition-colors hover:text-apple-blue"
                          >
                            <DocumentDuplicateIcon className="h-3.5 w-3.5" />
                          </button>
                        </div>
                        <p className="text-xs text-apple-gray-500 dark:text-apple-gray-400">{order.description || '--'}</p>
                      </td>
                      <td className="whitespace-nowrap px-6 py-4">{getStatusBadge(order.status)}</td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm font-semibold text-apple-gray-900 dark:text-white">
                        {formatUSD(order.amount)}
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-apple-gray-500 dark:text-apple-gray-400">
                        {new Date(order.createdAt).toLocaleDateString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {activeTab === 'redeems' && (
        <div className="overflow-hidden rounded-2xl border border-apple-gray-200 bg-white shadow-sm dark:border-white/10 dark:bg-[#1C1C1E]">
          {redeemHistory.length === 0 ? (
            <div className="p-12 text-center">
              <GiftIcon className="mx-auto mb-4 h-12 w-12 text-apple-gray-300 dark:text-apple-gray-500" />
              <h3 className="text-lg font-medium text-apple-gray-900 dark:text-white">{t('subscription.no_redeems')}</h3>
              <p className="mt-1 text-sm text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.no_redeems_desc')}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-apple-gray-200 dark:divide-white/10">
                <thead className="bg-apple-gray-50 dark:bg-white/5">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.redeem_info')}</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.redeem_type')}</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.redeem_amount')}</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-apple-gray-500 dark:text-apple-gray-400">{t('subscription.redeemed_at')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-apple-gray-200 bg-white dark:divide-white/10 dark:bg-[#1C1C1E]">
                  {redeemHistory.map((record: any) => {
                    const creditAmount = moneyNumber(record.creditAmount);
                    const label = record.planName || t('subscription.credits');
                    return (
                      <tr key={record.id} className="transition-colors hover:bg-apple-gray-50 dark:hover:bg-white/5">
                        <td className="whitespace-nowrap px-6 py-4">
                          <div className="flex items-center gap-2">
                            <span className="text-sm font-mono text-apple-gray-900 dark:text-white">{record.code}</span>
                            <button
                              onClick={() => {
                                navigator.clipboard.writeText(record.code);
                                toast.success(t('common.copied'));
                              }}
                              className="text-apple-gray-400 transition-colors hover:text-apple-blue"
                              title={t('common.copy')}
                            >
                              <DocumentDuplicateIcon className="h-3.5 w-3.5" />
                            </button>
                          </div>
                        </td>
                        <td className="whitespace-nowrap px-6 py-4">
                          <span className="inline-flex items-center rounded-full bg-blue-50 px-2.5 py-1 text-xs font-semibold text-apple-blue dark:bg-blue-500/15">
                            {label}
                          </span>
                        </td>
                        <td className="whitespace-nowrap px-6 py-4 text-sm font-semibold text-green-600 dark:text-green-400">
                          {creditAmount > 0 ? `+${formatUSD(creditAmount)}` : '--'}
                        </td>
                        <td className="whitespace-nowrap px-6 py-4 text-sm text-apple-gray-500 dark:text-apple-gray-400">
                          {record.redeemedAt ? new Date(record.redeemedAt).toLocaleString() : '--'}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      <RechargeModal 
        isOpen={isRechargeModalOpen} 
        onClose={() => setIsRechargeModalOpen(false)} 
      />
    </div>
  );
}

export default SubscriptionPage;
