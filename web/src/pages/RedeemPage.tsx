import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  GiftIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  XCircleIcon,
  TicketIcon,
  SparklesIcon,
} from '@heroicons/react/24/outline';
import { useMutation, useQuery } from '@apollo/client/react';
import { REDEEM_CODE_MUTATION, MY_REDEEM_HISTORY } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';
import toast from 'react-hot-toast';

/* eslint-disable @typescript-eslint/no-explicit-any */

function RedeemPage() {
  const { t } = useTranslation();
  const [code, setCode] = useState('');
  const [redeemMut, { loading: redeeming }] = useMutation<any>(REDEEM_CODE_MUTATION);
  const { data: historyData, refetch } = useQuery<any>(MY_REDEEM_HISTORY, {
    fetchPolicy: 'network-only',
  });
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);

  const history = (historyData?.myRedeemHistory || []) as any[];

  const handleRedeem = async () => {
    const trimmed = code.trim();
    if (!trimmed) return;
    setResult(null);
    try {
      const { data } = await redeemMut({ variables: { code: trimmed } });
      const res = data?.redeemCode;
      if (res?.success) {
        setResult({ success: true, message: res.message || t('redeem.success_msg') });
        setCode('');
        refetch();
        toast.success(t('redeem.success_msg'));
      } else {
        setResult({ success: false, message: res?.message || t('redeem.error_msg') });
      }
    } catch (err: any) {
      const message = err?.graphQLErrors?.[0]?.message || t('redeem.error_msg');
      setResult({ success: false, message });
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-apple-gray-900">{t('redeem.title')}</h1>
        <p className="mt-1 text-apple-gray-500">{t('redeem.subtitle')}</p>
      </div>

      {/* Redeem input card */}
      <div className="card p-8">
        <div className="max-w-lg mx-auto text-center">
          <div className="w-16 h-16 bg-gradient-to-br from-blue-50 to-purple-50 rounded-2xl flex items-center justify-center mx-auto mb-6">
            <GiftIcon className="w-8 h-8 text-apple-blue" />
          </div>
          <h2 className="text-xl font-bold text-apple-gray-900 mb-2">{t('redeem.input_title')}</h2>
          <p className="text-sm text-apple-gray-500 mb-6">{t('redeem.input_desc')}</p>

          <div className="flex gap-3">
            <input
              type="text"
              value={code}
              onChange={(e) => {
                setCode(e.target.value.toUpperCase());
                setResult(null);
              }}
              onKeyDown={(e) => e.key === 'Enter' && handleRedeem()}
              placeholder={t('redeem.placeholder')}
              className="flex-1 px-4 py-3 rounded-xl border border-apple-gray-200 text-sm font-mono tracking-widest text-center focus:ring-2 focus:ring-apple-blue focus:border-apple-blue outline-none transition-all bg-apple-gray-50 placeholder:text-apple-gray-400"
              maxLength={32}
              disabled={redeeming}
            />
            <button
              onClick={handleRedeem}
              disabled={!code.trim() || redeeming}
              className="px-6 py-3 bg-apple-blue text-white font-semibold text-sm rounded-xl hover:bg-blue-600 active:scale-95 transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 shadow-sm"
            >
              {redeeming ? (
                <ArrowPathIcon className="w-4 h-4 animate-spin" />
              ) : (
                <TicketIcon className="w-4 h-4" />
              )}
              {t('redeem.submit')}
            </button>
          </div>

          <AnimatePresence>
            {result && (
              <motion.div
                initial={{ opacity: 0, y: -8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className={`mt-4 flex items-center justify-center gap-2 text-sm font-medium ${
                  result.success ? 'text-green-600' : 'text-red-600'
                }`}
              >
                {result.success ? (
                  <CheckCircleIcon className="w-4 h-4" />
                ) : (
                  <XCircleIcon className="w-4 h-4" />
                )}
                {result.message}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>

      {/* Redeem history */}
      {history.length > 0 && (
        <div className="card overflow-hidden">
          <div className="p-5 border-b border-apple-gray-100">
            <h3 className="text-lg font-semibold text-apple-gray-900">{t('redeem.history_title')}</h3>
          </div>
          <div className="divide-y divide-apple-gray-100">
            {history.map((item: any) => (
              <div key={item.id} className="px-5 py-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-9 h-9 bg-green-50 rounded-lg flex items-center justify-center">
                    <SparklesIcon className="w-4 h-4 text-green-600" />
                  </div>
                  <div>
                    <p className="text-sm font-mono text-apple-gray-900">{item.code}</p>
                    <p className="text-xs text-apple-gray-500">
                      {new Date(item.redeemedAt).toLocaleString()}
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  {item.creditAmount > 0 && (
                    <p className="text-sm font-bold text-green-600">+${item.creditAmount.toFixed(2)}</p>
                  )}
                  {item.planName && (
                    <p className="text-xs text-apple-gray-500">{item.planName}</p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {history.length === 0 && (
        <div className="card p-8 text-center">
          <TicketIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
          <p className="text-sm text-apple-gray-500">{t('redeem.no_history')}</p>
        </div>
      )}
    </div>
  );
}

export default RedeemPage;
