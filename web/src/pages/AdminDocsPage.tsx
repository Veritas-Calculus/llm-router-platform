/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import { DocumentTextIcon, PlusIcon, PencilSquareIcon, TrashIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';
import { DOCUMENTS_QUERY, CREATE_DOCUMENT, UPDATE_DOCUMENT, DELETE_DOCUMENT } from '@/lib/graphql/operations/documents';

interface Document {
  id: string;
  title: string;
  slug: string;
  content: string;
  category: string;
  sortOrder: number;
  isPublished: boolean;
  createdAt: string;
  updatedAt: string;
}

const emptyForm = { title: '', slug: '', content: '', category: 'general', sortOrder: 0, isPublished: false };

/* ── Tiny Markdown renderer ── */
function renderMarkdown(md: string): string {
  if (!md) return '<p class="text-apple-gray-400 italic">Nothing to preview</p>';
  return md
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/^### (.+)$/gm, '<h3 class="text-base font-semibold mt-3 mb-1">$1</h3>')
    .replace(/^## (.+)$/gm, '<h2 class="text-lg font-semibold mt-4 mb-1">$1</h2>')
    .replace(/^# (.+)$/gm, '<h1 class="text-xl font-bold mt-4 mb-2">$1</h1>')
    .replace(/`([^`]+)`/g, '<code class="px-1.5 py-0.5 rounded bg-apple-gray-100 text-sm font-mono">$1</code>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" class="text-apple-blue hover:underline" target="_blank" rel="noopener">$1</a>')
    .replace(/^[-*] (.+)$/gm, '<li class="ml-4 list-disc">$1</li>')
    .replace(/^(?!<[hla-z])(.*\S.*)$/gm, '<p class="my-1">$1</p>');
}

function AdminDocsPage() {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Document | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [previewMode, setPreviewMode] = useState(false);

  const { data, loading, refetch } = useQuery<any>(DOCUMENTS_QUERY);
  const [createDocument, { loading: saving }] = useMutation<any>(CREATE_DOCUMENT);
  const [updateDocument] = useMutation<any>(UPDATE_DOCUMENT);
  const [deleteDocument] = useMutation<any>(DELETE_DOCUMENT);

  const items: Document[] = data?.documents || [];

  const openCreate = () => { setForm(emptyForm); setEditing(null); setCreating(true); setPreviewMode(false); };
  const openEdit = (d: Document) => {
    setForm({ title: d.title, slug: d.slug, content: d.content, category: d.category, sortOrder: d.sortOrder, isPublished: d.isPublished });
    setEditing(d); setCreating(true); setPreviewMode(false);
  };

  const autoSlug = (title: string) => title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');

  const handleSubmit = async () => {
    try {
      const input = { ...form, category: form.category || undefined, sortOrder: form.sortOrder || undefined, isPublished: form.isPublished || undefined };
      if (editing) { await updateDocument({ variables: { id: editing.id, input } }); }
      else { await createDocument({ variables: { input } }); }
      setCreating(false); setEditing(null); refetch();
    } catch (err) { console.error('Failed to save document:', err); }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('documents.confirm_delete'))) return;
    try { await deleteDocument({ variables: { id } }); refetch(); }
    catch (err) { console.error('Failed to delete:', err); }
  };

  const categoryColors: Record<string, string> = {
    'getting-started': 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    'api-reference': 'bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
    faq: 'bg-yellow-50 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300',
    general: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300',
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">{t('documents.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('documents.subtitle')}</p>
        </div>
        <button onClick={openCreate} className="btn-primary flex items-center gap-2">
          <PlusIcon className="w-4 h-4" />{t('documents.create')}
        </button>
      </div>

      {/* Create / Edit Form */}
      {creating && (
        <motion.div initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} className="card">
          <div className="flex items-center justify-between mb-5">
            <h3 className="text-lg font-semibold text-apple-gray-900">{editing ? t('documents.edit') : t('documents.create')}</h3>
            <button onClick={() => { setCreating(false); setEditing(null); }} className="btn-icon">
              <XMarkIcon className="w-5 h-5" />
            </button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {/* Title */}
            <div>
              <label className="form-label">{t('documents.field_title')}</label>
              <input className="form-input" value={form.title}
                onChange={e => { const v = e.target.value; setForm(f => ({ ...f, title: v, slug: editing ? f.slug : autoSlug(v) })); }} />
            </div>
            {/* Slug */}
            <div>
              <label className="form-label">{t('documents.slug')}</label>
              <input className="form-input font-mono text-sm" value={form.slug}
                onChange={e => setForm(f => ({ ...f, slug: e.target.value }))} />
            </div>
            {/* Category */}
            <div>
              <label className="form-label">{t('documents.category')}</label>
              <select className="form-input" value={form.category} onChange={e => setForm(f => ({ ...f, category: e.target.value }))}>
                <option value="general">{t('documents.cat_general')}</option>
                <option value="getting-started">{t('documents.cat_getting_started')}</option>
                <option value="api-reference">{t('documents.cat_api_reference')}</option>
                <option value="faq">{t('documents.cat_faq')}</option>
              </select>
            </div>
            {/* Sort Order */}
            <div>
              <label className="form-label">{t('documents.sort_order')}</label>
              <input type="number" className="form-input" value={form.sortOrder}
                onChange={e => setForm(f => ({ ...f, sortOrder: Number(e.target.value) }))} />
            </div>
            {/* Content — Markdown Editor with Tabs */}
            <div className="sm:col-span-2">
              <label className="form-label">{t('documents.content')}</label>
              <div className="border border-[var(--theme-border)] rounded-xl overflow-hidden">
                <div className="markdown-tabs">
                  <button className={`markdown-tab ${!previewMode ? 'markdown-tab--active' : ''}`}
                    onClick={() => setPreviewMode(false)}>{t('common.write') || 'Write'}</button>
                  <button className={`markdown-tab ${previewMode ? 'markdown-tab--active' : ''}`}
                    onClick={() => setPreviewMode(true)}>{t('common.preview') || 'Preview'}</button>
                </div>
                {previewMode ? (
                  <div className="markdown-preview prose prose-sm max-w-none dark:prose-invert"
                    dangerouslySetInnerHTML={{ __html: renderMarkdown(form.content) }} />
                ) : (
                  <textarea className="w-full px-3 py-3 font-mono text-sm border-0 focus:ring-0 focus:outline-none bg-transparent resize-y"
                    rows={10} value={form.content}
                    onChange={e => setForm(f => ({ ...f, content: e.target.value }))}
                    placeholder="Markdown supported — # Heading, **bold**, *italic*, `code`, - list" />
                )}
              </div>
            </div>
            {/* Publish Toggle */}
            <div className="flex items-center gap-2.5">
              <input type="checkbox" id="doc-published" className="form-checkbox" checked={form.isPublished}
                onChange={e => setForm(f => ({ ...f, isPublished: e.target.checked }))} />
              <label htmlFor="doc-published" className="text-sm text-apple-gray-600 cursor-pointer select-none">{t('documents.publish')}</label>
            </div>
          </div>
          {/* Actions */}
          <div className="mt-5 flex justify-end gap-3">
            <button onClick={() => { setCreating(false); setEditing(null); }} className="btn-secondary">{t('common.cancel') || 'Cancel'}</button>
            <button onClick={handleSubmit} disabled={saving} className="btn-primary">
              {saving ? t('common.loading') : t('common.save')}
            </button>
          </div>
        </motion.div>
      )}

      {/* List */}
      <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} className="card overflow-hidden p-0">
        {loading ? (
          <div className="p-8 text-center text-apple-gray-400">{t('common.loading')}</div>
        ) : items.length === 0 ? (
          <div className="p-12 text-center">
            <DocumentTextIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
            <p className="text-apple-gray-500">{t('documents.empty')}</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-apple-gray-50 text-apple-gray-500 text-xs uppercase tracking-wider">
              <tr>
                <th className="px-5 py-3 text-left font-medium">{t('documents.field_title')}</th>
                <th className="px-5 py-3 text-left font-medium">{t('documents.slug')}</th>
                <th className="px-5 py-3 text-left font-medium">{t('documents.category')}</th>
                <th className="px-5 py-3 text-center font-medium">{t('common.status')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {items.map(d => (
                <tr key={d.id} className="hover:bg-apple-gray-50/50 transition-colors">
                  <td className="px-5 py-3.5 font-medium text-apple-gray-900">{d.title}</td>
                  <td className="px-5 py-3.5 font-mono text-xs text-apple-gray-500">{d.slug}</td>
                  <td className="px-5 py-3.5">
                    <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium ${categoryColors[d.category] || 'bg-gray-100 text-gray-600'}`}>
                      {d.category}
                    </span>
                  </td>
                  <td className="px-5 py-3.5 text-center">
                    <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      d.isPublished ? 'bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300'}`}>
                      {d.isPublished ? t('documents.published') : t('documents.draft')}
                    </span>
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <div className="flex gap-1 justify-end">
                      <button onClick={() => openEdit(d)} className="btn-icon" title={t('common.edit')}>
                        <PencilSquareIcon className="w-4 h-4" />
                      </button>
                      <button onClick={() => handleDelete(d.id)} className="btn-icon btn-icon-danger" title={t('common.delete')}>
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

export default AdminDocsPage;
