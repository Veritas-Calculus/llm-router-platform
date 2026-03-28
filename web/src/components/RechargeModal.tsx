/* eslint-disable @typescript-eslint/no-explicit-any */

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { XMarkIcon, CreditCardIcon, ArrowPathIcon } from '@heroicons/react/24/outline';
import { useMutation } from '@apollo/client/react';
import { CREATE_RECHARGE_SESSION } from '@/lib/graphql/operations/billing';
import { useTranslation } from '@/lib/i18n';
import toast from 'react-hot-toast';

interface RechargeModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function RechargeModal({ isOpen, onClose }: RechargeModalProps) {
  const { t } = useTranslation();
  const [amount, setAmount] = useState<number>(10);
  const [customAmount, setCustomAmount] = useState<string>('');
  const [createSession, { loading }] = useMutation(CREATE_RECHARGE_SESSION);

  const presetAmounts = [10, 50, 100, 500];

  const handleRecharge = async () => {
    const finalAmount = customAmount ? parseFloat(customAmount) : amount;
    if (!finalAmount || finalAmount < 1) {
      toast.error(t('subscription.invalid_amount'));
      return;
    }

    try {
      const { data } = await createSession({
        variables: { amount: finalAmount }
      });
      if ((data as any)?.createRechargeSession?.url) {
        window.location.href = (data as any).createRechargeSession.url;
      }
    } catch (e: any) {
      toast.error(e.message || t('subscription.recharge_error'));
    }
  };

  if (!isOpen) return null;

  return (
    <AnimatePresence>
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
        {/* Backdrop */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={onClose}
          className="absolute inset-0 bg-black/20 backdrop-blur-sm"
        />

        {/* Modal */}
        <motion.div
          initial={{ opacity: 0, scale: 0.95, y: 10 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={{ opacity: 0, scale: 0.95, y: 10 }}
          className="relative w-full max-w-md bg-white dark:bg-[#1C1C1E] rounded-2xl shadow-xl overflow-hidden"
          style={{ border: '1px solid var(--theme-border)' }}
        >
          {/* Header */}
          <div className="flex items-center justify-between p-5 border-b border-apple-gray-100 dark:border-[var(--theme-border)]">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-green-50 dark:bg-green-900/30 rounded-xl">
                <CreditCardIcon className="w-5 h-5 text-green-600 dark:text-green-400" />
              </div>
              <h2 className="text-lg font-bold" style={{ color: 'var(--theme-text)' }}>
                {t('subscription.top_up')}
              </h2>
            </div>
            <button
              onClick={onClose}
              className="p-1.5 rounded-full hover:bg-apple-gray-100 dark:hover:bg-white/10 transition-colors"
              style={{ color: 'var(--theme-text-muted)' }}
            >
              <XMarkIcon className="w-5 h-5" />
            </button>
          </div>

          {/* Body */}
          <div className="p-6 space-y-6">
            <div>
              <label className="block text-sm font-medium mb-3" style={{ color: 'var(--theme-text-secondary)' }}>
                {t('subscription.select_amount')}
              </label>
              <div className="grid grid-cols-2 gap-3">
                {presetAmounts.map((preset) => (
                  <button
                    key={preset}
                    onClick={() => {
                      setAmount(preset);
                      setCustomAmount('');
                    }}
                    className={`py-3 px-4 rounded-xl border text-center font-bold transition-all ${
                      amount === preset && !customAmount
                        ? 'border-green-500 bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400 ring-1 ring-green-500'
                        : 'border-apple-gray-200 dark:border-[var(--theme-border)] hover:bg-apple-gray-50 dark:hover:bg-white/5'
                    }`}
                    style={amount !== preset || customAmount ? { color: 'var(--theme-text)' } : {}}
                  >
                    ${preset}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium mb-2" style={{ color: 'var(--theme-text-secondary)' }}>
                {t('subscription.custom_amount')}
              </label>
              <div className="relative">
                <span className="absolute left-4 top-1/2 -translate-y-1/2 text-apple-gray-400 font-bold">$</span>
                <input
                  type="number"
                  min="1"
                  step="0.01"
                  value={customAmount}
                  onChange={(e) => {
                    setCustomAmount(e.target.value);
                    if (e.target.value) setAmount(0);
                  }}
                  placeholder="0.00"
                  className="w-full pl-8 pr-4 py-3 rounded-xl focus:outline-none focus:ring-2 focus:ring-green-500/50 focus:border-green-500 transition-shadow font-bold text-lg"
                  style={{
                    backgroundColor: 'var(--theme-bg-input)',
                    border: '1px solid var(--theme-border)',
                    color: 'var(--theme-text)',
                  }}
                />
              </div>
            </div>
          </div>

          {/* Footer */}
          <div className="p-5 bg-apple-gray-50 dark:bg-[#2C2C2E] border-t border-apple-gray-100 dark:border-[var(--theme-border)]">
            <button
              onClick={handleRecharge}
              disabled={loading || (!amount && !customAmount)}
              className="w-full py-3 px-4 bg-green-500 hover:bg-green-600 active:scale-95 text-white rounded-xl font-bold flex items-center justify-center gap-2 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? (
                <ArrowPathIcon className="w-5 h-5 animate-spin" />
              ) : (
                <>
                  <CreditCardIcon className="w-5 h-5" />
                  {t('subscription.pay_now')}
                </>
              )}
            </button>
            <p className="text-center text-xs mt-3" style={{ color: 'var(--theme-text-muted)' }}>
              {t('subscription.secure_payment_desc')}
            </p>
          </div>
        </motion.div>
      </div>
    </AnimatePresence>
  );
}
