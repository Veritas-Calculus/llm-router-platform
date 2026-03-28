/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useEffect, useMemo } from 'react';
import { useLazyQuery } from '@apollo/client/react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { GET_REQUEST_LOGS } from '@/lib/graphql/operations/errorLogs';
import {
  MagnifyingGlassIcon,
  ExclamationCircleIcon,
  ExclamationTriangleIcon,
  InformationCircleIcon,
  ArrowPathIcon,
  Cog6ToothIcon,
  ClockIcon,
  ChevronDownIcon,
  AdjustmentsHorizontalIcon,
} from '@heroicons/react/24/outline';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { useTranslation } from '@/lib/i18n';

/* ── Time range presets ─────────────────────────────────────────────── */

interface TimePreset {
  key: string;
  labelKey: string;
  minutes: number;
}

const timePresets: TimePreset[] = [
  { key: '30m', labelKey: 'troubleshooting.time_30m', minutes: 30 },
  { key: '1h', labelKey: 'troubleshooting.time_1h', minutes: 60 },
  { key: '3h', labelKey: 'troubleshooting.time_3h', minutes: 180 },
  { key: '6h', labelKey: 'troubleshooting.time_6h', minutes: 360 },
  { key: '12h', labelKey: 'troubleshooting.time_12h', minutes: 720 },
  { key: '24h', labelKey: 'troubleshooting.time_24h', minutes: 1440 },
  { key: '7d', labelKey: 'troubleshooting.time_7d', minutes: 10080 },
];

const limitOptions = [50, 100, 200, 500, 1000];

const logLevels = [
  { key: '', labelKey: 'troubleshooting.level_all', color: 'text-apple-gray-600 bg-apple-gray-100 border-apple-gray-200' },
  { key: 'debug', labelKey: 'troubleshooting.level_debug', color: 'text-gray-600 bg-gray-100 border-gray-200' },
  { key: 'info', labelKey: 'troubleshooting.level_info', color: 'text-blue-700 bg-blue-100 border-blue-200' },
  { key: 'warn', labelKey: 'troubleshooting.level_warn', color: 'text-amber-700 bg-amber-100 border-amber-200' },
  { key: 'error', labelKey: 'troubleshooting.level_error', color: 'text-red-700 bg-red-100 border-red-200' },
];

export default function TroubleshootingPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [inputValue, setInputValue] = useState(searchParams.get('requestId') || '');

  // Time range state
  const [selectedPreset, setSelectedPreset] = useState('30m');
  const [customStart, setCustomStart] = useState('');
  const [customEnd, setCustomEnd] = useState('');
  const [showTimeDropdown, setShowTimeDropdown] = useState(false);
  const [resultLimit, setResultLimit] = useState(200);
  const [selectedLevel, setSelectedLevel] = useState('');
  const [expandedIdx, setExpandedIdx] = useState<number | null>(null);

  const [fetchLogs, { data, loading, error, called }] = useLazyQuery<any>(GET_REQUEST_LOGS, {
    fetchPolicy: 'network-only',
  });

  // Compute the time range variables based on selection
  const timeRange = useMemo(() => {
    if (selectedPreset === 'custom') {
      return {
        startTime: customStart || undefined,
        endTime: customEnd || undefined,
      };
    }
    const preset = timePresets.find((p) => p.key === selectedPreset);
    if (!preset) return {};
    const now = new Date();
    const start = new Date(now.getTime() - preset.minutes * 60 * 1000);
    return {
      startTime: start.toISOString(),
      endTime: now.toISOString(),
    };
  }, [selectedPreset, customStart, customEnd]);

  // Auto-trigger search if requestId comes from URL query param
  useEffect(() => {
    const requestId = searchParams.get('requestId');
    if (requestId) {
      setInputValue(requestId);
      fetchLogs({
        variables: { requestId, level: selectedLevel || undefined, ...timeRange, limit: resultLimit },
      });
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSearch = () => {
    const trimmed = inputValue.trim();
    const params: Record<string, string> = {};
    if (trimmed) params.requestId = trimmed;
    if (selectedLevel) params.level = selectedLevel;
    setSearchParams(params);
    fetchLogs({
      variables: {
        requestId: trimmed || undefined,
        level: selectedLevel || undefined,
        ...timeRange,
        limit: resultLimit,
      },
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') handleSearch();
  };

  const logs = data?.requestLogs || [];

  const currentPresetLabel = useMemo(() => {
    if (selectedPreset === 'custom') return t('troubleshooting.time_custom');
    const p = timePresets.find((p) => p.key === selectedPreset);
    return p ? t(p.labelKey) : selectedPreset;
  }, [selectedPreset, t]);

  const getLevelIcon = (level: string) => {
    switch (level?.toLowerCase()) {
      case 'error':
      case 'fatal':
      case 'dpanic':
        return <ExclamationCircleIcon className="w-5 h-5 text-red-500 flex-shrink-0" />;
      case 'warn':
      case 'warning':
        return <ExclamationTriangleIcon className="w-5 h-5 text-amber-500 flex-shrink-0" />;
      default:
        return <InformationCircleIcon className="w-5 h-5 text-blue-500 flex-shrink-0" />;
    }
  };

  const getLevelStyles = (level: string) => {
    switch (level?.toLowerCase()) {
      case 'error':
      case 'fatal':
      case 'dpanic':
        return 'border-red-200 bg-red-50/60';
      case 'warn':
      case 'warning':
        return 'border-amber-200 bg-amber-50/60';
      default:
        return 'border-gray-200 bg-white';
    }
  };

  const getLevelBadge = (level: string) => {
    switch (level?.toLowerCase()) {
      case 'error':
      case 'fatal':
      case 'dpanic':
        return 'bg-red-100 text-red-700 border-red-200';
      case 'warn':
      case 'warning':
        return 'bg-amber-100 text-amber-700 border-amber-200';
      case 'debug':
        return 'bg-gray-100 text-gray-600 border-gray-200';
      default:
        return 'bg-blue-100 text-blue-700 border-blue-200';
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">{t('troubleshooting.title')}</h1>
        <p className="text-apple-gray-500 mt-1">{t('troubleshooting.subtitle')}</p>
      </div>

      {/* Search Bar */}
      <div className="card p-5 space-y-4">
        {/* Main search row */}
        <div className="flex items-center gap-3">
          <div className="flex-1 relative">
            <MagnifyingGlassIcon className="absolute left-3.5 top-1/2 -translate-y-1/2 w-5 h-5 text-apple-gray-400" />
            <input
              type="text"
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t('troubleshooting.search_placeholder')}
              className="w-full pl-11 pr-4 py-3 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm text-apple-gray-900 placeholder:text-apple-gray-400 focus:outline-none focus:ring-2 focus:ring-apple-blue/30 focus:border-apple-blue transition-all font-mono"
            />
          </div>
          <button
            onClick={handleSearch}
            disabled={loading}
            className="px-6 py-3 bg-apple-blue text-white text-sm font-semibold rounded-xl shadow-sm hover:bg-blue-600 transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {loading ? (
              <>
                <ArrowPathIcon className="w-4 h-4 animate-spin" />
                {t('troubleshooting.searching')}
              </>
            ) : (
              t('troubleshooting.search')
            )}
          </button>
        </div>

        {/* Filters row */}
        <div className="flex items-center gap-3 flex-wrap">
          <AdjustmentsHorizontalIcon className="w-4 h-4 text-apple-gray-400" />

          {/* Time Range Selector */}
          <div className="relative">
            <button
              onClick={() => setShowTimeDropdown(!showTimeDropdown)}
              className="inline-flex items-center gap-2 px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm text-apple-gray-700 hover:bg-apple-gray-100 transition-colors"
            >
              <ClockIcon className="w-4 h-4 text-apple-gray-400" />
              <span>{currentPresetLabel}</span>
              <ChevronDownIcon className="w-3.5 h-3.5 text-apple-gray-400" />
            </button>

            <AnimatePresence>
              {showTimeDropdown && (
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  className="absolute z-20 mt-1 left-0 w-64 bg-white border border-apple-gray-200 rounded-xl shadow-lg overflow-hidden"
                >
                  <div className="p-2 space-y-0.5">
                    {timePresets.map((p) => (
                      <button
                        key={p.key}
                        onClick={() => {
                          setSelectedPreset(p.key);
                          setShowTimeDropdown(false);
                        }}
                        className={clsx(
                          'w-full text-left px-3 py-2 rounded-lg text-sm transition-colors',
                          selectedPreset === p.key
                            ? 'bg-apple-blue/10 text-apple-blue font-medium'
                            : 'text-apple-gray-700 hover:bg-apple-gray-50'
                        )}
                      >
                        {t(p.labelKey)}
                      </button>
                    ))}
                    <div className="border-t border-apple-gray-100 my-1" />
                    <button
                      onClick={() => {
                        setSelectedPreset('custom');
                        setShowTimeDropdown(false);
                      }}
                      className={clsx(
                        'w-full text-left px-3 py-2 rounded-lg text-sm transition-colors',
                        selectedPreset === 'custom'
                          ? 'bg-apple-blue/10 text-apple-blue font-medium'
                          : 'text-apple-gray-700 hover:bg-apple-gray-50'
                      )}
                    >
                      {t('troubleshooting.time_custom')}
                    </button>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          {/* Limit Selector */}
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-apple-gray-400">{t('troubleshooting.limit_label')}</span>
            <select
              value={resultLimit}
              onChange={(e) => setResultLimit(Number(e.target.value))}
              className="px-2 py-1.5 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue appearance-none cursor-pointer"
            >
              {limitOptions.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </div>

          {/* Level Filter */}
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-apple-gray-400">{t('troubleshooting.level_label')}</span>
            <div className="flex items-center gap-1">
              {logLevels.map((l) => (
                <button
                  key={l.key}
                  onClick={() => setSelectedLevel(l.key)}
                  className={clsx(
                    'px-2.5 py-1 rounded-md text-xs font-medium border transition-all',
                    selectedLevel === l.key
                      ? `${l.color} ring-2 ring-apple-blue/30`
                      : 'text-apple-gray-500 bg-white border-apple-gray-200 hover:bg-apple-gray-50'
                  )}
                >
                  {t(l.labelKey)}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Custom Time Range Inputs */}
        <AnimatePresence>
          {selectedPreset === 'custom' && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
                <div>
                  <label className="block text-xs font-medium text-apple-gray-500 mb-1">
                    {t('troubleshooting.custom_start')}
                  </label>
                  <input
                    type="datetime-local"
                    value={customStart ? customStart.slice(0, 16) : ''}
                    onChange={(e) => setCustomStart(e.target.value ? new Date(e.target.value).toISOString() : '')}
                    className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-apple-gray-500 mb-1">
                    {t('troubleshooting.custom_end')}
                  </label>
                  <input
                    type="datetime-local"
                    value={customEnd ? customEnd.slice(0, 16) : ''}
                    onChange={(e) => setCustomEnd(e.target.value ? new Date(e.target.value).toISOString() : '')}
                    className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-lg text-sm text-apple-gray-700 focus:outline-none focus:ring-2 focus:ring-apple-blue/20 focus:border-apple-blue"
                  />
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Results Area */}
      <AnimatePresence mode="wait">
        {/* Loading State */}
        {loading && (
          <motion.div
            key="loading"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="card p-12 text-center"
          >
            <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-apple-blue mx-auto mb-4" />
            <p className="text-apple-gray-500 text-sm">{t('troubleshooting.fetching')}</p>
          </motion.div>
        )}

        {/* Error State */}
        {!loading && error && (
          <motion.div
            key="error"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            className="card p-6 border-red-200 bg-red-50"
          >
            <div className="flex items-start gap-3">
              <ExclamationCircleIcon className="w-6 h-6 text-red-500 flex-shrink-0 mt-0.5" />
              <div>
                <h3 className="text-sm font-semibold text-red-800">{t('troubleshooting.error_title')}</h3>
                <p className="text-sm text-red-700 mt-1">{error.message}</p>
                {error.message.includes('not configured') && (
                  <button
                    onClick={() => navigate('/admin/settings')}
                    className="mt-3 inline-flex items-center gap-1.5 px-4 py-2 bg-white text-apple-gray-700 text-sm font-medium rounded-lg border border-apple-gray-200 hover:bg-apple-gray-50 transition-colors"
                  >
                    <Cog6ToothIcon className="w-4 h-4" />
                    {t('troubleshooting.go_to_settings')}
                  </button>
                )}
              </div>
            </div>
          </motion.div>
        )}

        {/* Empty State - no search yet */}
        {!loading && !error && !called && (
          <motion.div
            key="idle"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="card p-16 text-center"
          >
            <div className="w-16 h-16 bg-apple-gray-50 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-apple-gray-100">
              <MagnifyingGlassIcon className="w-8 h-8 text-apple-gray-300" />
            </div>
            <h3 className="text-base font-medium text-apple-gray-900 mb-1">{t('troubleshooting.idle_title')}</h3>
            <p className="text-sm text-apple-gray-500 max-w-md mx-auto">{t('troubleshooting.idle_desc')}</p>
          </motion.div>
        )}

        {/* Empty Results */}
        {!loading && !error && called && logs.length === 0 && (
          <motion.div
            key="empty"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            className="card p-12 text-center"
          >
            <InformationCircleIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-4" />
            <h3 className="text-base font-medium text-apple-gray-900 mb-1">{t('troubleshooting.no_results')}</h3>
            <p className="text-sm text-apple-gray-500 max-w-md mx-auto">{t('troubleshooting.no_results_desc')}</p>
          </motion.div>
        )}

        {/* Results List */}
        {!loading && !error && logs.length > 0 && (
          <motion.div
            key="results"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="space-y-3"
          >
            <div className="flex items-center justify-between">
              <p className="text-sm text-apple-gray-500">
                {t('troubleshooting.results_count', { count: logs.length })}
                {logs.length >= resultLimit && (
                  <span className="ml-1 text-amber-600 font-medium">
                    ({t('troubleshooting.limit_reached')})
                  </span>
                )}
              </p>
              <button
                onClick={handleSearch}
                className="text-sm text-apple-blue hover:text-blue-600 font-medium flex items-center gap-1 transition-colors"
              >
                <ArrowPathIcon className="w-3.5 h-3.5" />
                {t('troubleshooting.refresh')}
              </button>
            </div>

            {logs.map((log: any, idx: number) => {
              const isExpanded = expandedIdx === idx;
              const hasMethod = !!log.method;
              const statusColor = log.statusCode >= 500 ? 'text-red-600' : log.statusCode >= 400 ? 'text-amber-600' : 'text-green-600';

              return (
                <motion.div
                  key={idx}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: Math.min(idx * 0.02, 0.5) }}
                  className={`card border ${getLevelStyles(log.level)} transition-all hover:shadow-md cursor-pointer`}
                  onClick={() => setExpandedIdx(isExpanded ? null : idx)}
                >
                  {/* Compact Header */}
                  <div className="p-4">
                    <div className="flex items-center justify-between mb-1.5">
                      <div className="flex items-center gap-2 flex-wrap">
                        {getLevelIcon(log.level)}
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-bold uppercase tracking-widest border ${getLevelBadge(log.level)}`}>
                          {log.level}
                        </span>
                        {hasMethod && (
                          <>
                            <span className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-bold bg-purple-100 text-purple-700 border border-purple-200">
                              {log.method}
                            </span>
                            <span className="text-xs font-mono text-apple-gray-600 truncate max-w-[300px]">
                              {log.path}
                            </span>
                            {log.statusCode && (
                              <span className={`text-xs font-bold ${statusColor}`}>
                                {log.statusCode}
                              </span>
                            )}
                          </>
                        )}
                        {log.clientIp && (
                          <span className="text-[11px] font-mono text-apple-gray-400">
                            {log.clientIp}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-apple-gray-400 font-mono whitespace-nowrap">
                          {new Date(log.timestamp).toLocaleString()}
                        </span>
                        <ChevronDownIcon className={clsx('w-4 h-4 text-apple-gray-400 transition-transform', isExpanded && 'rotate-180')} />
                      </div>
                    </div>

                    {/* Log Message */}
                    <p className="text-sm text-apple-gray-800 font-medium break-words whitespace-pre-wrap pl-7">
                      {log.message}
                    </p>
                  </div>

                  {/* Expanded Detail Panel */}
                  <AnimatePresence>
                    {isExpanded && (
                      <motion.div
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: 'auto', opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ duration: 0.2 }}
                        className="overflow-hidden border-t border-apple-gray-100"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <div className="p-4 bg-apple-gray-50/80 space-y-4">
                          {/* Request Details Grid */}
                          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                            {log.requestId && (
                              <DetailField label="Request ID" value={log.requestId} mono />
                            )}
                            {log.method && (
                              <DetailField label={t('troubleshooting.detail_method')} value={log.method} />
                            )}
                            {log.path && (
                              <DetailField label={t('troubleshooting.detail_path')} value={log.path} mono />
                            )}
                            {log.statusCode != null && (
                              <DetailField label={t('troubleshooting.detail_status')} value={String(log.statusCode)} className={statusColor} />
                            )}
                            {log.latency != null && (
                              <DetailField label={t('troubleshooting.detail_latency')} value={`${(log.latency * 1000).toFixed(2)} ms`} />
                            )}
                            {log.clientIp && (
                              <DetailField label={t('troubleshooting.detail_client_ip')} value={log.clientIp} mono />
                            )}
                            {log.caller && (
                              <DetailField label={t('troubleshooting.detail_caller')} value={log.caller} mono />
                            )}
                            {log.userAgent && (
                              <DetailField label={t('troubleshooting.detail_user_agent')} value={log.userAgent} mono className="col-span-2 sm:col-span-3 md:col-span-4" />
                            )}
                          </div>

                          {/* Error Detail */}
                          {log.error && (
                            <div className="bg-red-50 border border-red-200 rounded-xl px-4 py-3">
                              <p className="text-[10px] font-bold uppercase tracking-wider text-red-500 mb-1">{t('troubleshooting.detail_error')}</p>
                              <p className="text-sm text-red-700 font-mono break-words whitespace-pre-wrap">
                                {log.error}
                              </p>
                            </div>
                          )}

                          {/* Raw JSON */}
                          {log.rawJson && (
                            <div className="rounded-xl overflow-hidden border border-apple-gray-200">
                              <div className="px-4 py-2 bg-[#1E1E1E] flex items-center justify-between">
                                <span className="text-xs font-medium text-[#D4D4D4]">{t('troubleshooting.detail_raw_json')}</span>
                                <button
                                  onClick={() => {
                                    navigator.clipboard.writeText(JSON.stringify(JSON.parse(log.rawJson), null, 2));
                                  }}
                                  className="text-[11px] text-apple-blue hover:text-blue-400 font-medium transition-colors"
                                >
                                  {t('troubleshooting.copy')}
                                </button>
                              </div>
                              <pre className="p-4 bg-[#1E1E1E] text-[#D4D4D4] text-xs font-mono overflow-x-auto m-0 whitespace-pre-wrap max-h-[300px] overflow-y-auto">
                                {(() => {
                                  try { return JSON.stringify(JSON.parse(log.rawJson), null, 2); }
                                  catch { return log.rawJson; }
                                })()}
                              </pre>
                            </div>
                          )}
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </motion.div>
              );
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

/* ── Detail Field Component ─────────────────────────────────────────── */
function DetailField({ label, value, mono, className }: { label: string; value: string; mono?: boolean; className?: string }) {
  return (
    <div className={className}>
      <p className="text-[10px] font-bold uppercase tracking-wider text-apple-gray-400 mb-0.5">{label}</p>
      <p className={clsx('text-sm text-apple-gray-800 break-all', mono && 'font-mono text-xs')}>{value}</p>
    </div>
  );
}
