/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState, useEffect, useCallback, lazy, Suspense } from 'react';
import { motion } from 'framer-motion';
import clsx from 'clsx';
import { useQuery as useApolloQuery, useMutation as useApolloMutation } from '@apollo/client/react';
import { FEATURE_GATES_QUERY, UPDATE_FEATURE_GATE } from '@/lib/graphql/operations/featuregates';
import {
  GlobeAltIcon,
  ShieldCheckIcon,
  UserGroupIcon,
  EnvelopeIcon,
  CloudArrowUpIcon,
  CreditCardIcon,
  CheckCircleIcon,
  XCircleIcon,
  CloudIcon,
  KeyIcon,
  SignalIcon,
  EyeIcon,
  EyeSlashIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { SYSTEM_SETTINGS_QUERY, UPDATE_SYSTEM_SETTINGS, SEND_TEST_EMAIL, TRIGGER_BACKUP, SITE_CONFIG_QUERY } from '@/lib/graphql/operations/settings';
import { GET_INTEGRATIONS, UPDATE_INTEGRATION, TEST_LANGFUSE_CONNECTION } from '@/lib/graphql/operations/integrations';
import { useTranslation } from '@/lib/i18n';
import toast from 'react-hot-toast';

/* ── Tab definitions ── */

const settingsTabs = [
  { key: 'site', icon: GlobeAltIcon, labelKey: 'admin_settings.tabs.site' },
  { key: 'security', icon: ShieldCheckIcon, labelKey: 'admin_settings.tabs.security' },
  { key: 'defaults', icon: UserGroupIcon, labelKey: 'admin_settings.tabs.defaults' },
  { key: 'email', icon: EnvelopeIcon, labelKey: 'admin_settings.tabs.email' },
  { key: 'backup', icon: CloudArrowUpIcon, labelKey: 'admin_settings.tabs.backup' },
  { key: 'payment', icon: CreditCardIcon, labelKey: 'admin_settings.tabs.payment' },
  { key: 'sso', icon: KeyIcon, labelKey: 'admin_settings.tabs.sso' },
  { key: 'integrations', icon: CloudIcon, labelKey: 'admin_settings.tabs.integrations' },
  { key: 'featuregates', icon: SignalIcon, labelKey: 'Feature Gates' },
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
      {/* ── Stripe ── */}
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

      {/* ── WeChat Pay ── */}
      <div className="border-t border-apple-gray-200 pt-4 space-y-4">
        <h4 className="text-sm font-semibold text-apple-gray-900">微信支付 (WeChat Pay)</h4>
        <Toggle checked={data.wechatPayEnabled ?? false} onChange={(v) => onChange({ ...data, wechatPayEnabled: v })} label={t('admin_settings.payment.wechat_enabled')} />
        {data.wechatPayEnabled && (
          <div className="space-y-4 pl-1">
            <FormField label="App ID">
              <TextInput value={data.wechatPayAppId || ''} onChange={(v) => onChange({ ...data, wechatPayAppId: v })} placeholder="wx..." />
            </FormField>
            <FormField label="商户号 (Merchant ID)">
              <TextInput value={data.wechatPayMchId || ''} onChange={(v) => onChange({ ...data, wechatPayMchId: v })} placeholder="1900000000" />
            </FormField>
            <FormField label="API v3 密钥">
              <TextInput type="password" value={data.wechatPayApiV3Key || ''} onChange={(v) => onChange({ ...data, wechatPayApiV3Key: v })} placeholder="32位密钥..." />
            </FormField>
            <FormField label="商户证书序列号">
              <TextInput value={data.wechatPaySerialNo || ''} onChange={(v) => onChange({ ...data, wechatPaySerialNo: v })} placeholder="证书序列号..." />
            </FormField>
            <FormField label="商户私钥 (PEM)">
              <textarea
                value={data.wechatPayPrivateKey || ''}
                onChange={(e) => onChange({ ...data, wechatPayPrivateKey: e.target.value })}
                placeholder="-----BEGIN PRIVATE KEY-----&#10;..."
                rows={4}
                className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all resize-none"
              />
            </FormField>
            <FormField label="异步通知地址 (Notify URL)">
              <TextInput value={data.wechatPayNotifyUrl || ''} onChange={(v) => onChange({ ...data, wechatPayNotifyUrl: v })} placeholder="https://yourdomain.com/api/v1/payments/webhook/wechat-pay" />
            </FormField>
          </div>
        )}
      </div>

      {/* ── Alipay ── */}
      <div className="border-t border-apple-gray-200 pt-4 space-y-4">
        <h4 className="text-sm font-semibold text-apple-gray-900">支付宝 (Alipay)</h4>
        <Toggle checked={data.alipayEnabled ?? false} onChange={(v) => onChange({ ...data, alipayEnabled: v })} label={t('admin_settings.payment.alipay_enabled')} />
        {data.alipayEnabled && (
          <div className="space-y-4 pl-1">
            <FormField label="App ID">
              <TextInput value={data.alipayAppId || ''} onChange={(v) => onChange({ ...data, alipayAppId: v })} placeholder="2021000000000000" />
            </FormField>
            <FormField label="应用私钥 (Private Key PEM)">
              <textarea
                value={data.alipayPrivateKey || ''}
                onChange={(e) => onChange({ ...data, alipayPrivateKey: e.target.value })}
                placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;..."
                rows={4}
                className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all resize-none"
              />
            </FormField>
            <FormField label="支付宝公钥 (Alipay Public Key)">
              <textarea
                value={data.alipayPublicKey || ''}
                onChange={(e) => onChange({ ...data, alipayPublicKey: e.target.value })}
                placeholder="MIIBIjANBgkq..."
                rows={3}
                className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all resize-none"
              />
            </FormField>
            <FormField label="异步通知地址 (Notify URL)">
              <TextInput value={data.alipayNotifyUrl || ''} onChange={(v) => onChange({ ...data, alipayNotifyUrl: v })} placeholder="https://yourdomain.com/api/v1/payments/webhook/alipay" />
            </FormField>
            <Toggle checked={data.alipaySandbox ?? false} onChange={(v) => onChange({ ...data, alipaySandbox: v })} label="沙箱模式 (Sandbox)" />
          </div>
        )}
      </div>
    </div>
  );
}

const SsoManagementContent = lazy(() => import('@/pages/SsoManagementPage'));

function SsoSettingsTab({ data, onChange }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
  return (
    <div className="space-y-6">
      <p className="text-sm text-apple-gray-500">
        Configure OAuth2 social login providers. Users will see login buttons for enabled providers.
      </p>

      {/* GitHub */}
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <svg className="w-5 h-5 text-apple-gray-700" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
          </svg>
          <h4 className="text-sm font-semibold text-apple-gray-900 flex-1">GitHub</h4>
          <Toggle checked={data.githubEnabled ?? false} onChange={(v) => onChange({ ...data, githubEnabled: v })} label="" />
        </div>
        {data.githubEnabled && (
          <div className="space-y-4 pl-8">
            <FormField label="Client ID">
              <TextInput value={data.githubClientId || ''} onChange={(v) => onChange({ ...data, githubClientId: v })} placeholder="Ov23li..." />
            </FormField>
            <FormField label="Client Secret">
              <TextInput type="password" value={data.githubClientSecret || ''} onChange={(v) => onChange({ ...data, githubClientSecret: v })} placeholder="••••••••" />
            </FormField>
            <div className="bg-apple-gray-50 rounded-xl p-3">
              <p className="text-xs text-apple-gray-500">
                <strong>Callback URL:</strong>{' '}
                <code className="bg-apple-gray-100 px-1.5 py-0.5 rounded text-xs">
                  {window.location.origin}/auth/oauth2/github/callback
                </code>
              </p>
            </div>
          </div>
        )}
      </div>

      <div className="border-t border-apple-gray-200" />

      {/* Google */}
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <svg className="w-5 h-5" viewBox="0 0 24 24">
            <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
            <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
            <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
            <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
          </svg>
          <h4 className="text-sm font-semibold text-apple-gray-900 flex-1">Google</h4>
          <Toggle checked={data.googleEnabled ?? false} onChange={(v) => onChange({ ...data, googleEnabled: v })} label="" />
        </div>
        {data.googleEnabled && (
          <div className="space-y-4 pl-8">
            <FormField label="Client ID">
              <TextInput value={data.googleClientId || ''} onChange={(v) => onChange({ ...data, googleClientId: v })} placeholder="123456789.apps.googleusercontent.com" />
            </FormField>
            <FormField label="Client Secret">
              <TextInput type="password" value={data.googleClientSecret || ''} onChange={(v) => onChange({ ...data, googleClientSecret: v })} placeholder="••••••••" />
            </FormField>
            <div className="bg-apple-gray-50 rounded-xl p-3">
              <p className="text-xs text-apple-gray-500">
                <strong>Callback URL:</strong>{' '}
                <code className="bg-apple-gray-100 px-1.5 py-0.5 rounded text-xs">
                  {window.location.origin}/auth/oauth2/google/callback
                </code>
              </p>
            </div>
          </div>
        )}
    </div>

      {/* Enterprise SSO (OIDC / SAML) */}
      <div className="border-t border-apple-gray-200 pt-6">
        <h3 className="text-sm font-semibold text-apple-gray-900 mb-1">Enterprise SSO</h3>
        <p className="text-xs text-apple-gray-500 mb-4">Configure OIDC and SAML identity providers for organization-level single sign-on.</p>
        <Suspense fallback={<div className="text-center py-8 text-apple-gray-400 text-sm">Loading SSO configuration...</div>}>
          <SsoManagementContent />
        </Suspense>
      </div>
    </div>
  );
}

/* -- Feature Gates tab -- */

interface FeatureGateItem {
  name: string;
  enabled: boolean;
  category: string;
  description: string;
  source: string;
}

const categoryOrder = ['security', 'feature', 'observability'];
const categoryLabels: Record<string, string> = {
  security: 'Security',
  feature: 'Features',
  observability: 'Observability',
};

function FeatureGatesSettingsTab() {
  const { data, loading, refetch } = useApolloQuery<{ featureGates: FeatureGateItem[] }>(FEATURE_GATES_QUERY, {
    fetchPolicy: 'network-only',
  });
  const [updateGate] = useApolloMutation(UPDATE_FEATURE_GATE);

  const gates = data?.featureGates || [];

  const grouped = categoryOrder.reduce<Record<string, FeatureGateItem[]>>((acc, cat) => {
    acc[cat] = gates.filter((g) => g.category === cat);
    return acc;
  }, {});

  const handleToggle = async (name: string, currentValue: boolean) => {
    try {
      await updateGate({ variables: { name, enabled: !currentValue } });
      toast.success(`${name} ${!currentValue ? 'enabled' : 'disabled'}`);
      refetch();
    } catch (err: any) {
      toast.error(err?.message || 'Failed to update gate');
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <p className="text-sm text-apple-gray-500">
        Toggle platform capabilities at runtime. Changes take effect immediately.
      </p>
      {categoryOrder.map((cat) => {
        const items = grouped[cat];
        if (!items || items.length === 0) return null;
        return (
          <div key={cat} className="space-y-3">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-apple-gray-400">
              {categoryLabels[cat]}
            </h3>
            <div className="border border-apple-gray-200 rounded-xl divide-y divide-apple-gray-100">
              {items.map((gate) => (
                <div key={gate.name} className="flex items-center justify-between px-5 py-3.5">
                  <div className="flex-1 min-w-0 pr-4">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-apple-gray-900">{gate.name}</span>
                      {gate.source === 'database' && (
                        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-50 text-blue-600 border border-blue-200">
                          DB
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-apple-gray-500 mt-0.5 truncate">{gate.description}</p>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={gate.enabled}
                    onClick={() => handleToggle(gate.name, gate.enabled)}
                    className={clsx(
                      'relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200 flex-shrink-0 cursor-pointer',
                      gate.enabled ? 'bg-apple-blue' : 'bg-apple-gray-300'
                    )}
                  >
                    <span
                      className={clsx(
                        'inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform duration-200',
                        gate.enabled ? 'translate-x-6' : 'translate-x-1'
                      )}
                    />
                  </button>
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/* eslint-disable @typescript-eslint/no-explicit-any */
function IntegrationsSettingsTab() {
  const { data, loading, refetch } = useQuery<any>(GET_INTEGRATIONS, { fetchPolicy: 'cache-and-network' });
  const [updateIntegration] = useMutation(UPDATE_INTEGRATION);
  const [testLangfuse] = useMutation(TEST_LANGFUSE_CONNECTION);

  const integrations = (data?.integrations || []) as any[];

  const handleSave = async (name: string, enabled: boolean, configStr: string) => {
    try {
      JSON.parse(configStr);
      await updateIntegration({ variables: { name, input: { enabled, config: configStr } } });
      toast.success(`${name} configuration saved`);
      refetch();
    } catch (err: any) {
      toast.error(`Failed: ${err.message || 'Invalid JSON'}`);
    }
  };

  const handleTestLangfuse = async (publicKey: string, secretKey: string, host: string) => {
    try {
      const result: any = await testLangfuse({ variables: { publicKey, secretKey, host } });
      return result?.data?.testLangfuseConnection === true;
    } catch {
      return false;
    }
  };

  if (loading) return <div className="flex justify-center py-12"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" /></div>;

  return (
    <div className="space-y-6">
      <p className="text-sm text-apple-gray-500">Configure external logging, tracing, and metrics platforms.</p>
      {integrations.map((ig: any) => (
        ig.name === 'langfuse' ? (
          <LangfuseInlineCard key={ig.id} integration={ig} onSave={handleSave} onTestConnection={handleTestLangfuse} />
        ) : (
          <IntegrationInlineCard key={ig.id} integration={ig} onSave={handleSave} />
        )
      ))}
      {integrations.length === 0 && (
        <p className="text-sm text-apple-gray-400 text-center py-8">No integrations configured yet.</p>
      )}
    </div>
  );
}

function LangfuseInlineCard({ integration, onSave, onTestConnection }: {
  integration: any;
  onSave: (name: string, enabled: boolean, config: string) => void;
  onTestConnection: (publicKey: string, secretKey: string, host: string) => Promise<boolean>;
}) {
  const [enabled, setEnabled] = useState(integration.enabled);
  const [publicKey, setPublicKey] = useState('');
  const [secretKey, setSecretKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('https://cloud.langfuse.com');
  const [showSecret, setShowSecret] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<'idle' | 'success' | 'failed'>('idle');

  useEffect(() => {
    try {
      const cfg = JSON.parse(integration.config);
      if (cfg.publicKey) setPublicKey(cfg.publicKey);
      if (cfg.secretKey) setSecretKey(cfg.secretKey);
      if (cfg.baseUrl) setBaseUrl(cfg.baseUrl);
    } catch { /* ignore */ }
  }, [integration.config]);

  const handleSave = () => {
    const config = JSON.stringify({ publicKey, secretKey, baseUrl });
    onSave('langfuse', enabled, config);
  };

  const handleTest = async () => {
    if (!publicKey || !secretKey || !baseUrl) {
      toast.error('Please fill in all Langfuse fields before testing');
      return;
    }
    setTesting(true);
    setTestResult('idle');
    const ok = await onTestConnection(publicKey, secretKey, baseUrl);
    setTestResult(ok ? 'success' : 'failed');
    if (ok) {
      toast.success('Langfuse connection successful');
    } else {
      toast.error('Langfuse connection failed. Check your credentials and host.');
    }
    setTesting(false);
  };

  return (
    <div className="border border-apple-gray-200 rounded-xl p-5 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <SignalIcon className="w-5 h-5 text-orange-500" />
          <h4 className="text-sm font-semibold text-apple-gray-900">Langfuse</h4>
          {enabled ? (
            <span className="flex items-center text-xs font-medium text-emerald-600"><CheckCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Active</span>
          ) : (
            <span className="flex items-center text-xs font-medium text-apple-gray-400"><XCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Off</span>
          )}
        </div>
        <Toggle checked={enabled} onChange={setEnabled} label="" />
      </div>

      <p className="text-xs text-apple-gray-500">
        LLM observability and analytics. Traces, generations, and token usage are automatically reported.
      </p>

      <div className="space-y-3">
        <FormField label="Public Key">
          <TextInput value={publicKey} onChange={setPublicKey} placeholder="pk-lf-..." />
        </FormField>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-apple-gray-700">Secret Key</label>
          <div className="relative">
            <input
              type={showSecret ? 'text' : 'password'}
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
              placeholder="sk-lf-..."
              className="w-full px-3.5 py-2.5 pr-10 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all"
            />
            <button
              type="button"
              onClick={() => setShowSecret(!showSecret)}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-apple-gray-400 hover:text-apple-gray-600"
            >
              {showSecret ? <EyeSlashIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
            </button>
          </div>
        </div>

        <FormField label="Base URL">
          <TextInput value={baseUrl} onChange={setBaseUrl} placeholder="https://cloud.langfuse.com" />
        </FormField>
      </div>

      {/* Test Connection Button */}
      <button
        onClick={handleTest}
        disabled={testing}
        className={clsx(
          'w-full py-2.5 rounded-xl text-sm font-medium transition-all',
          testResult === 'success' ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
            : testResult === 'failed' ? 'bg-red-50 text-red-700 border border-red-200'
            : 'bg-apple-gray-50 text-apple-gray-700 border border-apple-gray-200 hover:bg-apple-gray-100',
          testing && 'opacity-60 cursor-wait'
        )}
      >
        {testing ? (
          <span className="flex items-center justify-center">
            <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />
            Testing...
          </span>
        ) : testResult === 'success' ? (
          <span className="flex items-center justify-center">
            <CheckCircleIcon className="w-4 h-4 mr-1.5" /> Connection Successful
          </span>
        ) : testResult === 'failed' ? (
          <span className="flex items-center justify-center">
            <XCircleIcon className="w-4 h-4 mr-1.5" /> Connection Failed — Retry
          </span>
        ) : (
          'Test Connection'
        )}
      </button>

      <div className="flex justify-end">
        <button
          onClick={handleSave}
          className="px-4 py-2 bg-apple-blue text-white rounded-xl text-sm font-semibold hover:bg-blue-600 transition-all"
        >
          Save
        </button>
      </div>
    </div>
  );
}

function IntegrationInlineCard({ integration, onSave }: {
  integration: any;
  onSave: (name: string, enabled: boolean, config: string) => void;
}) {
  const [enabled, setEnabled] = useState(integration.enabled);

  const getTemplate = (name: string) => {
    if (name === 'sentry') return '{\n  "dsn": "https://example@sentry.io/123"\n}';
    if (name === 'loki') return '{\n  "endpoint": "http://loki:3100/loki/api/v1/push"\n}';
    return '{\n  \n}';
  };

  const [configStr, setConfigStr] = useState(
    integration.config === '{}' ? getTemplate(integration.name) : JSON.stringify(JSON.parse(integration.config), null, 2)
  );

  return (
    <div className="border border-apple-gray-200 rounded-xl p-5 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <CloudIcon className="w-5 h-5 text-apple-gray-500" />
          <h4 className="text-sm font-semibold text-apple-gray-900 capitalize">{integration.name}</h4>
          {enabled ? (
            <span className="flex items-center text-xs font-medium text-emerald-600"><CheckCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Active</span>
          ) : (
            <span className="flex items-center text-xs font-medium text-apple-gray-400"><XCircleIcon className="w-3.5 h-3.5 mr-0.5" /> Off</span>
          )}
        </div>
        <Toggle checked={enabled} onChange={setEnabled} label="" />
      </div>
      <textarea
        value={configStr}
        onChange={(e) => setConfigStr(e.target.value)}
        className="w-full h-28 p-3 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm font-mono text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue resize-none"
        placeholder={getTemplate(integration.name)}
      />
      <div className="flex justify-end">
        <button
          onClick={() => onSave(integration.name, enabled, configStr)}
          className="px-4 py-2 bg-apple-blue text-white rounded-xl text-sm font-semibold hover:bg-blue-600 transition-all"
        >
          Save
        </button>
      </div>
    </div>
  );
}

/* ── Main settings page ── */

function AdminSettingsPage() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<TabKey>('site');
  const [formData, setFormData] = useState<Record<TabKey, any>>({
    site: {}, security: {}, defaults: {}, email: {}, backup: {}, payment: {}, sso: {}, integrations: {}, featuregates: {},
  });
  const [saved, setSaved] = useState(false);
  const [dirty, setDirty] = useState(false);

  const { data, loading } = useQuery<any>(SYSTEM_SETTINGS_QUERY, { fetchPolicy: 'network-only' });
  const [updateSettings, { loading: saving }] = useMutation<any>(UPDATE_SYSTEM_SETTINGS);

  // Initialize form data from server
  useEffect(() => {
    if (data?.systemSettings) {
      const s = data.systemSettings;
      const parsed: Record<TabKey, any> = { site: {}, security: {}, defaults: {}, email: {}, backup: {}, payment: {}, sso: {}, integrations: {}, featuregates: {} };
      for (const key of Object.keys(parsed) as TabKey[]) {
        try {
          // Map GraphQL 'oauth' field → frontend 'sso' tab
          const gqlKey = key === 'sso' ? 'oauth' : key;
          if (s[gqlKey]) parsed[key] = JSON.parse(s[gqlKey]);
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
    // Map frontend tab key to backend category
    const category = activeTab === 'sso' ? 'oauth' : activeTab;
    try {
      await updateSettings({
        variables: {
          input: {
            category,
            data: JSON.stringify(formData[activeTab]),
          },
        },
        // Refetch siteConfig so Layout sidebar updates immediately
        ...(activeTab === 'site' ? { refetchQueries: [{ query: SITE_CONFIG_QUERY }] } : {}),
      });
      setSaved(true);
      setDirty(false);
      toast.success(t('common.saved'));
      setTimeout(() => setSaved(false), 3000);
    } catch (err: any) {
      toast.error(err?.message || t('common.error'));
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
      case 'sso': return <SsoSettingsTab {...props} />;
      case 'integrations': return <IntegrationsSettingsTab />;
      case 'featuregates': return <FeatureGatesSettingsTab />;
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
        {/* Tab bar — Segmented Control */}
        <div className="px-4 pt-4 pb-2">
          <div className="segmented-control">
            {settingsTabs.map((tab) => (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={clsx(
                  'segmented-control-item',
                  activeTab === tab.key && 'segmented-control-item--active'
                )}
              >
                <tab.icon className="w-4 h-4" />
                {t(tab.labelKey)}
              </button>
            ))}
          </div>
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

              {/* Save button — hidden for tabs with independent save (featuregates, integrations) */}
              {activeTab !== 'featuregates' && activeTab !== 'integrations' && (
              <div className="mt-8 flex items-center gap-3">
                <button
                  onClick={handleSave}
                  disabled={saving || !dirty}
                  className={clsx(
                    'flex-1 py-3 rounded-xl text-sm font-semibold transition-all duration-200',
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
              )}
            </div>
          </motion.div>
        )}
      </div>
    </div>
  );
}

export default AdminSettingsPage;
