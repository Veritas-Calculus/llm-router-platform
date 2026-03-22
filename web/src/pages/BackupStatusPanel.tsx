import { useQuery, useMutation } from '@apollo/client/react';
import { gql } from '@apollo/client';
import { useTranslation } from '@/lib/i18n';
import {
  ExclamationTriangleIcon,
  ClockIcon,
  CircleStackIcon,
  ServerStackIcon,
  CloudArrowUpIcon,
} from '@heroicons/react/24/outline';

const BACKUP_STATUS_QUERY = gql`
  query BackupStatus {
    backupStatus {
      isConfigured
      scheduleEnabled
      lastBackup {
        id
        type
        status
        sizeBytes
        durationMs
        destination
        errorMessage
        startedAt
        completedAt
      }
      records {
        id
        type
        status
        sizeBytes
        durationMs
        destination
        errorMessage
        startedAt
        completedAt
      }
    }
  }
`;

const TRIGGER_BACKUP = gql`
  mutation TriggerBackup {
    triggerBackup
  }
`;

interface BackupRecord {
  id: string;
  type: string;
  status: string;
  sizeBytes: number;
  durationMs: number;
  destination: string;
  errorMessage?: string;
  startedAt: string;
  completedAt?: string;
}

interface BackupStatusData {
  backupStatus: {
    isConfigured: boolean;
    scheduleEnabled: boolean;
    lastBackup?: BackupRecord;
    records: BackupRecord[];
  };
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    success: 'bg-green-100 text-green-700',
    running: 'bg-blue-100 text-blue-700',
    failed: 'bg-red-100 text-red-700',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${colors[status] || 'bg-gray-100 text-gray-700'}`}>
      {status}
    </span>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString();
}

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export default function BackupStatusPanel() {
  const { t } = useTranslation();
  const { data, loading, error, refetch } = useQuery<BackupStatusData>(BACKUP_STATUS_QUERY, {
    pollInterval: 30000,
  });
  const [triggerBackup, { loading: triggering }] = useMutation(TRIGGER_BACKUP, {
    onCompleted: () => refetch(),
  });

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center py-20 text-apple-gray-400">
        <svg className="animate-spin h-6 w-6 mr-3" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        {t('common.loading')}
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-20 text-red-500">
        <p>{t('common.error')}: {error.message}</p>
      </div>
    );
  }

  const backup = data?.backupStatus;
  if (!backup) return null;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-apple-gray-900">{t('monitoring.backup_status')}</h2>
          <p className="text-sm text-apple-gray-400 mt-1">{t('monitoring.backup_status_desc')}</p>
        </div>
        <button
          onClick={() => triggerBackup()}
          disabled={triggering || !backup.isConfigured}
          className={`flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-xl transition-colors shadow-sm ${
            backup.isConfigured
              ? 'text-white bg-apple-blue hover:bg-blue-600'
              : 'text-apple-gray-400 bg-apple-gray-100 cursor-not-allowed'
          }`}
        >
          <CloudArrowUpIcon className="w-4 h-4" />
          {t('monitoring.trigger_backup')}
        </button>
      </div>

      {/* Config status banner */}
      {!backup.isConfigured && (
        <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 flex items-center gap-3">
          <div className="p-2 bg-amber-100 rounded-xl flex-shrink-0">
            <ExclamationTriangleIcon className="w-5 h-5 text-amber-600" />
          </div>
          <div>
            <span className="font-medium text-amber-800">{t('monitoring.backup_not_configured')}</span>
            <p className="text-sm text-amber-600 mt-0.5">{t('monitoring.backup_not_configured_desc')}</p>
          </div>
        </div>
      )}

      {/* Last Backup Card */}
      {backup.lastBackup && (
        <div className="bg-white rounded-2xl border border-apple-gray-200 shadow-sm p-6">
          <h3 className="text-base font-semibold text-apple-gray-900 mb-4 flex items-center gap-2">
            <ClockIcon className="w-5 h-5 text-apple-gray-400" />
            {t('monitoring.last_backup')}
          </h3>
          <div className="grid grid-cols-5 gap-6">
            <div className="text-center">
              <div className="text-xs text-apple-gray-400 mb-1">{t('monitoring.backup_status_label')}</div>
              <StatusBadge status={backup.lastBackup.status} />
            </div>
            <div className="text-center">
              <div className="text-xs text-apple-gray-400 mb-1">{t('monitoring.backup_type')}</div>
              <div className="text-sm font-medium text-apple-gray-900 capitalize">{backup.lastBackup.type}</div>
            </div>
            <div className="text-center">
              <div className="text-xs text-apple-gray-400 mb-1">{t('monitoring.backup_time')}</div>
              <div className="text-sm font-medium text-apple-gray-900">{timeAgo(backup.lastBackup.startedAt)}</div>
            </div>
            <div className="text-center">
              <div className="text-xs text-apple-gray-400 mb-1">{t('monitoring.backup_size')}</div>
              <div className="text-sm font-medium text-apple-gray-900">{formatBytes(backup.lastBackup.sizeBytes)}</div>
            </div>
            <div className="text-center">
              <div className="text-xs text-apple-gray-400 mb-1">{t('monitoring.backup_duration')}</div>
              <div className="text-sm font-medium text-apple-gray-900">{formatDuration(backup.lastBackup.durationMs)}</div>
            </div>
          </div>
          {backup.lastBackup.errorMessage && (
            <div className="mt-3 text-sm text-red-500 bg-red-50 rounded-lg p-2">
              {backup.lastBackup.errorMessage}
            </div>
          )}
        </div>
      )}

      {/* Backup History */}
      <div className="bg-white rounded-2xl border border-apple-gray-200 shadow-sm overflow-hidden">
        <div className="px-6 py-4 border-b border-apple-gray-100">
          <h3 className="text-base font-semibold text-apple-gray-900 flex items-center gap-2">
            <CircleStackIcon className="w-5 h-5 text-apple-gray-400" />
            {t('monitoring.backup_history')}
          </h3>
        </div>
        {backup.records.length === 0 ? (
          <div className="text-center py-12 text-apple-gray-400">
            <ServerStackIcon className="w-10 h-10 mx-auto mb-3 text-apple-gray-300" />
            <p>{t('monitoring.no_backups')}</p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="text-xs text-apple-gray-400 border-b border-apple-gray-100">
                <th className="px-6 py-3 text-left font-medium">{t('monitoring.backup_status_label')}</th>
                <th className="px-6 py-3 text-left font-medium">{t('monitoring.backup_type')}</th>
                <th className="px-6 py-3 text-left font-medium">{t('monitoring.backup_time')}</th>
                <th className="px-6 py-3 text-left font-medium">{t('monitoring.backup_size')}</th>
                <th className="px-6 py-3 text-left font-medium">{t('monitoring.backup_duration')}</th>
                <th className="px-6 py-3 text-left font-medium">{t('monitoring.backup_dest')}</th>
              </tr>
            </thead>
            <tbody>
              {backup.records.map((rec) => (
                <tr key={rec.id} className="border-b border-apple-gray-50 hover:bg-apple-gray-50 transition-colors">
                  <td className="px-6 py-3">
                    <StatusBadge status={rec.status} />
                  </td>
                  <td className="px-6 py-3 text-sm text-apple-gray-700 capitalize">{rec.type}</td>
                  <td className="px-6 py-3 text-sm text-apple-gray-700">{formatTime(rec.startedAt)}</td>
                  <td className="px-6 py-3 text-sm text-apple-gray-700">{formatBytes(rec.sizeBytes)}</td>
                  <td className="px-6 py-3 text-sm text-apple-gray-700">{formatDuration(rec.durationMs)}</td>
                  <td className="px-6 py-3 text-sm text-apple-gray-400 font-mono text-xs truncate max-w-[200px]" title={rec.destination}>
                    {rec.destination}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
