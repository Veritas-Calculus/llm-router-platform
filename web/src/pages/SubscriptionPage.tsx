import { useState, useMemo } from 'react';
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
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { PLANS_QUERY, MY_BILLING_QUERY, CREATE_CHECKOUT_SESSION, CREATE_RECHARGE_SESSION, CREATE_PORTAL_SESSION } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';
import { useAuthStore } from '@/stores/authStore';
import toast from 'react-hot-toast';

/* eslint-disable @typescript-eslint/no-explicit-any */

function SubscriptionPage() {
  const { t } = useTranslation();
  const { user } = useAuthStore();
  const { data: plansData, loading: plansLoading } = useQuery<any>(PLANS_QUERY);
  const { data: billingData, loading: billingLoading } = useQuery<any>(MY_BILLING_QUERY);
  const [checkoutMut] = useMutation(CREATE_CHECKOUT_SESSION);
  const [portalMut] = useMutation(CREATE_PORTAL_SESSION);
  const [rechargeMut] = useMutation(CREATE_RECHARGE_SESSION);
  const [processingId, setProcessingId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'plans' | 'orders'>('plans');
  const loading = plansLoading || billingLoading;

  const plans = useMemo(() => (plansData?.plans || []) as any[], [plansData]);
  const subscription = billingData?.mySubscription as any;
  const orders = useMemo(() => (billingData?.myOrders || []) as any[], [billingData]);

  const handleSubscribe = async (planId: string) => {
    try {
      setProcessingId(planId);
      if (subscription?.status === 'active' && subscription?.plan?.priceMonth !== 0) {
        // They already have a paid subscription, send them to the portal to upgrade/downgrade
        const { data } = await portalMut();
        const url = (data as any)?.createPortalSession?.url;
        if (url) window.location.href = url;
        return;
      }
      // Otherwise, create a new checkout session
      const { data } = await checkoutMut({ variables: { planId } });
      const url = (data as any)?.createCheckoutSession?.url;
      if (url) window.location.href = url;
    } catch {
      toast.error(t('subscription.checkout_error'));
    } finally {
      setProcessingId(null);
    }
  };

  const handleRecharge = async (amount: number) => {
    try {
      setProcessingId(`recharge-${amount}`);
      const { data } = await rechargeMut({ variables: { amount } });
      const url = (data as any)?.createRechargeSession?.url;
      if (url) window.location.href = url;
    } catch {
      toast.error(t('subscription.recharge_error'));
    } finally {
      setProcessingId(null);
    }
  };

  const getPlanIcon = (name: string) => {
    switch (name.toLowerCase()) {
      case 'free': return <SparklesIcon className="w-7 h-7 text-apple-gray-400" />;
      case 'pro': return <RocketLaunchIcon className="w-7 h-7 text-apple-blue" />;
      case 'enterprise': return <BuildingOffice2Icon className="w-7 h-7 text-purple-600" />;
      default: return <SparklesIcon className="w-7 h-7 text-apple-blue" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const config: Record<string, { bg: string; text: string; icon: any; label: string }> = {
      paid: { bg: 'bg-green-100', text: 'text-green-800', icon: CheckCircleIcon, label: t('subscription.status_paid') },
      pending: { bg: 'bg-orange-100', text: 'text-orange-800', icon: ClockIcon, label: t('subscription.status_pending') },
      failed: { bg: 'bg-red-100', text: 'text-red-800', icon: XCircleIcon, label: t('subscription.status_failed') },
    };
    const s = config[status] || { bg: 'bg-gray-100', text: 'text-gray-800', icon: ClockIcon, label: status };
    const Icon = s.icon;
    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${s.bg} ${s.text}`}>
        <Icon className="w-3 h-3 mr-1" />
        {s.label}
      </span>
    );
  };

  if (loading) {
    return (
      <div className="flex justify-center py-24">
        <ArrowPathIcon className="w-8 h-8 text-apple-blue animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-apple-gray-900">{t('subscription.title')}</h1>
        <p className="mt-1 text-apple-gray-500">{t('subscription.subtitle')}</p>
      </div>

      {/* Current status cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="card p-5">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
              <CreditCardIcon className="w-5 h-5 text-apple-blue" />
            </div>
            <div>
              <p className="text-xs text-apple-gray-500">{t('subscription.current_plan')}</p>
              <p className="text-lg font-bold text-apple-gray-900">
                {subscription?.planName || t('subscription.no_plan')}
              </p>
            </div>
          </div>
        </div>
        <div className="card p-5">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-green-50 rounded-xl flex items-center justify-center">
              <SparklesIcon className="w-5 h-5 text-green-600" />
            </div>
            <div>
              <p className="text-xs text-apple-gray-500">{t('subscription.balance')}</p>
              <p className="text-lg font-bold text-apple-gray-900">
                ${(user?.balance ?? 0).toFixed(2)}
              </p>
            </div>
          </div>
        </div>
        <div className="card p-5">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-purple-50 rounded-xl flex items-center justify-center">
              <ClockIcon className="w-5 h-5 text-purple-600" />
            </div>
            <div>
              <p className="text-xs text-apple-gray-500">{t('subscription.period')}</p>
              <p className="text-sm font-semibold text-apple-gray-900">
                {subscription?.currentPeriodEnd
                  ? new Date(subscription.currentPeriodEnd).toLocaleDateString()
                  : '—'}
              </p>
            </div>
            {subscription?.status === 'active' && subscription?.plan?.priceMonth !== 0 && (
              <button
                onClick={() => handleSubscribe('portal')}
                disabled={!!processingId}
                className="ml-auto text-xs font-semibold text-apple-blue hover:text-blue-600 bg-blue-50 px-3 py-1.5 rounded-lg active:scale-95 transition-all flex items-center gap-1"
              >
                {processingId === 'portal' ? (
                  <ArrowPathIcon className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  t('subscription.manage') || 'Manage'
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex gap-1 bg-apple-gray-100 rounded-xl p-1 w-fit">
        <button
          onClick={() => setActiveTab('plans')}
          className={`px-5 py-2 rounded-lg text-sm font-semibold transition-all ${
            activeTab === 'plans'
              ? 'bg-white text-apple-blue shadow-sm'
              : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          {t('subscription.tab_plans')}
        </button>
        <button
          onClick={() => setActiveTab('orders')}
          className={`px-5 py-2 rounded-lg text-sm font-semibold transition-all ${
            activeTab === 'orders'
              ? 'bg-white text-apple-blue shadow-sm'
              : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          {t('subscription.tab_orders')}
        </button>
      </div>

      {activeTab === 'plans' ? (
        <>
          {/* Plan cards */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {plans.map((plan: any) => {
              const isCurrent = subscription?.planId === plan.id;
              const features = (plan.features || '').split(',').map((f: string) => f.trim()).filter(Boolean);

              return (
                <motion.div
                  key={plan.id}
                  whileHover={{ y: -4 }}
                  className={`relative card overflow-hidden ${
                    isCurrent ? 'ring-2 ring-apple-blue' : ''
                  }`}
                >
                  {isCurrent && (
                    <div className="absolute top-0 right-0 bg-apple-blue text-white px-3 py-1 text-[10px] font-bold rounded-bl-xl">
                      {t('subscription.current')}
                    </div>
                  )}
                  <div className="p-6">
                    <div className="mb-4">{getPlanIcon(plan.name)}</div>
                    <h3 className="text-xl font-bold text-apple-gray-900">{plan.name}</h3>
                    <p className="mt-1 text-apple-gray-500 text-sm h-10">{plan.description}</p>
                    <div className="mt-4 flex items-baseline">
                      <span className="text-3xl font-bold text-apple-gray-900">${plan.price}</span>
                      <span className="ml-1 text-apple-gray-500 text-sm">/{t('subscription.month')}</span>
                    </div>
                    <ul className="mt-5 space-y-2.5">
                      {features.map((f: string, i: number) => (
                        <li key={i} className="flex items-start text-sm">
                          <CheckIcon className="w-4 h-4 text-apple-green shrink-0 mr-2 mt-0.5" />
                          <span className="text-apple-gray-600">{f}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                  <div className="p-6 bg-apple-gray-50 border-t border-apple-gray-100">
                    <button
                      onClick={() => handleSubscribe(plan.id)}
                      disabled={isCurrent || !!processingId}
                      className={`w-full py-2.5 px-4 rounded-xl font-semibold text-sm transition-all flex justify-center items-center ${
                        isCurrent
                          ? 'bg-apple-gray-200 text-apple-gray-500 cursor-default'
                          : 'bg-apple-blue text-white hover:bg-blue-600 active:scale-95 shadow-sm'
                      }`}
                    >
                      {processingId === plan.id ? (
                        <ArrowPathIcon className="w-5 h-5 animate-spin" />
                      ) : isCurrent ? (
                        t('subscription.subscribed')
                      ) : (subscription?.status === 'active' && subscription?.plan?.priceMonth !== 0) ? (
                        t('subscription.manage') || 'Manage Subscription'
                      ) : plan.price === 0 ? (
                        t('subscription.free_tier')
                      ) : (
                        t('subscription.upgrade')
                      )}
                    </button>
                  </div>
                </motion.div>
              );
            })}
          </div>

          {/* Recharge */}
          <div className="card p-8 text-center">
            <h2 className="text-xl font-bold text-apple-gray-900 mb-2">{t('subscription.recharge_title')}</h2>
            <p className="text-apple-gray-500 text-sm mb-6 max-w-lg mx-auto">{t('subscription.recharge_desc')}</p>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 max-w-2xl mx-auto">
              {[10, 20, 50, 100].map((amount) => (
                <button
                  key={amount}
                  onClick={() => handleRecharge(amount)}
                  disabled={!!processingId}
                  className="bg-apple-gray-50 border border-apple-gray-200 rounded-xl p-4 hover:border-apple-blue hover:bg-blue-50 transition-all active:scale-95 flex flex-col items-center gap-1 group"
                >
                  <span className="text-xl font-bold text-apple-gray-900 group-hover:text-apple-blue">${amount}</span>
                  <span className="text-[10px] text-apple-gray-500 font-medium uppercase tracking-wider">{t('subscription.credits')}</span>
                  {processingId === `recharge-${amount}` && (
                    <ArrowPathIcon className="w-4 h-4 animate-spin text-apple-blue mt-1" />
                  )}
                </button>
              ))}
            </div>
          </div>
        </>
      ) : (
        /* Orders tab */
        <div className="card overflow-hidden">
          {orders.length === 0 ? (
            <div className="p-12 text-center">
              <CreditCardIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-4" />
              <h3 className="text-lg font-medium text-apple-gray-900">{t('subscription.no_orders')}</h3>
              <p className="text-apple-gray-500 text-sm mt-1">{t('subscription.no_orders_desc')}</p>
            </div>
          ) : (
            <table className="min-w-full divide-y divide-apple-gray-200">
              <thead className="bg-apple-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">{t('subscription.order_info')}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">{t('common.status')}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">{t('subscription.amount')}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">{t('common.created_at')}</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-apple-gray-200">
                {orders.map((order: any) => (
                  <tr key={order.id} className="hover:bg-apple-gray-50 transition-colors">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-mono text-apple-gray-900">
                          {order.id.slice(0, 8)}...
                        </span>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(order.id);
                            toast.success(t('common.copied'));
                          }}
                          className="text-apple-gray-400 hover:text-apple-blue"
                        >
                          <DocumentDuplicateIcon className="w-3.5 h-3.5" />
                        </button>
                      </div>
                      <p className="text-xs text-apple-gray-500">{order.description || '—'}</p>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">{getStatusBadge(order.status)}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-semibold text-apple-gray-900">
                      ${order.amount.toFixed(2)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-apple-gray-500">
                      {new Date(order.createdAt).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}

export default SubscriptionPage;
