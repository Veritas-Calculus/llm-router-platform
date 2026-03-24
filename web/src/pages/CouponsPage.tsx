/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { TagIcon, PlusIcon, PencilSquareIcon, TrashIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';
import { COUPONS_QUERY, CREATE_COUPON, UPDATE_COUPON, DELETE_COUPON } from '@/lib/graphql/operations/coupons';

interface Coupon {
  id: string;
  code: string;
  name: string;
  type: string;
  discountValue: number;
  minAmount: number;
  maxUses: number;
  useCount: number;
  maxUsesPerUser: number;
  isActive: boolean;
  expiresAt?: string;
  createdAt: string;
}

const emptyForm = { code: '', name: '', type: 'percent', discountValue: 10, minAmount: 0, maxUses: 0, maxUsesPerUser: 1, isActive: true, expiresAt: '' };

function CouponsPage() {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Coupon | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const { data, loading, refetch } = useQuery<any>(COUPONS_QUERY);
  const [createCoupon, { loading: saving }] = useMutation<any>(CREATE_COUPON);
  const [updateCoupon] = useMutation<any>(UPDATE_COUPON);
  const [deleteCoupon] = useMutation<any>(DELETE_COUPON);

  const items: Coupon[] = data?.coupons || [];

  const openCreate = () => { setForm(emptyForm); setEditing(null); setCreating(true); };
  const openEdit = (c: Coupon) => {
    setForm({ code: c.code, name: c.name, type: c.type, discountValue: c.discountValue, minAmount: c.minAmount, maxUses: c.maxUses, maxUsesPerUser: c.maxUsesPerUser, isActive: c.isActive, expiresAt: c.expiresAt || '' });
    setEditing(c); setCreating(true);
  };

  const handleSubmit = async () => {
    try {
      const input = { ...form, expiresAt: form.expiresAt || undefined, minAmount: form.minAmount || undefined, maxUses: form.maxUses || undefined, maxUsesPerUser: form.maxUsesPerUser || undefined };
      if (editing) { await updateCoupon({ variables: { id: editing.id, input } }); }
      else { await createCoupon({ variables: { input } }); }
      setCreating(false); setEditing(null); refetch();
    } catch (err) { console.error('Failed to save coupon:', err); }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('coupons.confirm_delete'))) return;
    try { await deleteCoupon({ variables: { id } }); refetch(); }
    catch (err) { console.error('Failed to delete:', err); }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">{t('coupons.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('coupons.subtitle')}</p>
        </div>
        <button onClick={openCreate} className="btn-primary flex items-center gap-2">
          <PlusIcon className="w-4 h-4" />{t('coupons.create')}
        </button>
      </div>

      {creating && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">{editing ? t('coupons.edit') : t('coupons.create')}</h3>
            <button onClick={() => { setCreating(false); setEditing(null); }} className="btn-icon"><XMarkIcon className="w-5 h-5" /></button>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="form-label">{t('coupons.code')}</label>
              <input className="form-input" value={form.code} onChange={e => setForm(f => ({ ...f, code: e.target.value }))} />
            </div>
            <div>
              <label className="form-label">{t('coupons.name')}</label>
              <input className="form-input" value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} />
            </div>
            <div>
              <label className="form-label">{t('coupons.discount_type')}</label>
              <select className="form-input" value={form.type} onChange={e => setForm(f => ({ ...f, type: e.target.value }))}>
                <option value="percent">{t('coupons.type_percent')}</option>
                <option value="fixed">{t('coupons.type_fixed')}</option>
              </select>
            </div>
            <div>
              <label className="form-label">{t('coupons.discount_value')}</label>
              <input type="number" className="form-input" value={form.discountValue} step={0.01}
                onChange={e => setForm(f => ({ ...f, discountValue: Number(e.target.value) }))} />
            </div>
            <div>
              <label className="form-label">{t('coupons.min_amount')}</label>
              <input type="number" className="form-input" value={form.minAmount} step={0.01}
                onChange={e => setForm(f => ({ ...f, minAmount: Number(e.target.value) }))} />
            </div>
            <div>
              <label className="form-label">{t('coupons.max_uses')}</label>
              <input type="number" className="form-input" value={form.maxUses} placeholder="0 = unlimited"
                onChange={e => setForm(f => ({ ...f, maxUses: Number(e.target.value) }))} />
            </div>
            <div className="flex items-center gap-2">
              <input type="checkbox" id="coupon-active" checked={form.isActive}
                onChange={e => setForm(f => ({ ...f, isActive: e.target.checked }))} />
              <label htmlFor="coupon-active" className="text-sm">{t('common.active')}</label>
            </div>
          </div>
          <div className="mt-5 flex justify-end gap-3">
            <button onClick={() => { setCreating(false); setEditing(null); }} className="btn-secondary">{t('common.cancel') || 'Cancel'}</button>
            <button onClick={handleSubmit} disabled={saving} className="btn-primary">
              {saving ? t('common.loading') : t('common.save')}
            </button>
          </div>
        </motion.div>
      )}

      <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} className="card overflow-hidden p-0">
        {loading ? (
          <div className="p-8 text-center text-apple-gray-400">{t('common.loading')}</div>
        ) : items.length === 0 ? (
          <div className="p-12 text-center">
            <TagIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
            <p className="text-apple-gray-500">{t('coupons.empty')}</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-apple-gray-50 text-apple-gray-500 text-xs uppercase tracking-wider">
              <tr>
                <th className="px-5 py-3 text-left font-medium">{t('coupons.code')}</th>
                <th className="px-5 py-3 text-left font-medium">{t('coupons.name')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('coupons.discount')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('coupons.usage')}</th>
                <th className="px-5 py-3 text-center font-medium">{t('common.status')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {items.map(c => (
                <tr key={c.id} className="hover:bg-apple-gray-50/50 transition-colors">
                  <td className="px-5 py-3.5 font-mono text-xs">{c.code}</td>
                  <td className="px-5 py-3.5">{c.name}</td>
                  <td className="px-5 py-3.5 text-right">
                    {c.type === 'percent' ? `${c.discountValue}%` : `$${c.discountValue.toFixed(2)}`}
                  </td>
                  <td className="px-5 py-3.5 text-right">{c.useCount}{c.maxUses > 0 ? `/${c.maxUses}` : ''}</td>
                  <td className="px-5 py-3.5 text-center">
                    <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      c.isActive ? 'bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300'}`}>
                      {c.isActive ? t('common.active') : t('common.inactive')}
                    </span>
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <div className="flex gap-1 justify-end">
                      <button onClick={() => openEdit(c)} className="btn-icon" title={t('common.edit')}>
                        <PencilSquareIcon className="w-4 h-4" />
                      </button>
                      <button onClick={() => handleDelete(c.id)} className="btn-icon btn-icon-danger" title={t('common.delete')}>
                        <TrashIcon className="w-4 h-4" />
                      </button>
                    </div>
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

export default CouponsPage;
