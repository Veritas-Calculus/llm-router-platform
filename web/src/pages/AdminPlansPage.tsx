/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { CreditCardIcon, PlusIcon, PencilSquareIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';
import { PLANS_QUERY, CREATE_PLAN, UPDATE_PLAN } from '@/lib/graphql/operations/plans';

interface Plan {
  id: string;
  name: string;
  description: string;
  priceMonth: number;
  tokenLimit: number;
  rateLimit: number;
  supportLevel: string;
  features?: string;
  isActive: boolean;
}

const emptyForm = { name: '', description: '', priceMonth: 0, tokenLimit: 100000, rateLimit: 10, supportLevel: 'basic', features: '', isActive: true };

function AdminPlansPage() {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Plan | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const { data, loading, refetch } = useQuery<any>(PLANS_QUERY);
  const [createPlan, { loading: saving }] = useMutation<any>(CREATE_PLAN);
  const [updatePlan] = useMutation<any>(UPDATE_PLAN);

  const plans: Plan[] = data?.plans || [];

  const openCreate = () => { setForm(emptyForm); setEditing(null); setCreating(true); };
  const openEdit = (p: Plan) => {
    setForm({ name: p.name, description: p.description || '', priceMonth: p.priceMonth, tokenLimit: p.tokenLimit, rateLimit: p.rateLimit, supportLevel: p.supportLevel || 'basic', features: p.features || '', isActive: p.isActive });
    setEditing(p); setCreating(true);
  };

  const handleSubmit = async () => {
    try {
      const input = { ...form, description: form.description || undefined, supportLevel: form.supportLevel || undefined, features: form.features || undefined };
      if (editing) {
        await updatePlan({ variables: { id: editing.id, input } });
      } else {
        await createPlan({ variables: { input } });
      }
      setCreating(false); setEditing(null); refetch();
    } catch (err) { console.error('Failed to save plan:', err); }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">{t('plans.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('plans.subtitle')}</p>
        </div>
        <button onClick={openCreate} className="btn-primary flex items-center gap-2">
          <PlusIcon className="w-5 h-5 mr-2" />
          {t('plans.create')}
        </button>
      </div>

      {creating && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">{editing ? t('plans.edit') : t('plans.create')}</h3>
            <button onClick={() => { setCreating(false); setEditing(null); }}><XMarkIcon className="w-5 h-5" /></button>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="form-label">{t('plans.name')}</label>
              <input className="form-input" value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} />
            </div>
            <div>
              <label className="form-label">{t('plans.price_month')}</label>
              <input type="number" className="form-input" value={form.priceMonth} step={0.01}
                onChange={e => setForm(f => ({ ...f, priceMonth: Number(e.target.value) }))} />
            </div>
            <div>
              <label className="form-label">{t('plans.token_limit')}</label>
              <input type="number" className="form-input" value={form.tokenLimit}
                onChange={e => setForm(f => ({ ...f, tokenLimit: Number(e.target.value) }))} />
            </div>
            <div>
              <label className="form-label">{t('plans.rate_limit')}</label>
              <input type="number" className="form-input" value={form.rateLimit}
                onChange={e => setForm(f => ({ ...f, rateLimit: Number(e.target.value) }))} />
            </div>
            <div className="col-span-2">
              <label className="form-label">{t('plans.description')}</label>
              <textarea className="form-input" rows={2} value={form.description}
                onChange={e => setForm(f => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="col-span-2">
              <label className="form-label">{t('plans.features')}</label>
              <textarea className="form-input" rows={2} value={form.features} placeholder="Feature 1, Feature 2, ..."
                onChange={e => setForm(f => ({ ...f, features: e.target.value }))} />
            </div>
            <div className="flex items-center gap-2">
              <input type="checkbox" id="plan-active" checked={form.isActive}
                onChange={e => setForm(f => ({ ...f, isActive: e.target.checked }))} />
              <label htmlFor="plan-active" className="text-sm">{t('common.active')}</label>
            </div>
          </div>
          <div className="mt-4 flex justify-end">
            <button onClick={handleSubmit} disabled={saving} className="btn-primary">
              {saving ? t('common.loading') : t('common.save')}
            </button>
          </div>
        </motion.div>
      )}

      <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} className="card overflow-hidden">
        {loading ? (
          <div className="p-8 text-center text-apple-gray-400">{t('common.loading')}</div>
        ) : plans.length === 0 ? (
          <div className="p-12 text-center">
            <CreditCardIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
            <p className="text-apple-gray-500">{t('plans.empty')}</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-apple-gray-50 text-apple-gray-500 text-xs uppercase">
              <tr>
                <th className="px-4 py-3 text-left">{t('plans.name')}</th>
                <th className="px-4 py-3 text-right">{t('plans.price_month')}</th>
                <th className="px-4 py-3 text-right">{t('plans.token_limit')}</th>
                <th className="px-4 py-3 text-right">{t('plans.rate_limit')}</th>
                <th className="px-4 py-3 text-center">{t('common.status')}</th>
                <th className="px-4 py-3 text-right">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {plans.map(plan => (
                <tr key={plan.id} className="hover:bg-apple-gray-50/50">
                  <td className="px-4 py-3 font-medium">{plan.name}</td>
                  <td className="px-4 py-3 text-right">${plan.priceMonth.toFixed(2)}/mo</td>
                  <td className="px-4 py-3 text-right">{(plan.tokenLimit / 1000).toFixed(0)}K</td>
                  <td className="px-4 py-3 text-right">{plan.rateLimit} req/s</td>
                  <td className="px-4 py-3 text-center">
                    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${
                      plan.isActive ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-600'}`}>
                      {plan.isActive ? t('common.active') : t('common.inactive')}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button onClick={() => openEdit(plan)} className="text-blue-600 hover:text-blue-700 inline-flex items-center gap-1 text-sm">
                      <PencilSquareIcon className="w-4 h-4" />
                      {t('common.edit')}
                    </button>
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

export default AdminPlansPage;
