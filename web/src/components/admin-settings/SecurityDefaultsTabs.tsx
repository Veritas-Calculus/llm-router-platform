/* eslint-disable @typescript-eslint/no-explicit-any */
import { FormField, TextInput, Toggle, SelectInput } from './FormPrimitives';

export function SecuritySettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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

export function DefaultsSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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

export default SecuritySettingsTab;
