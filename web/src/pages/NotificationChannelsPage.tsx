import { useState, useMemo } from 'react';
import { useTranslation } from '@/lib/i18n';
import { useQuery, useMutation } from '@apollo/client/react';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import {
  PlusIcon,
  TrashIcon,
  PencilIcon,
  BellIcon,
  PaperAirplaneIcon,
  LinkIcon,
  EnvelopeIcon,
  ChatBubbleLeftIcon,
} from '@heroicons/react/24/outline';
import {
  NOTIFICATION_CHANNELS_QUERY,
  CREATE_NOTIFICATION_CHANNEL,
  UPDATE_NOTIFICATION_CHANNEL,
  DELETE_NOTIFICATION_CHANNEL,
  TEST_NOTIFICATION_CHANNEL,
} from '@/lib/graphql/operations/notifications';

/* eslint-disable @typescript-eslint/no-explicit-any */

const CHANNEL_TYPES = [
  { value: 'webhook', label: 'Webhook', icon: LinkIcon, desc: 'HTTP POST to any URL' },
  { value: 'email', label: 'Email', icon: EnvelopeIcon, desc: 'SMTP email delivery' },
  { value: 'dingtalk', label: 'DingTalk', icon: ChatBubbleLeftIcon, desc: 'DingTalk robot webhook' },
  { value: 'feishu', label: 'Feishu', icon: PaperAirplaneIcon, desc: 'Feishu (Lark) robot webhook' },
];

const CONFIG_TEMPLATES: Record<string, any> = {
  webhook: { url: '' },
  email: { host: '', port: 587, username: '', password: '', from: '', recipients: [] },
  dingtalk: { webhook_url: '', secret: '' },
  feishu: { webhook_url: '', secret: '' },
};

function NotificationChannelsPage() {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<any>(NOTIFICATION_CHANNELS_QUERY);
  const channels = useMemo(() => data?.notificationChannels || [], [data]);

  const [createMut] = useMutation(CREATE_NOTIFICATION_CHANNEL);
  const [updateMut] = useMutation(UPDATE_NOTIFICATION_CHANNEL);
  const [deleteMut] = useMutation(DELETE_NOTIFICATION_CHANNEL);
  const [testMut] = useMutation(TEST_NOTIFICATION_CHANNEL);

  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);

  // Form
  const [name, setName] = useState('');
  const [channelType, setChannelType] = useState('webhook');
  const [configJson, setConfigJson] = useState('');

  const resetForm = () => {
    setEditing(null);
    setName('');
    setChannelType('webhook');
    setConfigJson(JSON.stringify(CONFIG_TEMPLATES.webhook, null, 2));
  };

  const openCreate = () => { resetForm(); setShowModal(true); };
  const openEdit = (ch: any) => {
    setEditing(ch);
    setName(ch.name);
    setChannelType(ch.type);
    try { setConfigJson(JSON.stringify(JSON.parse(ch.config), null, 2)); }
    catch { setConfigJson(ch.config); }
    setShowModal(true);
  };

  const handleSave = async () => {
    if (!name.trim()) { toast.error(t('notification_channels.name_required')); return; }
    try { JSON.parse(configJson); } catch { toast.error(t('notification_channels.invalid_json')); return; }

    setSaving(true);
    try {
      if (editing) {
        await updateMut({ variables: { id: editing.id, input: { name, config: configJson } } });
        toast.success(t('notification_channels.channel_updated'));
      } else {
        await createMut({ variables: { input: { name, type: channelType, config: configJson } } });
        toast.success(t('notification_channels.channel_created'));
      }
      setShowModal(false);
      refetch();
    } catch (e: any) {
      toast.error(e.message || 'Failed to save');
    } finally { setSaving(false); }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm(t('notification_channels.delete_confirm'))) return;
    try {
      await deleteMut({ variables: { id } });
      toast.success(t('notification_channels.channel_deleted'));
      refetch();
    } catch { toast.error(t('common.error')); }
  };

  const handleToggle = async (ch: any) => {
    try {
      await updateMut({ variables: { id: ch.id, input: { isEnabled: !ch.isEnabled } } });
      toast.success(ch.isEnabled ? t('notification_channels.channel_disabled') : t('notification_channels.channel_enabled'));
      refetch();
    } catch { toast.error(t('common.error')); }
  };

  const handleTest = async (id: string) => {
    setTesting(id);
    try {
      await testMut({ variables: { id } });
      toast.success(t('notification_channels.test_success'));
    } catch (e: any) {
      toast.error(`${t('notification_channels.test_failed')}: ${e.message || t('common.error')}`);
    } finally { setTesting(null); }
  };

  if (loading) {
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
          <h1 className="text-2xl font-semibold text-apple-gray-900">{t('notification_channels.title')}</h1>
          <p className="text-apple-gray-500 mt-1">
            {t('notification_channels.subtitle')}
          </p>
        </div>
        {channels.length > 0 && (
          <button onClick={openCreate} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" /> {t('notification_channels.add_channel')}
          </button>
        )}
      </div>

      {channels.length === 0 ? (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card text-center py-16">
          <BellIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-apple-gray-900 mb-1">{t('notification_channels.no_channels')}</h3>
          <p className="text-apple-gray-500 text-sm mb-6">
            {t('notification_channels.no_channels_desc')}
          </p>
          <button onClick={openCreate} className="btn btn-primary rounded-xl">{t('notification_channels.add_first')}</button>
        </motion.div>
      ) : (
        <div className="grid gap-4">
          {channels.map((ch: any, i: number) => {
            const typeInfo = CHANNEL_TYPES.find((ct) => ct.value === ch.type);
            return (
              <motion.div
                key={ch.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.05 }}
                className="card p-5"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-xl bg-apple-gray-100 flex items-center justify-center">
                      {typeInfo ? <typeInfo.icon className="w-6 h-6 text-apple-gray-600" /> : <BellIcon className="w-6 h-6 text-apple-gray-600" />}
                    </div>
                    <div>
                      <h3 className="font-semibold text-apple-gray-900">{ch.name}</h3>
                      <p className="text-sm text-apple-gray-500">
                        {typeInfo?.label || ch.type} · {typeInfo?.desc}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => handleTest(ch.id)}
                      disabled={testing === ch.id || !ch.isEnabled}
                      className="btn btn-secondary text-sm px-3 py-1.5"
                      title="Send test notification"
                    >
                      <PaperAirplaneIcon className={`w-4 h-4 mr-1.5 ${testing === ch.id ? 'animate-pulse' : ''}`} />
                      {testing === ch.id ? t('common.sending') : t('common.test')}
                    </button>
                    <button
                      onClick={() => handleToggle(ch)}
                      className={`relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
                        ch.isEnabled ? 'bg-apple-green' : 'bg-apple-gray-200'
                      }`}
                    >
                      <span className={`pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow transition ${
                        ch.isEnabled ? 'translate-x-4' : 'translate-x-0'
                      }`} />
                    </button>
                    <button onClick={() => openEdit(ch)} className="inline-flex items-center gap-1 text-sm text-apple-gray-500 hover:text-apple-blue transition-colors">
                      <PencilIcon className="w-4 h-4" />
                      {t('common.edit')}
                    </button>
                    <button onClick={() => handleDelete(ch.id)} className="inline-flex items-center gap-1 text-sm text-apple-gray-400 hover:text-red-500 transition-colors">
                      <TrashIcon className="w-4 h-4" />
                      {t('common.delete')}
                    </button>
                  </div>
                </div>
              </motion.div>
            );
          })}
        </div>
      )}

      {/* Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">
              {editing ? t('notification_channels.edit_channel') : t('notification_channels.add_title')}
            </h2>
            <div className="space-y-4">
              <div>
                <label className="label">Name *</label>
                <input type="text" value={name} onChange={(e) => setName(e.target.value)} className="input" placeholder="My Webhook" />
              </div>

              {!editing && (
                <div>
                  <label className="label">{t('notification_channels.channel_type')}</label>
                  <div className="grid grid-cols-2 gap-3 mt-1">
                    {CHANNEL_TYPES.map((ct) => (
                      <button
                        key={ct.value}
                        type="button"
                        onClick={() => {
                          setChannelType(ct.value);
                          setConfigJson(JSON.stringify(CONFIG_TEMPLATES[ct.value], null, 2));
                        }}
                        className={`p-3 rounded-xl border text-center text-sm transition-colors ${
                          channelType === ct.value
                            ? 'border-apple-blue bg-apple-blue/5 text-apple-blue'
                            : 'border-apple-gray-200 text-apple-gray-600 hover:border-apple-gray-300'
                        }`}
                      >
                        <ct.icon className="w-5 h-5 mx-auto mb-1" />
                        {ct.label}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <label className="label">{t('notification_channels.config_json')}</label>
                <textarea
                  value={configJson}
                  onChange={(e) => setConfigJson(e.target.value)}
                  className="input font-mono text-xs h-40"
                  spellCheck={false}
                />
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-8">
              <button onClick={() => setShowModal(false)} className="btn btn-secondary">{t('common.cancel')}</button>
              <button onClick={handleSave} className="btn btn-primary" disabled={saving}>
                {saving ? t('common.saving') : t('notification_channels.save_channel')}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default NotificationChannelsPage;
