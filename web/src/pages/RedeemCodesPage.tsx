/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { TicketIcon, PlusIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';
import { ADMIN_REDEEM_CODES_QUERY, GENERATE_REDEEM_CODES, REVOKE_REDEEM_CODE } from '@/lib/graphql/operations/redeem';

interface RedeemCode {
  id: string;
  code: string;
  type: string;
  creditAmount: number;
  planId?: string;
  usedBy?: string;
  usedAt?: string;
  expiresAt?: string;
  isActive: boolean;
  createdAt: string;
}

function RedeemCodesPage() {
  const { t } = useTranslation();
  const [showForm, setShowForm] = useState(false);
  const [page] = useState(1);
  const [formData, setFormData] = useState({
    type: 'credit',
    count: 10,
    creditAmount: 10,
    planDays: 30,
    note: '',
  });

  const { data, loading, refetch } = useQuery<any>(ADMIN_REDEEM_CODES_QUERY, {
    variables: { page, pageSize: 50 },
  });
  const [generateCodes, { loading: generating }] = useMutation<any>(GENERATE_REDEEM_CODES);
  const [revokeCode] = useMutation<any>(REVOKE_REDEEM_CODE);

  const codes: RedeemCode[] = data?.redeemCodes?.nodes || [];

  const handleGenerate = async () => {
    try {
      await generateCodes({
        variables: {
          input: {
            type: formData.type,
            count: formData.count,
            creditAmount: formData.creditAmount,
            planDays: formData.planDays,
            note: formData.note || undefined,
          },
        },
      });
      setShowForm(false);
      refetch();
    } catch (err) {
      console.error('Failed to generate codes:', err);
    }
  };

  const handleRevoke = async (id: string) => {
    if (!confirm(t('redeem_codes.confirm_revoke'))) return;
    try {
      await revokeCode({ variables: { id } });
      refetch();
    } catch (err) {
      console.error('Failed to revoke code:', err);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">{t('redeem_codes.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('redeem_codes.subtitle')}</p>
        </div>
        <button onClick={() => setShowForm(true)} className="btn-primary flex items-center gap-2">
          <PlusIcon className="w-4 h-4" />
          {t('redeem_codes.generate')}
        </button>
      </div>

      {showForm && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">{t('redeem_codes.generate_form_title')}</h3>
            <button onClick={() => setShowForm(false)} className="btn-icon"><XMarkIcon className="w-5 h-5" /></button>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="form-label">{t('redeem_codes.type')}</label>
              <select className="form-input" value={formData.type} onChange={e => setFormData(d => ({ ...d, type: e.target.value }))}>
                <option value="credit">Credit</option>
                <option value="plan">Plan</option>
              </select>
            </div>
            <div>
              <label className="form-label">{t('redeem_codes.count')}</label>
              <input type="number" className="form-input" value={formData.count} min={1} max={100}
                onChange={e => setFormData(d => ({ ...d, count: Number(e.target.value) }))} />
            </div>
            <div>
              <label className="form-label">{t('redeem_codes.credit_amount')}</label>
              <input type="number" className="form-input" value={formData.creditAmount} min={0} step={0.01}
                onChange={e => setFormData(d => ({ ...d, creditAmount: Number(e.target.value) }))} />
            </div>
            <div>
              <label className="form-label">{t('redeem_codes.note')}</label>
              <input type="text" className="form-input" value={formData.note}
                onChange={e => setFormData(d => ({ ...d, note: e.target.value }))} />
            </div>
          </div>
          <div className="mt-5 flex justify-end gap-3">
            <button onClick={() => setShowForm(false)} className="btn-secondary">{t('common.cancel') || 'Cancel'}</button>
            <button onClick={handleGenerate} disabled={generating} className="btn-primary">
              {generating ? t('common.loading') : t('redeem_codes.generate')}
            </button>
          </div>
        </motion.div>
      )}

      <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} className="card overflow-hidden p-0">
        {loading ? (
          <div className="p-8 text-center text-apple-gray-400">{t('common.loading')}</div>
        ) : codes.length === 0 ? (
          <div className="p-12 text-center">
            <TicketIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
            <p className="text-apple-gray-500">{t('redeem_codes.empty')}</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-apple-gray-50 text-apple-gray-500 text-xs uppercase tracking-wider">
              <tr>
                <th className="px-5 py-3 text-left font-medium">{t('redeem_codes.code')}</th>
                <th className="px-5 py-3 text-left font-medium">{t('redeem_codes.type')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('redeem_codes.credit_amount')}</th>
                <th className="px-5 py-3 text-center font-medium">{t('common.status')}</th>
                <th className="px-5 py-3 text-left font-medium">{t('common.created')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {codes.map(code => (
                <tr key={code.id} className="hover:bg-apple-gray-50/50 transition-colors">
                  <td className="px-5 py-3.5 font-mono text-xs">{code.code}</td>
                  <td className="px-5 py-3.5 capitalize">{code.type}</td>
                  <td className="px-5 py-3.5 text-right">${code.creditAmount.toFixed(2)}</td>
                  <td className="px-5 py-3.5 text-center">
                    <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      code.usedBy ? 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300' : code.isActive ? 'bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300' : 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
                    }`}>
                      {code.usedBy ? t('redeem_codes.used') : code.isActive ? t('common.active') : t('common.inactive')}
                    </span>
                  </td>
                  <td className="px-5 py-3.5 text-apple-gray-500">{new Date(code.createdAt).toLocaleDateString()}</td>
                  <td className="px-5 py-3.5 text-right">
                    {code.isActive && !code.usedBy && (
                      <button onClick={() => handleRevoke(code.id)} className="text-xs font-medium text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300 px-2 py-1 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors">
                        {t('redeem_codes.revoke')}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </motion.div>
    </div>
  );
}

export default RedeemCodesPage;
