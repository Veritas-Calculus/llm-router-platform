/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState } from 'react';
import clsx from 'clsx';
import { useMutation } from '@apollo/client/react';
import { SEND_TEST_EMAIL, TRIGGER_BACKUP } from '@/lib/graphql/operations/settings';
import { FormField, TextInput, Toggle, SelectInput } from './FormPrimitives';

export function EmailSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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
      <div className="border-t border-apple-gray-200 pt-4 space-y-3">
        <h4 className="text-sm font-semibold text-apple-gray-900">{t('admin_settings.email.test_title')}</h4>
        <div className="flex gap-3 items-end">
          <div className="flex-1">
            <FormField label={t('admin_settings.email.test_to')}>
              <TextInput value={testEmail} onChange={setTestEmail} placeholder="test@example.com" />
            </FormField>
          </div>
          <button onClick={handleTestEmail} disabled={sending || !testEmail}
            className="px-4 py-2.5 bg-apple-gray-100 text-apple-gray-700 rounded-xl text-sm font-medium hover:bg-apple-gray-200 transition-colors disabled:opacity-50">
            {sending ? t('admin_settings.email.test_sending') : t('admin_settings.email.test_send')}
          </button>
        </div>
        {testResult && (
          <p className={clsx('text-sm font-medium', testResult.ok ? 'text-green-600' : 'text-red-500')}>{testResult.msg}</p>
        )}
      </div>
    </div>
  );
}

export function BackupSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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
      <div className="border-t border-apple-gray-200 pt-4 space-y-3">
        <div className="flex items-center gap-3">
          <button onClick={handleTriggerBackup} disabled={triggering || !data.enabled}
            className="px-4 py-2.5 bg-apple-gray-100 text-apple-gray-700 rounded-xl text-sm font-medium hover:bg-apple-gray-200 transition-colors disabled:opacity-50">
            {triggering ? t('admin_settings.backup.triggering') : t('admin_settings.backup.trigger_now')}
          </button>
          {backupResult && (
            <span className={clsx('text-sm font-medium', backupResult.ok ? 'text-green-600' : 'text-red-500')}>{backupResult.msg}</span>
          )}
        </div>
      </div>
    </div>
  );
}

export default EmailSettingsTab;
