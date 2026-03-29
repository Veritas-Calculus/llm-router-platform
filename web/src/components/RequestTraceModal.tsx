import { Fragment } from 'react';
import { useQuery } from '@apollo/client/react';
import { Dialog, Transition } from '@headlessui/react';
import { XMarkIcon, ExclamationTriangleIcon, InformationCircleIcon, ExclamationCircleIcon } from '@heroicons/react/24/outline';
import { GET_REQUEST_LOGS } from '../lib/graphql/operations/errorLogs';
import { useTranslation } from 'react-i18next';

interface RequestTraceModalProps {
  isOpen: boolean;
  onClose: () => void;
  requestId: string | null;
}

interface RequestLogEntry {
  level: string;
  timestamp: string;
  message: string;
  caller?: string;
  error?: string;
}

interface RequestLogsData {
  requestLogs: RequestLogEntry[];
}

export default function RequestTraceModal({ isOpen, onClose, requestId }: RequestTraceModalProps) {
  const { t } = useTranslation();
  const { data, loading, error } = useQuery<RequestLogsData>(GET_REQUEST_LOGS, {
    variables: { requestId },
    skip: !isOpen || !requestId,
    fetchPolicy: 'network-only',
  });

  const getLevelIcon = (level: string) => {
    switch (level?.toLowerCase()) {
      case 'error':
      case 'fatal':
        return <ExclamationCircleIcon className="w-5 h-5 text-red-500" />;
      case 'warn':
      case 'warning':
        return <ExclamationTriangleIcon className="w-5 h-5 text-amber-500" />;
      default:
        return <InformationCircleIcon className="w-5 h-5 text-blue-500" />;
    }
  };

  const getLevelColor = (level: string) => {
    switch (level?.toLowerCase()) {
      case 'error':
      case 'fatal':
        return 'bg-red-50 text-red-700 border-red-200';
      case 'warn':
      case 'warning':
        return 'bg-amber-50 text-amber-700 border-amber-200';
      default:
        return 'bg-white text-gray-800 border-gray-200';
    }
  };

  return (
    <Transition appear show={isOpen} as={Fragment}>
      <Dialog as="div" className="relative z-50" onClose={onClose}>
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-300"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-200"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-black/30 backdrop-blur-sm" />
        </Transition.Child>

        <div className="fixed inset-0 overflow-y-auto">
          <div className="flex min-h-full items-center justify-center p-4 text-center">
            <Transition.Child
              as={Fragment}
              enter="ease-out duration-300"
              enterFrom="opacity-0 scale-95"
              enterTo="opacity-100 scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 scale-100"
              leaveTo="opacity-0 scale-95"
            >
              <Dialog.Panel className="w-full max-w-4xl transform overflow-hidden rounded-[20px] bg-white p-6 text-left align-middle shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all flex flex-col max-h-[85vh]">
                <div className="flex justify-between items-center mb-4 pb-4 border-b border-gray-100">
                  <Dialog.Title as="h3" className="text-lg font-semibold leading-6 text-gray-900 flex items-center gap-2">
                    {t('Trace Logs')}
                    {requestId && <span className="text-sm font-normal text-gray-500 bg-gray-100 px-2 py-1 rounded-md font-mono">{requestId}</span>}
                  </Dialog.Title>
                  <button
                    onClick={onClose}
                    className="rounded-full p-2 hover:bg-gray-100 transition-colors focus:outline-none"
                  >
                    <XMarkIcon className="w-5 h-5 text-gray-400" />
                  </button>
                </div>

                <div className="overflow-y-auto flex-1 bg-[#F5F5F7] rounded-xl p-4 border border-gray-200 font-mono text-[13px] leading-relaxed relative">
                  {loading && (
                    <div className="flex flex-col justify-center items-center h-full text-gray-500 py-12">
                      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mb-3"></div>
                      <p>{t('Fetching trace logs from Loki...')}</p>
                    </div>
                  )}

                  {error && (
                    <div className="text-red-700 bg-red-50 p-4 rounded-xl flex items-start border border-red-100 my-4">
                      <ExclamationCircleIcon className="w-6 h-6 mr-3 flex-shrink-0" />
                      <div>
                        <p className="font-semibold">{t('Failed to load trace')}</p>
                        <p className="text-sm mt-1 opacity-90">{error.message}</p>
                      </div>
                    </div>
                  )}

                  {!loading && !error && (!data?.requestLogs || data.requestLogs.length === 0) && (
                    <div className="text-center text-gray-500 my-8 py-12 bg-white rounded-xl border border-gray-200 shadow-sm">
                      <InformationCircleIcon className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                      <p className="text-base font-medium text-gray-900">{t('No logs found for this request ID')}</p>
                      <p className="text-sm text-gray-500 mt-2 max-w-md mx-auto">
                         {t('It may take a few seconds for logs to be ingested into Loki, or the log retention period may have passed.')}
                      </p>
                    </div>
                  )}

                  {!loading && !error && data?.requestLogs && data.requestLogs.length > 0 && (
                    <div className="space-y-3">
                      {data.requestLogs.map((log: RequestLogEntry, idx: number) => (
                        <div key={idx} className={`p-4 rounded-xl border ${getLevelColor(log.level)} shadow-sm transition-all hover:shadow-md`}>
                          <div className="flex items-start justify-between mb-2">
                            <div className="flex items-center space-x-2">
                              {getLevelIcon(log.level)}
                              <span className="font-bold uppercase text-xs tracking-wider">
                                {log.level}
                              </span>
                            </div>
                            <span className="text-xs opacity-60 font-medium">
                              {new Date(log.timestamp).toLocaleString()}
                            </span>
                          </div>
                          
                          <p className="mt-1 text-gray-800 break-words whitespace-pre-wrap font-medium">
                            {log.message}
                          </p>
                          
                          {(log.caller || log.error) && (
                            <div className="mt-3 pt-3 border-t border-opacity-10 border-current space-y-1.5 bg-black/5 p-3 rounded-lg">
                              {log.caller && (
                                <p className="text-xs opacity-80 flex gap-2">
                                  <span className="font-semibold uppercase tracking-wider text-[10px] opacity-70 mt-0.5">Caller</span>
                                  <span>{log.caller}</span>
                                </p>
                              )}
                              {log.error && (
                                <p className="text-xs text-red-600 font-medium flex gap-2">
                                  <span className="font-semibold uppercase tracking-wider text-[10px] opacity-70 mt-0.5">Error</span>
                                  <span>{log.error}</span>
                                </p>
                              )}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </div>
      </Dialog>
    </Transition>
  );
}
