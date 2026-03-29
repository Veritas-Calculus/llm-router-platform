/* eslint-disable @typescript-eslint/no-explicit-any */
import { lazy, Suspense } from 'react';
import { FormField, TextInput, Toggle } from './FormPrimitives';

const SsoManagementContent = lazy(() => import('@/pages/SsoManagementPage'));

export function PaymentSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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
              <textarea value={data.wechatPayPrivateKey || ''} onChange={(e) => onChange({ ...data, wechatPayPrivateKey: e.target.value })}
                placeholder={"-----BEGIN PRIVATE KEY-----\n..."} rows={4}
                className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all resize-none" />
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
              <textarea value={data.alipayPrivateKey || ''} onChange={(e) => onChange({ ...data, alipayPrivateKey: e.target.value })}
                placeholder={"-----BEGIN RSA PRIVATE KEY-----\n..."} rows={4}
                className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all resize-none" />
            </FormField>
            <FormField label="支付宝公钥 (Alipay Public Key)">
              <textarea value={data.alipayPublicKey || ''} onChange={(e) => onChange({ ...data, alipayPublicKey: e.target.value })}
                placeholder="MIIBIjANBgkq..." rows={3}
                className="w-full px-3.5 py-2.5 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-mono text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all resize-none" />
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

export function SsoSettingsTab({ data, onChange }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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
                <code className="bg-apple-gray-100 px-1.5 py-0.5 rounded text-xs">{window.location.origin}/auth/oauth2/github/callback</code>
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
                <code className="bg-apple-gray-100 px-1.5 py-0.5 rounded text-xs">{window.location.origin}/auth/oauth2/google/callback</code>
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

export default PaymentSettingsTab;
