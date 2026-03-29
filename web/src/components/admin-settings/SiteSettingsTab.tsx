/* eslint-disable @typescript-eslint/no-explicit-any */
import { FormField, TextInput } from './FormPrimitives';

export function SiteSettingsTab({ data, onChange, t }: { data: any; onChange: (d: any) => void; t: (k: string) => string }) {
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

export default SiteSettingsTab;
