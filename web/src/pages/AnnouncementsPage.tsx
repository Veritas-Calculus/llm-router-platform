/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { MegaphoneIcon, PlusIcon, PencilSquareIcon, TrashIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';
import { ANNOUNCEMENTS_QUERY, CREATE_ANNOUNCEMENT, UPDATE_ANNOUNCEMENT, DELETE_ANNOUNCEMENT } from '@/lib/graphql/operations/announcements';

interface Announcement {
  id: string;
  title: string;
  content: string;
  type: string;
  priority: number;
  isActive: boolean;
  startsAt?: string;
  endsAt?: string;
  createdAt: string;
}

const emptyForm = { title: '', content: '', type: 'info', priority: 0, isActive: false, startsAt: '', endsAt: '' };

function AnnouncementsPage() {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const { data, loading, refetch } = useQuery<any>(ANNOUNCEMENTS_QUERY);
  const [createAnnouncement, { loading: saving }] = useMutation<any>(CREATE_ANNOUNCEMENT);
  const [updateAnnouncement] = useMutation<any>(UPDATE_ANNOUNCEMENT);
  const [deleteAnnouncement] = useMutation<any>(DELETE_ANNOUNCEMENT);

  const items: Announcement[] = data?.announcements || [];

  const openCreate = () => { setForm(emptyForm); setEditing(null); setCreating(true); };
  const openEdit = (a: Announcement) => {
    setForm({ title: a.title, content: a.content, type: a.type, priority: a.priority, isActive: a.isActive, startsAt: a.startsAt || '', endsAt: a.endsAt || '' });
    setEditing(a); setCreating(true);
  };

  const handleSubmit = async () => {
    try {
      const input = { title: form.title, content: form.content, type: form.type, priority: form.priority, isActive: form.isActive,
        startsAt: form.startsAt || undefined, endsAt: form.endsAt || undefined };
      if (editing) { await updateAnnouncement({ variables: { id: editing.id, input } }); }
      else { await createAnnouncement({ variables: { input } }); }
      setCreating(false); setEditing(null); refetch();
    } catch (err) { console.error('Failed to save announcement:', err); }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('announcements.confirm_delete'))) return;
    try { await deleteAnnouncement({ variables: { id } }); refetch(); }
    catch (err) { console.error('Failed to delete:', err); }
  };

  const typeColors: Record<string, string> = {
    info: 'bg-blue-50 text-blue-700', warning: 'bg-yellow-50 text-yellow-700', maintenance: 'bg-orange-50 text-orange-700',
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">{t('announcements.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('announcements.subtitle')}</p>
        </div>
        <button onClick={openCreate} className="btn-primary flex items-center gap-2">
          <PlusIcon className="w-5 h-5 mr-2" />{t('announcements.create')}
        </button>
      </div>

      {creating && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">{editing ? t('announcements.edit') : t('announcements.create')}</h3>
            <button onClick={() => { setCreating(false); setEditing(null); }}><XMarkIcon className="w-5 h-5" /></button>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2">
              <label className="form-label">{t('announcements.field_title')}</label>
              <input className="form-input" value={form.title} onChange={e => setForm(f => ({ ...f, title: e.target.value }))} />
            </div>
            <div>
              <label className="form-label">{t('announcements.field_type')}</label>
              <select className="form-input" value={form.type} onChange={e => setForm(f => ({ ...f, type: e.target.value }))}>
                <option value="info">Info</option>
                <option value="warning">Warning</option>
                <option value="maintenance">Maintenance</option>
              </select>
            </div>
            <div>
              <label className="form-label">{t('announcements.priority')}</label>
              <input type="number" className="form-input" value={form.priority}
                onChange={e => setForm(f => ({ ...f, priority: Number(e.target.value) }))} />
            </div>
            <div className="col-span-2">
              <label className="form-label">{t('announcements.content')}</label>
              <textarea className="form-input font-mono text-sm" rows={5} value={form.content}
                onChange={e => setForm(f => ({ ...f, content: e.target.value }))} placeholder="Markdown supported" />
            </div>
            <div className="flex items-center gap-2 col-span-2">
              <input type="checkbox" id="ann-active" checked={form.isActive}
                onChange={e => setForm(f => ({ ...f, isActive: e.target.checked }))} />
              <label htmlFor="ann-active" className="text-sm">{t('announcements.publish')}</label>
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
        ) : items.length === 0 ? (
          <div className="p-12 text-center">
            <MegaphoneIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
            <p className="text-apple-gray-500">{t('announcements.empty')}</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-apple-gray-50 text-apple-gray-500 text-xs uppercase">
              <tr>
                <th className="px-4 py-3 text-left">{t('announcements.field_title')}</th>
                <th className="px-4 py-3 text-left">{t('announcements.field_type')}</th>
                <th className="px-4 py-3 text-center">{t('common.status')}</th>
                <th className="px-4 py-3 text-left">{t('common.created')}</th>
                <th className="px-4 py-3 text-right">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {items.map(a => (
                <tr key={a.id} className="hover:bg-apple-gray-50/50">
                  <td className="px-4 py-3 font-medium">{a.title}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${typeColors[a.type] || 'bg-gray-100 text-gray-600'}`}>
                      {a.type}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-center">
                    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${
                      a.isActive ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-600'}`}>
                      {a.isActive ? t('common.active') : t('common.inactive')}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-apple-gray-500">{new Date(a.createdAt).toLocaleDateString()}</td>
                  <td className="px-4 py-3 text-right flex gap-2 justify-end">
                    <button onClick={() => openEdit(a)} className="text-blue-600 hover:text-blue-700">
                      <PencilSquareIcon className="w-4 h-4" />
                    </button>
                    <button onClick={() => handleDelete(a.id)} className="text-red-600 hover:text-red-700">
                      <TrashIcon className="w-4 h-4" />
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

export default AnnouncementsPage;
