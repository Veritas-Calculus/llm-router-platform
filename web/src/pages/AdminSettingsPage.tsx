/* eslint-disable @typescript-eslint/no-explicit-any */

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
  KeyIcon,
  CloudIcon,
  SignalIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { SYSTEM_SETTINGS_QUERY, UPDATE_SYSTEM_SETTINGS, SITE_CONFIG_QUERY } from '@/lib/graphql/operations/settings';
import { useTranslation } from '@/lib/i18n';
import toast from 'react-hot-toast';
import {
  SiteSettingsTab,
  SecuritySettingsTab,
  DefaultsSettingsTab,
  EmailSettingsTab,
  BackupSettingsTab,
  PaymentSettingsTab,
  SsoSettingsTab,
  FeatureGatesSettingsTab,
  IntegrationsSettingsTab,
} from '@/components/admin-settings';

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
