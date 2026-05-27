/* eslint-disable @typescript-eslint/no-explicit-any */
import { useQuery } from '@apollo/client/react';
import clsx from 'clsx';
import { CAPTCHA_CONFIG } from '@/lib/graphql/operations';
import { FormField, HelpText } from './FormPrimitives';

interface TabProps {
  data: any;
  onChange: (d: any) => void;
  t: (k: string) => string;
}

// CaptchaSettingsTab lets the operator pick a captcha backend at runtime.
// The provider lives in the DB-backed system_settings table (settings
// category "captcha"); credentials (site key / secret key) stay in env
// vars because they're paired with secrets and shouldn't be editable
// through the admin UI.
//
// Selection takes effect within 5 minutes (Registry cache TTL) and
// immediately on save.
export function CaptchaSettingsTab({ data, onChange, t }: TabProps) {
  const { data: configData } = useQuery(CAPTCHA_CONFIG, { fetchPolicy: 'cache-first' });
  const siteKey = configData?.captchaConfig?.siteKey ?? '';
  // We cannot probe the secret key — it's server-side only. The proxy
  // signals "configured" by reporting enabled=true *and* providing a
  // siteKey for the real backends. For dev/disabled, no key is needed,
  // so we mark them as configured by default.
  const provider: string = data.provider || 'dev';
  const secretLikelyConfigured = provider === 'dev' || provider === 'disabled' || siteKey !== '';

  const options = [
    { value: 'dev', labelKey: 'admin_settings.captcha.provider_dev' },
    { value: 'hcaptcha', labelKey: 'admin_settings.captcha.provider_hcaptcha' },
    { value: 'turnstile', labelKey: 'admin_settings.captcha.provider_turnstile' },
    { value: 'disabled', labelKey: 'admin_settings.captcha.provider_disabled' },
  ];

  return (
    <div className="space-y-6">
      <p className="text-sm text-apple-gray-500">{t('admin_settings.captcha.desc')}</p>

      <FormField label={t('admin_settings.captcha.provider')}>
        <div className="space-y-2">
          {options.map((opt) => {
            const checked = provider === opt.value;
            return (
              <label
                key={opt.value}
                className={clsx(
                  'flex items-center gap-3 rounded-xl border px-3.5 py-2.5 cursor-pointer transition-all',
                  checked ? 'border-apple-blue bg-apple-blue/5' : 'border-apple-gray-200 bg-apple-gray-50 hover:bg-apple-gray-100'
                )}
              >
                <input
                  type="radio"
                  name="captcha-provider"
                  value={opt.value}
                  checked={checked}
                  onChange={() => onChange({ ...data, provider: opt.value })}
                  className="w-4 h-4 text-apple-blue focus:ring-apple-blue/30"
                />
                <span className="text-sm text-apple-gray-900">{t(opt.labelKey)}</span>
              </label>
            );
          })}
        </div>
      </FormField>

      {(provider === 'hcaptcha' || provider === 'turnstile') && !secretLikelyConfigured && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3">
          <p className="text-sm text-amber-800">{t('admin_settings.captcha.warn_no_secret')}</p>
        </div>
      )}

      <div className="border-t border-apple-gray-200 pt-5 space-y-3">
        <h4 className="text-sm font-semibold text-apple-gray-900">{t('admin_settings.captcha.credentials_title')}</h4>
        <HelpText>{t('admin_settings.captcha.credentials_desc')}</HelpText>
        <div className="space-y-2 pt-1">
          <CredentialRow
            label={t('admin_settings.captcha.site_key_label')}
            valueShown={siteKey || '—'}
            configured={!!siteKey}
            configuredLabel={t('admin_settings.captcha.configured')}
            notConfiguredLabel={t('admin_settings.captcha.not_configured')}
          />
          <CredentialRow
            label={t('admin_settings.captcha.secret_key_label')}
            // Secret key is server-side only — we never expose the value.
            valueShown={secretLikelyConfigured ? '••••••' : '—'}
            configured={secretLikelyConfigured}
            configuredLabel={t('admin_settings.captcha.configured')}
            notConfiguredLabel={t('admin_settings.captcha.not_configured')}
          />
        </div>
      </div>
    </div>
  );
}

interface CredentialRowProps {
  label: string;
  valueShown: string;
  configured: boolean;
  configuredLabel: string;
  notConfiguredLabel: string;
}

function CredentialRow({ label, valueShown, configured, configuredLabel, notConfiguredLabel }: CredentialRowProps) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-xl bg-apple-gray-50 border border-apple-gray-200 px-3.5 py-2.5">
      <div className="min-w-0">
        <p className="text-sm font-medium text-apple-gray-900">{label}</p>
        <p className="text-xs text-apple-gray-500 truncate font-mono">{valueShown}</p>
      </div>
      <span
        className={clsx(
          'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium',
          configured ? 'bg-green-100 text-green-700' : 'bg-apple-gray-200 text-apple-gray-600'
        )}
      >
        <span
          className={clsx(
            'h-1.5 w-1.5 rounded-full',
            configured ? 'bg-green-500' : 'bg-apple-gray-400'
          )}
        />
        {configured ? configuredLabel : notConfiguredLabel}
      </span>
    </div>
  );
}

export default CaptchaSettingsTab;
