import { useState, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import clsx from 'clsx';
import {
  GlobeAltIcon,
  ShieldCheckIcon,
  UserGroupIcon,
  EnvelopeIcon,
  CloudArrowUpIcon,
  CreditCardIcon,
  CheckCircleIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { SYSTEM_SETTINGS_QUERY, UPDATE_SYSTEM_SETTINGS, SEND_TEST_EMAIL, TRIGGER_BACKUP } from '@/lib/graphql/operations/settings';
import { useTranslation } from '@/lib/i18n';

/* ── Tab definitions ── */

const settingsTabs = [
  { key: 'site', icon: GlobeAltIcon, labelKey: 'admin_settings.tabs.site' },
  { key: 'security', icon: ShieldCheckIcon, labelKey: 'admin_settings.tabs.security' },
  { key: 'defaults', icon: UserGroupIcon, labelKey: 'admin_settings.tabs.defaults' },
  { key: 'email', icon: EnvelopeIcon, labelKey: 'admin_settings.tabs.email' },
  { key: 'backup', icon: CloudArrowUpIcon, labelKey: 'admin_settings.tabs.backup' },
  { key: 'payment', icon: CreditCardIcon, labelKey: 'admin_settings.tabs.payment' },
] as const;

type TabKey = typeof settingsTabs[number]['key'];

/* ── Shared form components ── */

function FormField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label className="block text-sm font-medium text-apple-gray-700">{label}</label>
      {children}
    </div>
  );
}

function TextInput({ value, onChange, placeholder, type = 'text' }: {
  value: string; onChange: (v: string) => void; placeholder?: string; type?: string;
}) {
  return (
    <input
      type={type}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all"
    />
  );
}

function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <label className="flex items-center justify-between cursor-pointer group">
      <span className="text-sm text-apple-gray-700">{label}</span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={clsx(
          'relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200',
          checked ? 'bg-apple-blue' : 'bg-apple-gray-300'
        )}
      >
        <span className={clsx(
          'inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform duration-200',
          checked ? 'translate-x-6' : 'translate-x-1'
        )} />
      </button>
    </label>
  );
}

function SelectInput({ value, onChange, options }: {
  value: string; onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm text-apple-gray-900 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all appearance-none"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>{o.label}</option>
      ))}
    </select>
  );
}

/* ── Tab content components ── */

function SiteSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  return (
    <div className="space-y-5">
      <FormField label={t('admin_settings.site.name')}>
        <TextInput value={data.siteName || ''} onChange={(v) => onChange({ ...data, siteName: v })} placeholder="LLM Router" />
      </FormField>
      <FormField label={t('admin_settings.site.subtitle_field')}>
        <TextInput value={data.subtitle || ''} onChange={(v) => onChange({ ...data, subtitle: v })} placeholder={t('admin_settings.site.subtitle_placeholder')} />
      </FormField>
      <FormField label={t('admin_settings.site.logo_url')}>
        <TextInput value={data.logoUrl || ''} onChange={(v) => onChange({ ...data, logoUrl: v })} placeholder="https://..." />
      </FormField>
      <FormField label={t('admin_settings.site.favicon_url')}>
        <TextInput value={data.faviconUrl || ''} onChange={(v) => onChange({ ...data, faviconUrl: v })} placeholder="https://..." />
      </FormField>
    </div>
  );
}

function SecuritySettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  return (
    <div className="space-y-5">
      <FormField label={t('admin_settings.security.registration_mode')}>
        <SelectInput
          value={data.registrationMode || 'closed'}
          onChange={(v) => onChange({ ...data, registrationMode: v })}
          options={[
            { value: 'open', label: t('admin_settings.security.mode_open') },
            { value: 'invite', label: t('admin_settings.security.mode_invite') },
            { value: 'closed', label: t('admin_settings.security.mode_closed') },
          ]}
        />
      </FormField>
      <div className="space-y-3 pt-2">
        <Toggle checked={data.emailVerification ?? false} onChange={(v) => onChange({ ...data, emailVerification: v })} label={t('admin_settings.security.email_verification')} />
        <Toggle checked={data.inviteOnly ?? false} onChange={(v) => onChange({ ...data, inviteOnly: v })} label={t('admin_settings.security.invite_only')} />
        <Toggle checked={data.couponEnabled ?? false} onChange={(v) => onChange({ ...data, couponEnabled: v })} label={t('admin_settings.security.coupon_enabled')} />
        <Toggle checked={data.twoFactorAuth ?? false} onChange={(v) => onChange({ ...data, twoFactorAuth: v })} label={t('admin_settings.security.two_factor')} />
        <Toggle checked={data.ssoEnabled ?? false} onChange={(v) => onChange({ ...data, ssoEnabled: v })} label={t('admin_settings.security.sso')} />
      </div>
    </div>
  );
}

function DefaultsSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  return (
    <div className="space-y-5">
      <FormField label={t('admin_settings.defaults.balance')}>
        <TextInput type="number" value={String(data.defaultBalance ?? 0)} onChange={(v) => onChange({ ...data, defaultBalance: parseFloat(v) || 0 })} placeholder="0.00" />
      </FormField>
      <FormField label={t('admin_settings.defaults.concurrency')}>
        <TextInput type="number" value={String(data.defaultConcurrency ?? 5)} onChange={(v) => onChange({ ...data, defaultConcurrency: parseInt(v) || 5 })} placeholder="5" />
      </FormField>
      <FormField label={t('admin_settings.defaults.plan')}>
        <TextInput value={data.defaultPlan || ''} onChange={(v) => onChange({ ...data, defaultPlan: v })} placeholder="free" />
      </FormField>
      <FormField label={t('admin_settings.defaults.rate_limit')}>
        <TextInput type="number" value={String(data.defaultRateLimit ?? 60)} onChange={(v) => onChange({ ...data, defaultRateLimit: parseInt(v) || 60 })} placeholder="60" />
      </FormField>
    </div>
  );
}

function EmailSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  const [testEmail, setTestEmail] = useState('');
  const [sendTestEmail, { loading: sending }] = useMutation<any>(SEND_TEST_EMAIL);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);

  const handleTestEmail = async () => {
    setTestResult(null);
    try {
      await sendTestEmail({ variables: { to: testEmail } });
      setTestResult({ ok: true, msg: t('admin_settings.email.test_success') });
    } catch (err: any) {
      setTestResult({ ok: false, msg: err?.message || 'Failed' });
    }
  };

  return (
    <div className="space-y-5">
      <Toggle checked={data.enabled ?? false} onChange={(v) => onChange({ ...data, enabled: v })} label={t('admin_settings.email.enabled')} />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FormField label={t('admin_settings.email.smtp_host')}>
          <TextInput value={data.host || ''} onChange={(v) => onChange({ ...data, host: v })} placeholder="smtp.example.com" />
        </FormField>
        <FormField label={t('admin_settings.email.smtp_port')}>
          <TextInput type="number" value={String(data.port ?? 587)} onChange={(v) => onChange({ ...data, port: parseInt(v) || 587 })} placeholder="587" />
        </FormField>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FormField label={t('admin_settings.email.smtp_user')}>
          <TextInput value={data.username || ''} onChange={(v) => onChange({ ...data, username: v })} placeholder="user@example.com" />
        </FormField>
        <FormField label={t('admin_settings.email.smtp_pass')}>
          <TextInput type="password" value={data.password || ''} onChange={(v) => onChange({ ...data, password: v })} placeholder="••••••••" />
        </FormField>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FormField label={t('admin_settings.email.from_address')}>
          <TextInput value={data.from || ''} onChange={(v) => onChange({ ...data, from: v })} placeholder="noreply@example.com" />
        </FormField>
        <FormField label={t('admin_settings.email.from_name')}>
          <TextInput value={data.fromName || ''} onChange={(v) => onChange({ ...data, fromName: v })} placeholder="LLM Router" />
        </FormField>
      </div>

      {/* Test Email */}
      <div className="border-t border-apple-gray-200 pt-4 space-y-3">
        <h4 className="text-sm font-semibold text-apple-gray-900">{t('admin_settings.email.test_title')}</h4>
        <div className="flex gap-3 items-end">
          <div className="flex-1">
            <FormField label={t('admin_settings.email.test_to')}>
              <TextInput value={testEmail} onChange={setTestEmail} placeholder="test@example.com" />
            </FormField>
          </div>
          <button
            onClick={handleTestEmail}
            disabled={sending || !testEmail}
            className="px-4 py-2.5 bg-apple-gray-100 text-apple-gray-700 rounded-xl text-sm font-medium hover:bg-apple-gray-200 transition-colors disabled:opacity-50"
          >
            {sending ? t('admin_settings.email.test_sending') : t('admin_settings.email.test_send')}
          </button>
        </div>
        {testResult && (
          <p className={clsx('text-sm font-medium', testResult.ok ? 'text-green-600' : 'text-red-500')}>
            {testResult.msg}
          </p>
        )}
      </div>
    </div>
  );
}

function BackupSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  const [triggerBackup, { loading: triggering }] = useMutation<any>(TRIGGER_BACKUP);
  const [backupResult, setBackupResult] = useState<{ ok: boolean; msg: string } | null>(null);

  const handleTriggerBackup = async () => {
    setBackupResult(null);
    try {
      await triggerBackup();
      setBackupResult({ ok: true, msg: t('admin_settings.backup.trigger_success') });
    } catch (err: any) {
      setBackupResult({ ok: false, msg: err?.message || 'Failed' });
    }
  };

  return (
    <div className="space-y-5">
      <Toggle checked={data.enabled ?? false} onChange={(v) => onChange({ ...data, enabled: v })} label={t('admin_settings.backup.enabled')} />
      <FormField label={t('admin_settings.backup.s3_endpoint')}>
        <TextInput value={data.s3Endpoint || ''} onChange={(v) => onChange({ ...data, s3Endpoint: v })} placeholder="https://s3.amazonaws.com" />
      </FormField>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FormField label={t('admin_settings.backup.s3_bucket')}>
          <TextInput value={data.s3Bucket || ''} onChange={(v) => onChange({ ...data, s3Bucket: v })} placeholder="my-backups" />
        </FormField>
        <FormField label={t('admin_settings.backup.s3_prefix')}>
          <TextInput value={data.s3Prefix || ''} onChange={(v) => onChange({ ...data, s3Prefix: v })} placeholder="llm-router/" />
        </FormField>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FormField label={t('admin_settings.backup.access_key')}>
          <TextInput value={data.accessKey || ''} onChange={(v) => onChange({ ...data, accessKey: v })} placeholder="AKIA..." />
        </FormField>
        <FormField label={t('admin_settings.backup.secret_key')}>
          <TextInput type="password" value={data.secretKey || ''} onChange={(v) => onChange({ ...data, secretKey: v })} placeholder="••••••••" />
        </FormField>
      </div>
      <FormField label={t('admin_settings.backup.schedule')}>
        <SelectInput
          value={data.schedule || 'daily'}
          onChange={(v) => onChange({ ...data, schedule: v })}
          options={[
            { value: 'hourly', label: t('admin_settings.backup.schedule_hourly') },
            { value: 'daily', label: t('admin_settings.backup.schedule_daily') },
            { value: 'weekly', label: t('admin_settings.backup.schedule_weekly') },
            { value: 'monthly', label: t('admin_settings.backup.schedule_monthly') },
          ]}
        />
      </FormField>

      {/* Manual Backup */}
      <div className="border-t border-apple-gray-200 pt-4 space-y-3">
        <div className="flex items-center gap-3">
          <button
            onClick={handleTriggerBackup}
            disabled={triggering || !data.enabled}
            className="px-4 py-2.5 bg-apple-gray-100 text-apple-gray-700 rounded-xl text-sm font-medium hover:bg-apple-gray-200 transition-colors disabled:opacity-50"
          >
            {triggering ? t('admin_settings.backup.triggering') : t('admin_settings.backup.trigger_now')}
          </button>
          {backupResult && (
            <span className={clsx('text-sm font-medium', backupResult.ok ? 'text-green-600' : 'text-red-500')}>
              {backupResult.msg}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

function PaymentSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  return (
    <div className="space-y-5">
      <div className="space-y-4">
        <h4 className="text-sm font-semibold text-apple-gray-900">Stripe</h4>
        <Toggle checked={data.stripeEnabled ?? false} onChange={(v) => onChange({ ...data, stripeEnabled: v })} label={t('admin_settings.payment.stripe_enabled')} />
        {data.stripeEnabled && (
          <div className="space-y-4 pl-1">
            <FormField label={t('admin_settings.payment.stripe_pk')}>
              <TextInput value={data.stripePublishableKey || ''} onChange={(v) => onChange({ ...data, stripePublishableKey: v })} placeholder="pk_..." />
            </FormField>
            <FormField label={t('admin_settings.payment.stripe_sk')}>
              <TextInput type="password" value={data.stripeSecretKey || ''} onChange={(v) => onChange({ ...data, stripeSecretKey: v })} placeholder="sk_..." />
            </FormField>
            <FormField label={t('admin_settings.payment.stripe_webhook')}>
              <TextInput type="password" value={data.stripeWebhookSecret || ''} onChange={(v) => onChange({ ...data, stripeWebhookSecret: v })} placeholder="whsec_..." />
            </FormField>
          </div>
        )}
      </div>
      <div className="border-t border-apple-gray-200 pt-4 space-y-3">
        <Toggle checked={data.wechatPayEnabled ?? false} onChange={(v) => onChange({ ...data, wechatPayEnabled: v })} label={t('admin_settings.payment.wechat_enabled')} />
        <Toggle checked={data.alipayEnabled ?? false} onChange={(v) => onChange({ ...data, alipayEnabled: v })} label={t('admin_settings.payment.alipay_enabled')} />
      </div>
    </div>
  );
}

/* ── Main settings page ── */

function AdminSettingsPage() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<TabKey>('site');
  const [formData, setFormData] = useState<Record<TabKey, any>>({
    site: {}, security: {}, defaults: {}, email: {}, backup: {}, payment: {},
  });
  const [saved, setSaved] = useState(false);
  const [dirty, setDirty] = useState(false);

  const { data, loading } = useQuery<any>(SYSTEM_SETTINGS_QUERY, { fetchPolicy: 'network-only' });
  const [updateSettings, { loading: saving }] = useMutation<any>(UPDATE_SYSTEM_SETTINGS);

  // Initialize form data from server
  useEffect(() => {
    if (data?.systemSettings) {
      const s = data.systemSettings;
      const parsed: Record<TabKey, any> = { site: {}, security: {}, defaults: {}, email: {}, backup: {}, payment: {} };
      for (const key of Object.keys(parsed) as TabKey[]) {
        try {
          if (s[key]) parsed[key] = JSON.parse(s[key]);
        } catch { /* empty */ }
      }
      setFormData(parsed);
    }
  }, [data]);

  const handleChange = useCallback((tabData: any) => {
    setFormData((prev) => ({ ...prev, [activeTab]: tabData }));
    setDirty(true);
    setSaved(false);
  }, [activeTab]);

  const handleSave = async () => {
    try {
      await updateSettings({
        variables: {
          input: {
            category: activeTab,
            data: JSON.stringify(formData[activeTab]),
          },
        },
      });
      setSaved(true);
      setDirty(false);
      setTimeout(() => setSaved(false), 3000);
    } catch (err) {
      console.error('Failed to save settings:', err);
    }
  };

  const renderTabContent = () => {
    const tabData = formData[activeTab] || {};
    const props = { data: tabData, onChange: handleChange, t };
    switch (activeTab) {
      case 'site': return <SiteSettingsTab {...props} />;
      case 'security': return <SecuritySettingsTab {...props} />;
      case 'defaults': return <DefaultsSettingsTab {...props} />;
      case 'email': return <EmailSettingsTab {...props} />;
      case 'backup': return <BackupSettingsTab {...props} />;
      case 'payment': return <PaymentSettingsTab {...props} />;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">{t('admin_settings.title')}</h1>
          <p className="mt-1 text-apple-gray-500">{t('admin_settings.subtitle')}</p>
        </div>
      </div>

      <div className="card overflow-hidden">
        {/* Tab bar */}
        <div className="border-b border-apple-gray-200 px-4 flex gap-1 overflow-x-auto">
          {settingsTabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={clsx(
                'flex items-center gap-2 px-4 py-3 text-sm font-medium whitespace-nowrap border-b-2 transition-colors',
                activeTab === tab.key
                  ? 'border-apple-blue text-apple-blue'
                  : 'border-transparent text-apple-gray-500 hover:text-apple-gray-700 hover:border-apple-gray-300'
              )}
            >
              <tab.icon className="w-4 h-4" />
              {t(tab.labelKey)}
            </button>
          ))}
        </div>

        {/* Tab content */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
          </div>
        ) : (
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, x: 10 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.2 }}
            className="p-6"
          >
            <div className="max-w-2xl">
              {renderTabContent()}

              {/* Save button */}
              <div className="mt-8 flex items-center gap-3">
                <button
                  onClick={handleSave}
                  disabled={saving || !dirty}
                  className={clsx(
                    'px-6 py-2.5 rounded-xl text-sm font-semibold transition-all duration-200',
                    dirty
                      ? 'bg-apple-blue text-white hover:bg-blue-600 shadow-sm'
                      : 'bg-apple-gray-100 text-apple-gray-400 cursor-not-allowed'
                  )}
                >
                  {saving ? t('common.saving') : t('common.save')}
                </button>
                {saved && (
                  <motion.span
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    className="flex items-center gap-1.5 text-sm text-green-600 font-medium"
                  >
                    <CheckCircleIcon className="w-4 h-4" />
                    {t('common.saved')}
                  </motion.span>
                )}
              </div>
            </div>
          </motion.div>
        )}
      </div>
    </div>
  );
}

export default AdminSettingsPage;
