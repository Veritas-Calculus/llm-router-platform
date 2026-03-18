import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { 
  CreditCardIcon, 
  SparklesIcon, 
  PlusIcon,
  TrashIcon
} from '@heroicons/react/24/outline';
import { plansApi, api, Plan, getApiErrorMessage } from '@/lib/api';
import toast from 'react-hot-toast';

function AdminSettingsPage() {
  const [plans, setPlans] = useState<Plan[]>([]);
  const [, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'plans' | 'payment'>('plans');
  
  // Payment Settings State
  const [paymentSettings, setPaymentSettings] = useState({
    stripe_enabled: 'false',
    stripe_publishable_key: '',
    stripe_secret_key: '',
    stripe_webhook_secret: ''
  });

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        if (activeTab === 'plans') {
          const response = await plansApi.list();
          setPlans(response.data);
        } else {
          const response = await api.get<{ data: Array<{ key: string; value: string }> }>('/api/v1/admin/settings?category=payment');
          const settingsMap: Record<string, string> = {};
          response.data.forEach(s => {
            settingsMap[s.key] = s.value;
          });
          setPaymentSettings(prev => ({ ...prev, ...settingsMap }));
        }
      } catch {
        // toast.error('Failed to fetch settings');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [activeTab]);

  const handleUpdatePayment = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const promises = Object.entries(paymentSettings).map(([key, value]) => 
        api.post('/api/v1/admin/settings', {
          key,
          value,
          category: 'payment',
          is_secret: key.includes('secret') || key.includes('key')
        })
      );
      await Promise.all(promises);
      toast.success('Payment settings updated');
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'Update failed'));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">System Settings</h1>
          <p className="text-apple-gray-500">Configure pricing, payments, and platform behavior</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex space-x-1 bg-apple-gray-100 p-1 rounded-xl w-fit">
        <button
          onClick={() => setActiveTab('plans')}
          className={`px-4 py-2 text-sm font-medium rounded-lg transition-all ${
            activeTab === 'plans' ? 'bg-white text-apple-blue shadow-sm' : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          Pricing Plans
        </button>
        <button
          onClick={() => setActiveTab('payment')}
          className={`px-4 py-2 text-sm font-medium rounded-lg transition-all ${
            activeTab === 'payment' ? 'bg-white text-apple-blue shadow-sm' : 'text-apple-gray-500 hover:text-apple-gray-700'
          }`}
        >
          Payment Channels
        </button>
      </div>

      {activeTab === 'plans' ? (
        <div className="space-y-4">
          <div className="flex justify-end">
            <button className="apple-button-primary flex items-center text-sm">
              <PlusIcon className="w-4 h-4 mr-2" />
              New Plan
            </button>
          </div>
          <div className="grid grid-cols-1 gap-4">
            {plans.map((plan) => (
              <div key={plan.id} className="card flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="p-3 bg-blue-50 rounded-xl text-apple-blue">
                    <SparklesIcon className="w-6 h-6" />
                  </div>
                  <div>
                    <h3 className="font-bold text-apple-gray-900">{plan.name}</h3>
                    <p className="text-sm text-apple-gray-500">${plan.price_month}/mo • {plan.token_limit === 0 ? 'Unlimited' : `${plan.token_limit / 1000}K`} tokens</p>
                  </div>
                </div>
                <div className="flex gap-2">
                  <button className="apple-button-secondary text-sm">Edit</button>
                  <button className="p-2 text-apple-gray-400 hover:text-apple-red"><TrashIcon className="w-5 h-5" /></button>
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <motion.div
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          className="max-w-2xl"
        >
          <form onSubmit={handleUpdatePayment} className="card space-y-6">
            <div className="flex items-center gap-3 pb-4 border-b border-apple-gray-100">
              <CreditCardIcon className="w-6 h-6 text-apple-blue" />
              <h2 className="text-lg font-semibold text-apple-gray-900">Stripe Configuration</h2>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">Stripe Enabled</label>
                <select
                  value={paymentSettings.stripe_enabled}
                  onChange={(e) => setPaymentSettings({...paymentSettings, stripe_enabled: e.target.value})}
                  className="apple-input w-full"
                >
                  <option value="true">Enabled</option>
                  <option value="false">Disabled</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">Publishable Key</label>
                <input
                  type="text"
                  value={paymentSettings.stripe_publishable_key}
                  onChange={(e) => setPaymentSettings({...paymentSettings, stripe_publishable_key: e.target.value})}
                  className="apple-input w-full font-mono text-sm"
                  placeholder="pk_test_..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">Secret Key</label>
                <input
                  type="password"
                  value={paymentSettings.stripe_secret_key}
                  onChange={(e) => setPaymentSettings({...paymentSettings, stripe_secret_key: e.target.value})}
                  className="apple-input w-full font-mono text-sm"
                  placeholder="sk_test_..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">Webhook Secret</label>
                <input
                  type="password"
                  value={paymentSettings.stripe_webhook_secret}
                  onChange={(e) => setPaymentSettings({...paymentSettings, stripe_webhook_secret: e.target.value})}
                  className="apple-input w-full font-mono text-sm"
                  placeholder="whsec_..."
                />
              </div>
            </div>

            <div className="pt-4">
              <button type="submit" className="apple-button-primary w-full">
                Save Changes
              </button>
            </div>
          </form>
        </motion.div>
      )}
    </div>
  );
}

export default AdminSettingsPage;
