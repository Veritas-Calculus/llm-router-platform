import { useState } from 'react';
import { useTranslation } from '@/lib/i18n';
import RedeemCodesPage from './RedeemCodesPage';
import CouponsPage from './CouponsPage';

const tabs = [
  { key: 'redeem', labelKey: 'nav.redeem_codes' },
  { key: 'coupons', labelKey: 'nav.coupons' },
] as const;

type TabKey = (typeof tabs)[number]['key'];

export default function PromotionsPage() {
  const { t } = useTranslation();
  const [active, setActive] = useState<TabKey>('redeem');

  return (
    <div className="space-y-6">
      {/* Tab bar */}
      <div className="flex items-center gap-1 bg-apple-gray-100 p-1 rounded-xl w-fit border border-apple-gray-200">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActive(tab.key)}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
              active === tab.key
                ? 'bg-white text-apple-blue shadow-sm'
                : 'text-apple-gray-500 hover:text-apple-gray-700'
            }`}
          >
            {t(tab.labelKey)}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {active === 'redeem' && <RedeemCodesPage />}
      {active === 'coupons' && <CouponsPage />}
    </div>
  );
}
