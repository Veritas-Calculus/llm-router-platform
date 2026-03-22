import { useState, useMemo } from 'react';
import { motion } from 'framer-motion';
import { 
  CheckIcon, 
  ArrowPathIcon,
  SparklesIcon,
  RocketLaunchIcon,
  BuildingOffice2Icon
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { PLANS_QUERY, MY_BILLING_QUERY, CREATE_CHECKOUT_SESSION, CREATE_RECHARGE_SESSION, CREATE_PORTAL_SESSION } from '@/lib/graphql/operations';
import type { Plan } from '@/lib/types';
import toast from 'react-hot-toast';

/* eslint-disable @typescript-eslint/no-explicit-any */

function PlansPage() {
  const { data: plansData, loading: plansLoading } = useQuery<any>(PLANS_QUERY);
  const { data: billingData, loading: billingLoading } = useQuery<any>(MY_BILLING_QUERY);
  const [checkoutMut] = useMutation(CREATE_CHECKOUT_SESSION);
  const [portalMut] = useMutation(CREATE_PORTAL_SESSION);
  const [rechargeMut] = useMutation(CREATE_RECHARGE_SESSION);
  const [processingId, setProcessingId] = useState<string | null>(null);
  const loading = plansLoading || billingLoading;

  const plans: Plan[] = useMemo(() =>
    (plansData?.plans || []).map((p: any) => ({
      id: p.id, name: p.name, description: p.description,
      price_month: p.price, features: (p.features || []).join(', '),
      token_limit: 0, rate_limit: 0, is_active: p.isActive,
    })),
  [plansData]);
  const subscription = useMemo(() => {
    const s = billingData?.mySubscription;
    if (!s) return null;
    return { id: s.id, plan_id: s.planId, plan_name: s.planName, status: s.status, current_period_start: s.currentPeriodStart, current_period_end: s.currentPeriodEnd } as any;
  }, [billingData]);

  const handleSubscribe = async (planId: string) => {
    try {
      setProcessingId(planId);
      if (subscription?.status === 'active' && subscription?.plan_name !== 'Free') {
        // Already have a paid sub
        const { data } = await portalMut();
        const url = (data as any)?.createPortalSession?.url;
        if (url) window.location.href = url;
        return;
      }
      
      const { data } = await checkoutMut({ variables: { planId } });
      const url = (data as any)?.createCheckoutSession?.url;
      if (url) window.location.href = url;
    } catch {
      toast.error('Failed to initiate checkout');
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
      toast.error('Failed to initiate recharge');
    } finally {
      setProcessingId(null);
    }
  };

  const getPlanIcon = (name: string) => {
    switch (name.toLowerCase()) {
      case 'free': return <SparklesIcon className="w-8 h-8 text-apple-gray-400" />;
      case 'pro': return <RocketLaunchIcon className="w-8 h-8 text-apple-blue" />;
      case 'enterprise': return <BuildingOffice2Icon className="w-8 h-8 text-purple-600" />;
      default: return <SparklesIcon className="w-8 h-8 text-apple-blue" />;
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-24">
        <ArrowPathIcon className="w-8 h-8 text-apple-blue animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto py-8 px-4 sm:px-6 lg:px-8">
      <div className="text-center mb-16">
        <h1 className="text-4xl font-bold text-apple-gray-900 tracking-tight sm:text-5xl">
          Simple, transparent pricing
        </h1>
        <p className="mt-4 text-xl text-apple-gray-500">
          Choose the plan that's right for you and your team
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        {plans.map((plan) => {
          const isCurrent = subscription?.plan_id === plan.id;
          const features = plan.features.split(',').map(f => f.trim());

          return (
            <motion.div
              key={plan.id}
              whileHover={{ y: -5 }}
              className={`relative bg-white rounded-3xl border ${
                isCurrent ? 'border-apple-blue ring-1 ring-apple-blue shadow-apple-lg' : 'border-apple-gray-200 shadow-sm'
              } flex flex-col overflow-hidden`}
            >
              {isCurrent && (
                <div className="absolute top-0 right-0 bg-apple-blue text-white px-4 py-1 text-xs font-bold rounded-bl-xl">
                  CURRENT PLAN
                </div>
              )}
              
              <div className="p-8 flex-1">
                <div className="mb-6">{getPlanIcon(plan.name)}</div>
                <h3 className="text-2xl font-bold text-apple-gray-900">{plan.name}</h3>
                <p className="mt-2 text-apple-gray-500 text-sm h-10">{plan.description}</p>
                
                <div className="mt-6 flex items-baseline">
                  <span className="text-4xl font-bold text-apple-gray-900">${plan.price_month}</span>
                  <span className="ml-1 text-apple-gray-500">/month</span>
                </div>

                <ul className="mt-8 space-y-4">
                  {features.map((feature, i) => (
                    <li key={i} className="flex items-start">
                      <CheckIcon className="w-5 h-5 text-apple-green shrink-0 mr-3" />
                      <span className="text-apple-gray-600 text-sm">{feature}</span>
                    </li>
                  ))}
                  <li className="flex items-start">
                    <CheckIcon className="w-5 h-5 text-apple-green shrink-0 mr-3" />
                    <span className="text-apple-gray-600 text-sm">
                      {plan.token_limit === 0 ? 'Unlimited tokens' : `${(plan.token_limit / 1000000).toFixed(1)}M tokens per month`}
                    </span>
                  </li>
                  <li className="flex items-start">
                    <CheckIcon className="w-5 h-5 text-apple-green shrink-0 mr-3" />
                    <span className="text-apple-gray-600 text-sm">{plan.rate_limit} requests per minute</span>
                  </li>
                </ul>
              </div>

              <div className="p-8 bg-apple-gray-50 border-t border-apple-gray-100">
                {plan.price_month === 0 ? (
                  <button
                    disabled={isCurrent}
                    className={`w-full py-3 px-6 rounded-xl font-semibold text-sm transition-all ${
                      isCurrent 
                        ? 'bg-apple-gray-200 text-apple-gray-500 cursor-default'
                        : 'bg-white border border-apple-gray-200 text-apple-gray-900 hover:bg-apple-gray-50 active:scale-95'
                    }`}
                  >
                    {isCurrent ? 'Default Plan' : 'Free Trial'}
                  </button>
                ) : (
                  <button
                    onClick={() => handleSubscribe(plan.id)}
                    disabled={isCurrent || processingId === plan.id}
                    className={`w-full py-3 px-6 rounded-xl font-semibold text-sm transition-all flex justify-center items-center ${
                      isCurrent
                        ? 'bg-apple-gray-200 text-apple-gray-500 cursor-default'
                        : 'bg-apple-blue text-white hover:bg-blue-600 active:scale-95 shadow-md hover:shadow-lg'
                    }`}
                  >
                    {processingId === plan.id ? (
                      <ArrowPathIcon className="w-5 h-5 animate-spin" />
                    ) : isCurrent ? (
                      'Subscribed'
                    ) : (subscription?.status === 'active' && subscription?.plan_name !== 'Free') ? (
                      'Manage Subscription'
                    ) : (
                      'Upgrade Now'
                    )}
                  </button>
                )}
              </div>
            </motion.div>
          );
        })}
      </div>

      <div className="mt-16 bg-blue-50 rounded-3xl p-8 border border-blue-100">
        <div className="flex flex-col md:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-4 text-left">
            <div className="p-3 bg-white rounded-2xl shadow-sm">
              <BuildingOffice2Icon className="w-8 h-8 text-apple-blue" />
            </div>
            <div>
              <h4 className="text-xl font-bold text-apple-gray-900">Need more power?</h4>
              <p className="text-apple-gray-600">Contact us for custom enterprise quotas and dedicated support.</p>
            </div>
          </div>
          <button className="apple-button-primary whitespace-nowrap px-8">
            Contact Sales
          </button>
        </div>
      </div>

      <div className="mt-16 bg-apple-gray-50 rounded-3xl p-12 border border-apple-gray-200 text-center">
        <h2 className="text-3xl font-bold text-apple-gray-900 mb-4">Pay-as-you-go Credits</h2>
        <p className="text-apple-gray-500 mb-8 max-w-2xl mx-auto">
          Not ready for a subscription? Top up your balance and pay only for what you use. 
          Credits never expire and work across all models.
        </p>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 max-w-4xl mx-auto">
          {[10, 20, 50, 100].map((amount) => (
            <button
              key={amount}
              onClick={() => handleRecharge(amount)}
              disabled={!!processingId}
              className="bg-white border border-apple-gray-200 rounded-2xl p-6 hover:border-apple-blue hover:shadow-apple-sm transition-all active:scale-95 flex flex-col items-center gap-2 group"
            >
              <span className="text-2xl font-bold text-apple-gray-900 group-hover:text-apple-blue transition-colors">${amount}</span>
              <span className="text-xs text-apple-gray-500 font-medium uppercase tracking-wider">Credits</span>
              {processingId === `recharge-${amount}` ? (
                <ArrowPathIcon className="w-4 h-4 animate-spin text-apple-blue mt-2" />
              ) : (
                <span className="text-xs text-apple-blue font-semibold mt-2 opacity-0 group-hover:opacity-100 transition-opacity">Select</span>
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

export default PlansPage;
