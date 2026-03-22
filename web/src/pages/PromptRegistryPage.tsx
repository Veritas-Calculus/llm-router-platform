import { useState } from 'react';
import { useTranslation } from '@/lib/i18n';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  PlusIcon,
  TrashIcon,
  PencilSquareIcon,
  CheckCircleIcon,
  ClockIcon,
  DocumentTextIcon,
  XMarkIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import {
  PROMPT_TEMPLATES_QUERY,
  PROMPT_VERSIONS_QUERY,
  CREATE_PROMPT_TEMPLATE,
  UPDATE_PROMPT_TEMPLATE,
  DELETE_PROMPT_TEMPLATE,
  CREATE_PROMPT_VERSION,
  SET_ACTIVE_PROMPT_VERSION,
} from '@/lib/graphql/operations/prompts';

/* eslint-disable @typescript-eslint/no-explicit-any */

// ─── Template List ──────────────────────────────────────────────────

function PromptRegistryPage() {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<any>(PROMPT_TEMPLATES_QUERY, {
    fetchPolicy: 'cache-and-network',
  });
  const [createMut] = useMutation(CREATE_PROMPT_TEMPLATE);
  const [updateMut] = useMutation(UPDATE_PROMPT_TEMPLATE);
  const [deleteMut] = useMutation(DELETE_PROMPT_TEMPLATE);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editTemplate, setEditTemplate] = useState<any>(null);
  const [selectedTemplate, setSelectedTemplate] = useState<any>(null);
  const [formName, setFormName] = useState('');
  const [formDesc, setFormDesc] = useState('');

  const templates = data?.promptTemplates?.data || [];

  const openCreate = () => {
    setFormName('');
    setFormDesc('');
    setEditTemplate(null);
    setShowCreateModal(true);
  };

  const openEdit = (tpl: any) => {
    setFormName(tpl.name);
    setFormDesc(tpl.description || '');
    setEditTemplate(tpl);
    setShowCreateModal(true);
  };

  const handleSave = async () => {
    try {
      if (editTemplate) {
        await updateMut({
          variables: { id: editTemplate.id, input: { name: formName, description: formDesc } },
        });
        toast.success(t('prompts.template_updated'));
      } else {
        await createMut({
          variables: { input: { name: formName, description: formDesc } },
        });
        toast.success(t('prompts.template_created'));
      }
      setShowCreateModal(false);
      await refetch();
    } catch {
      toast.error(t('prompts.save_failed'));
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('prompts.delete_confirm'))) return;
    try {
      await deleteMut({ variables: { id } });
      toast.success(t('prompts.template_deleted'));
      if (selectedTemplate?.id === id) setSelectedTemplate(null);
      await refetch();
    } catch {
      toast.error(t('prompts.delete_failed'));
    }
  };

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('prompts.title')}</h1>
          <p className="text-apple-gray-500 mt-1">
            {t('prompts.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={() => refetch()} className="btn btn-secondary">
            <ArrowPathIcon className="w-5 h-5 mr-2" />
            {t('common.refresh')}
          </button>
          <button onClick={openCreate} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" />
            {t('prompts.new_template')}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Template List */}
        <div className="lg:col-span-1 space-y-3">
          {templates.length === 0 ? (
            <div className="card text-center py-12">
              <DocumentTextIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
              <h3 className="text-lg font-semibold text-apple-gray-900">{t('prompts.no_templates')}</h3>
              <p className="text-sm text-apple-gray-500 mt-1">{t('prompts.no_templates_desc')}</p>
            </div>
          ) : (
            templates.map((tpl: any) => (
              <motion.div
                key={tpl.id}
                initial={{ opacity: 0, y: 5 }}
                animate={{ opacity: 1, y: 0 }}
                className={`card cursor-pointer transition-all hover:shadow-md ${
                  selectedTemplate?.id === tpl.id
                    ? 'ring-2 ring-apple-blue border-transparent'
                    : ''
                }`}
                onClick={() => setSelectedTemplate(tpl)}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <h3 className="text-sm font-semibold text-apple-gray-900 truncate">{tpl.name}</h3>
                    <p className="text-xs text-apple-gray-500 mt-1 line-clamp-2">
                      {tpl.description || t('prompts.no_description')}
                    </p>
                  </div>
                  <span
                    className={`ml-2 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                      tpl.isActive
                        ? 'bg-green-50 text-apple-green ring-1 ring-green-600/20'
                        : 'bg-apple-gray-100 text-apple-gray-500'
                    }`}
                  >
                    {tpl.isActive ? t('common.active') : t('common.inactive')}
                  </span>
                </div>
                <div className="flex items-center justify-between mt-3 pt-3 border-t border-apple-gray-100">
                  <div className="flex items-center gap-3 text-xs text-apple-gray-400">
                    <span>{tpl.versionCount} version{tpl.versionCount !== 1 ? 's' : ''}</span>
                    {tpl.activeVersion && (
                      <span className="text-apple-blue">v{tpl.activeVersion.version}</span>
                    )}
                  </div>
                    <div className="flex items-center gap-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        openEdit(tpl);
                      }}
                      className="inline-flex items-center gap-1 text-xs text-apple-gray-500 hover:text-apple-blue transition-colors"
                    >
                      <PencilSquareIcon className="w-3.5 h-3.5" />
                      {t('common.edit')}
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(tpl.id);
                      }}
                      className="inline-flex items-center gap-1 text-xs text-apple-gray-400 hover:text-apple-red transition-colors"
                    >
                      <TrashIcon className="w-3.5 h-3.5" />
                      {t('common.delete')}
                    </button>
                    </div>
                </div>
              </motion.div>
            ))
          )}
        </div>

        {/* Version Panel */}
        <div className="lg:col-span-2">
          {selectedTemplate ? (
            <VersionPanel template={selectedTemplate} onRefresh={refetch} />
          ) : (
            <div className="card flex items-center justify-center h-64">
              <div className="text-center">
                <DocumentTextIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
                <p className="text-apple-gray-500">{t('prompts.select_template')}</p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Create/Edit Modal */}
      <AnimatePresence>
        {showCreateModal && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm">
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              className="bg-white rounded-2xl shadow-xl w-full max-w-md p-6"
            >
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-lg font-semibold text-apple-gray-900">
                  {editTemplate ? t('prompts.edit_template') : t('prompts.new_template')}
                </h2>
                <button onClick={() => setShowCreateModal(false)}>
                  <XMarkIcon className="w-5 h-5 text-apple-gray-400" />
                </button>
              </div>
              <div className="space-y-4">
                <div>
                  <label className="label">{t('common.name')}</label>
                  <input
                    type="text"
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                    className="input mt-1 w-full"
                    placeholder="e.g. customer-support-v1"
                  />
                </div>
                <div>
                  <label className="label">Description</label>
                  <textarea
                    value={formDesc}
                    onChange={(e) => setFormDesc(e.target.value)}
                    className="input mt-1 w-full"
                    rows={3}
                    placeholder="Describe the purpose of this prompt template..."
                  />
                </div>
              </div>
              <div className="flex justify-end gap-3 mt-6">
                <button onClick={() => setShowCreateModal(false)} className="btn btn-secondary">
                  {t('common.cancel')}
                </button>
                <button onClick={handleSave} className="btn btn-primary" disabled={!formName.trim()}>
                  {editTemplate ? t('common.save') : t('common.submit')}
                </button>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}

// ─── Version Panel ──────────────────────────────────────────────────

function VersionPanel({ template, onRefresh }: { template: any; onRefresh: () => void }) {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<any>(PROMPT_VERSIONS_QUERY, {
    variables: { templateId: template.id },
    fetchPolicy: 'cache-and-network',
  });
  const [createVersionMut] = useMutation(CREATE_PROMPT_VERSION);
  const [setActiveMut] = useMutation(SET_ACTIVE_PROMPT_VERSION);

  const [showNewVersion, setShowNewVersion] = useState(false);
  const [content, setContent] = useState('');
  const [model, setModel] = useState('');
  const [changeLog, setChangeLog] = useState('');

  const versions = data?.promptVersions || [];

  const handleCreateVersion = async () => {
    try {
      await createVersionMut({
        variables: {
          input: {
            templateId: template.id,
            content,
            model: model || null,
            changeLog: changeLog || null,
          },
        },
      });
      toast.success(t('prompts.version_created'));
      setShowNewVersion(false);
      setContent('');
      setModel('');
      setChangeLog('');
      await refetch();
      await onRefresh();
    } catch {
      toast.error(t('prompts.version_failed'));
    }
  };

  const handleSetActive = async (versionId: string) => {
    try {
      await setActiveMut({
        variables: { templateId: template.id, versionId },
      });
      toast.success(t('prompts.active_updated'));
      await onRefresh();
    } catch {
      toast.error(t('prompts.active_failed'));
    }
  };

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold text-apple-gray-900">{template.name}</h2>
          <p className="text-sm text-apple-gray-500">{template.description || t('prompts.no_description')}</p>
        </div>
        <button onClick={() => setShowNewVersion(!showNewVersion)} className="btn btn-primary">
          <PlusIcon className="w-5 h-5 mr-2" />
          {t('prompts.new_version')}
        </button>
      </div>

      {/* New Version Form */}
      <AnimatePresence>
        {showNewVersion && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="mb-6 overflow-hidden"
          >
            <div className="bg-apple-gray-50 rounded-xl p-5 border border-apple-gray-200 space-y-4">
              <div>
                <label className="label">{t('prompts.prompt_content')}</label>
                <textarea
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                  className="input mt-1 w-full font-mono text-sm"
                  rows={6}
                  placeholder={'You are a helpful assistant for {{company_name}}.\n\nUser query: {{user_input}}\n\nPlease respond...'}
                />
                <p className="text-xs text-apple-gray-400 mt-1">
                  {t('prompts.variable_hint')}
                </p>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="label">{t('prompts.recommended_model')}</label>
                  <input
                    type="text"
                    value={model}
                    onChange={(e) => setModel(e.target.value)}
                    className="input mt-1 w-full"
                    placeholder="e.g. gpt-4o"
                  />
                </div>
                <div>
                  <label className="label">{t('prompts.change_log')}</label>
                  <input
                    type="text"
                    value={changeLog}
                    onChange={(e) => setChangeLog(e.target.value)}
                    className="input mt-1 w-full"
                    placeholder="What changed in this version?"
                  />
                </div>
              </div>
              <div className="flex justify-end gap-3">
                <button onClick={() => setShowNewVersion(false)} className="btn btn-secondary">
                  {t('common.cancel')}
                </button>
                <button
                  onClick={handleCreateVersion}
                  className="btn btn-primary"
                  disabled={!content.trim()}
                >
                  {t('prompts.publish_version')}
                </button>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Version Timeline */}
      {loading && !data ? (
        <div className="flex items-center justify-center h-32">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
        </div>
      ) : versions.length === 0 ? (
        <div className="text-center py-8">
          <ClockIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-3" />
          <p className="text-apple-gray-500">{t('prompts.no_versions')}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {versions.map((v: any) => {
            const isActive = template.activeVersionId === v.id;
            return (
              <motion.div
                key={v.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className={`rounded-xl border p-4 transition-all ${
                  isActive
                    ? 'border-apple-blue bg-blue-50/50'
                    : 'border-apple-gray-200 hover:border-apple-gray-300'
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-apple-gray-900">v{v.version}</span>
                    {isActive && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-apple-blue px-2 py-0.5 text-xs font-medium text-white">
                        <CheckCircleIcon className="w-3 h-3" />
                        {t('common.active')}
                      </span>
                    )}
                    {v.model && (
                      <span className="text-xs bg-apple-gray-100 px-2 py-0.5 rounded-full text-apple-gray-600">
                        {v.model}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {!isActive && (
                      <button
                        onClick={() => handleSetActive(v.id)}
                        className="text-xs text-apple-blue hover:text-blue-700 font-medium"
                      >
                        {t('prompts.set_active')}
                      </button>
                    )}
                    <span className="text-xs text-apple-gray-400">
                      {new Date(v.createdAt).toLocaleDateString('en-US', {
                        month: 'short',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </span>
                  </div>
                </div>
                {v.changeLog && (
                  <p className="text-xs text-apple-gray-500 mb-2 italic">{v.changeLog}</p>
                )}
                <pre className="text-xs text-apple-gray-700 bg-apple-gray-50 rounded-lg p-3 overflow-x-auto max-h-32 font-mono whitespace-pre-wrap">
                  {v.content}
                </pre>
              </motion.div>
            );
          })}
        </div>
      )}
    </div>
  );
}

export default PromptRegistryPage;
