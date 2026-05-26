import { useState } from 'react';
import { useTranslation } from '@/lib/i18n';
import HealthPage from './HealthPage';
import SlaDashboardPage from './SlaDashboardPage';
import SlaAlertsPage from './SlaAlertsPage';
import SystemStatusPanel from './SystemStatusPanel';
import SystemLoadPanel from './SystemLoadPanel';
import BackupStatusPanel from './BackupStatusPanel';

const tabs = [
  { key: 'system', labelKey: 'nav.system_status' },
  { key: 'load', labelKey: 'monitoring.load_monitoring' },
  { key: 'backup', labelKey: 'monitoring.backup_status' },
  { key: 'health', labelKey: 'nav.health' },
  { key: 'sla', labelKey: 'nav.sla' },
  { key: 'alerts', labelKey: 'nav.sla_alerts' },
] as const;

type TabKey = (typeof tabs)[number]['key'];

export default function MonitoringPage() {
  const { t } = useTranslation();
  const [active, setActive] = useState<TabKey>('system');

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">{t('monitoring.title')}</h1>
        <p className="mt-1 text-apple-gray-500">{t('monitoring.subtitle')}</p>
      </div>

      {/* Tab bar */}
      <div className="-mx-1 overflow-x-auto px-1 pb-1">
        <div className="inline-flex min-w-max items-center gap-1 bg-apple-gray-100 p-1 rounded-xl border border-apple-gray-200">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActive(tab.key)}
              className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
                active === tab.key
                  ? 'bg-white text-apple-blue shadow-sm'
                  : 'text-apple-gray-500 hover:text-apple-gray-700'
              }`}
            >
              {t(tab.labelKey)}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      {active === 'system' && <SystemStatusPanel />}
      {active === 'load' && <SystemLoadPanel />}
      {active === 'backup' && <BackupStatusPanel />}
      {active === 'health' && <HealthPage />}
      {active === 'sla' && <SlaDashboardPage />}
      {active === 'alerts' && <SlaAlertsPage />}
    </div>
  );
}
