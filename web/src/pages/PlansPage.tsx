import { useMemo } from 'react';
import { motion } from 'framer-motion';
import { 
  CheckIcon, 
  ArrowPathIcon,
  SparklesIcon,
  RocketLaunchIcon,
  BuildingOffice2Icon
} from '@heroicons/react/24/outline';
import { useQuery } from '@apollo/client/react';
import { PLANS_QUERY, MY_BILLING_QUERY } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

function PlansPage() {
  const { t } = useTranslation();
  const { data: plansData, loading: plansLoading } = useQuery<any>(PLANS_QUERY);
  const { data: billingData, loading: billingLoading } = useQuery<any>(MY_BILLING_QUERY);
  const loading = plansLoading || billingLoading;

  const plans = useMemo(() => (plansData?.plans || []) as any[], [plansData]);
  const subscription = billingData?.mySubscription as any;

  const getPlanIcon = (name: string) => {
    switch (name.toLowerCase()) {
      case 'free': return <SparklesIcon className="w-8 h-8 text-apple-gray-400" />;
      case 'pro': return <RocketLaunchIcon className="w-8 h-8 text-apple-blue" />;
      case 'enterprise': return <BuildingOffice2Icon className="w-8 h-8 text-purple-600" />;
      default: return <SparklesIcon className="w-8 h-8 text-apple-blue" />;
    }
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
    <div className="max-w-6xl mx-auto py-8 px-4 sm:px-6 lg:px-8">
      <div className="text-center mb-16">
        <h1 className="text-4xl font-bold text-apple-gray-900 tracking-tight sm:text-5xl">
          {t('subscription.tab_plans')}
        </h1>
        <p className="mt-4 text-xl text-apple-gray-500">
          {t('subscription.subtitle')}
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        {plans.filter((p: any) => p.isActive).map((plan: any) => {
          const isCurrent = subscription?.planId === plan.id;
          const features = (plan.features || '').split(',').map((f: string) => f.trim()).filter(Boolean);

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
                  {t('subscription.current')}
                </div>
              )}
              
              <div className="p-8 flex-1">
                <div className="mb-6">{getPlanIcon(plan.name)}</div>
                <h3 className="text-2xl font-bold text-apple-gray-900">{plan.name}</h3>
                <p className="mt-2 text-apple-gray-500 text-sm h-10">{plan.description}</p>
                
                <div className="mt-6 flex items-baseline">
                  <span className="text-4xl font-bold text-apple-gray-900">${plan.priceMonth}</span>
                  <span className="ml-1 text-apple-gray-500">{t('subscription.per_month')}</span>
                </div>

                <div className="mt-3 flex gap-4 text-xs text-apple-gray-500">
                  <span>{formatTokens(plan.tokenLimit)} {t('subscription.token_limit')}</span>
                  <span>{plan.rateLimit} {t('subscription.rate_limit_label')}</span>
                </div>

                <ul className="mt-8 space-y-4">
                  {features.map((feature: string, i: number) => (
                    <li key={i} className="flex items-start">
                      <CheckIcon className="w-5 h-5 text-apple-green shrink-0 mr-3" />
                      <span className="text-apple-gray-600 text-sm">{feature}</span>
                    </li>
                  ))}
                </ul>
              </div>

              <div className="p-8 bg-apple-gray-50 border-t border-apple-gray-100">
                <div
                  className={`w-full py-3 px-6 rounded-xl font-semibold text-sm text-center ${
                    isCurrent
                      ? 'bg-apple-gray-200 text-apple-gray-500'
                      : 'bg-apple-blue text-white'
                  }`}
                >
                  {isCurrent ? t('subscription.subscribed') : t('subscription.upgrade')}
                </div>
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
    </div>
  );
}

export default PlansPage;
