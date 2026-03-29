import { useState, useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { useNavigate } from 'react-router-dom';
import { GET_ERROR_LOGS } from '@/lib/graphql/operations/errorLogs';
import {
  ExclamationCircleIcon,
  XMarkIcon,
  DocumentDuplicateIcon,
  CloudArrowUpIcon
} from '@heroicons/react/24/outline';
import { motion, AnimatePresence } from 'framer-motion';
import toast from 'react-hot-toast';
import { useTranslation } from '@/lib/i18n';

interface ErrorLog {
  id: string;
  trajectoryId: string;
  traceId: string;
  provider: string;
  model: string;
  statusCode: number;
  headers: string;
  responseBody: string;
  createdAt: string;
}

interface ErrorLogsData {
  errorLogs: { data: ErrorLog[]; total: number };
}

export default function ErrorLogsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const pageSize = 20;
  const [selectedLog, setSelectedLog] = useState<ErrorLog | null>(null);

  const { data, loading } = useQuery<ErrorLogsData>(GET_ERROR_LOGS, {
    variables: { page, pageSize },
    fetchPolicy: 'cache-and-network',
  });

  const logs = useMemo(() => data?.errorLogs?.data || [], [data]);
  const total = data?.errorLogs?.total || 0;
  const totalPages = Math.ceil(total / pageSize);

  const formatDate = (dateStr: string) => {
    if (!dateStr || dateStr === '0001-01-01T00:00:00Z') return 'Unknown';
    return new Date(dateStr).toLocaleString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  };

  const copySherlog = (log: ErrorLog) => {
    const formatStr = `Trajectory ID: ${log.trajectoryId}
Error: HTTP ${log.statusCode} upstream
Sherlog: 
TraceID: ${log.traceId}
Provider: ${log.provider}
Model: ${log.model}
Headers: ${log.headers}

${log.responseBody}`;
    navigator.clipboard.writeText(formatStr);
    toast.success('Sherlog copied to clipboard!');
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Error Logs</h1>
        <p className="text-apple-gray-500 mt-1">Review upstream routing failures and copy telemetry</p>
      </div>

      <div className="card overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-apple-gray-200">
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Time</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Trace / Trajectory</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Provider / Model</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Status</th>
              <th className="text-right py-3 px-4 text-sm font-medium text-apple-gray-500">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5} className="py-12 text-center text-apple-gray-400">
                  <div className="flex justify-center items-center h-full">
                    <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
                  </div>
                </td>
              </tr>
            ) : logs.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-16 text-center">
                  <div className="w-16 h-16 bg-gray-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
                    <ExclamationCircleIcon className="w-8 h-8 text-apple-gray-400" />
                  </div>
                  <p className="text-apple-gray-500">No error logs found.</p>
                </td>
              </tr>
            ) : (
              logs.map((log: ErrorLog, idx: number) => (
                <motion.tr
                  key={log.id}
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: idx * 0.02 }}
                  className="border-b border-apple-gray-100 hover:bg-apple-gray-50 transition-colors cursor-pointer"
                  onClick={() => setSelectedLog(log)}
                >
                  <td className="py-3 px-4 text-sm text-apple-gray-500 whitespace-nowrap">
                    {formatDate(log.createdAt)}
                  </td>
                  <td className="py-3 px-4 text-xs font-mono text-apple-gray-600 truncate max-w-[200px]" title={log.trajectoryId}>
                    <div>{log.traceId}</div>
                    <div className="opacity-60">{log.trajectoryId}</div>
                  </td>
                  <td className="py-3 px-4 text-sm text-apple-gray-700">
                    <div className="font-medium capitalize">{log.provider}</div>
                    <div className="text-xs text-apple-gray-500">{log.model}</div>
                  </td>
                  <td className="py-3 px-4">
                    <span className="inline-flex flex-shrink-0 items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 border border-red-200 shadow-sm">
                      HTTP {log.statusCode}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-right">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copySherlog(log);
                      }}
                      className="p-1.5 text-apple-gray-400 hover:text-apple-blue hover:bg-apple-gray-100 rounded-lg transition-colors inline-block"
                      title={t('error_logs.copy_sherlog')}
                    >
                      <DocumentDuplicateIcon className="w-5 h-5" />
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        toast.success('Pushed to Integrations!');
                      }}
                      className="ml-1 p-1.5 text-apple-gray-400 hover:text-apple-blue hover:bg-apple-gray-100 rounded-lg transition-colors inline-block"
                      title={t('error_logs.push_integrations')}
                    >
                      <CloudArrowUpIcon className="w-5 h-5" />
                    </button>
                  </td>
                </motion.tr>
              ))
            )}
          </tbody>
        </table>

        {/* Pagination */ }
        {totalPages > 1 && (
          <div className="px-4 py-4 border-t border-apple-gray-100 flex items-center justify-between">
            <span className="text-sm text-apple-gray-500">
              Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, total)} of {total} results
            </span>
            <div className="flex items-center gap-2">
              <button
                disabled={page === 1}
                onClick={() => setPage(p => Math.max(1, p - 1))}
                className="px-3 py-1.5 rounded-lg text-sm font-medium text-apple-gray-600 hover:bg-apple-gray-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <span className="text-sm font-medium text-apple-gray-900 border border-apple-gray-200 bg-apple-gray-50 px-3 py-1.5 rounded-lg">
                Page {page} of {totalPages}
              </span>
              <button
                disabled={page === totalPages}
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                className="px-3 py-1.5 rounded-lg text-sm font-medium text-apple-gray-600 hover:bg-apple-gray-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Detail Drawer */}
      <AnimatePresence>
        {selectedLog && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 bg-black/20 backdrop-blur-sm z-40 transition-opacity"
              onClick={() => setSelectedLog(null)}
            />
            <motion.div
              initial={{ x: '100%', opacity: 0.5 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: '100%', opacity: 0.5 }}
              transition={{ type: 'spring', damping: 25, stiffness: 200 }}
              className="fixed inset-y-0 right-0 w-full max-w-2xl bg-white shadow-2xl z-50 flex flex-col border-l border-apple-gray-200"
            >
              <div className="flex items-center justify-between p-6 border-b border-apple-gray-200 bg-white/80 backdrop-blur-xl">
                <div>
                  <h2 className="text-xl font-semibold text-apple-gray-900">Error Details</h2>
                  <p className="text-sm font-mono text-apple-gray-500 mt-1">{selectedLog.traceId}</p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => navigate(`/admin/troubleshooting?requestId=${encodeURIComponent(selectedLog.traceId)}`)}
                    className="px-4 py-2 bg-apple-blue text-white text-sm font-medium rounded-lg shadow-sm hover:bg-blue-600 transition-colors"
                  >
                    View Trace
                  </button>
                  <button
                    onClick={() => setSelectedLog(null)}
                    className="p-2 text-apple-gray-400 hover:text-apple-gray-600 hover:bg-apple-gray-100 rounded-full transition-colors"
                  >
                    <XMarkIcon className="w-6 h-6" />
                  </button>
                </div>
              </div>

              <div className="flex-1 overflow-y-auto p-6 space-y-6 bg-apple-gray-50">
                <div className="grid grid-cols-2 gap-4">
                  <div className="bg-white p-4 rounded-xl shadow-sm border border-apple-gray-100">
                    <label className="text-xs font-semibold text-apple-gray-400 uppercase tracking-wider block mb-1">Provider</label>
                    <div className="text-sm font-medium text-apple-gray-900 capitalize">{selectedLog.provider}</div>
                  </div>
                  <div className="bg-white p-4 rounded-xl shadow-sm border border-apple-gray-100">
                    <label className="text-xs font-semibold text-apple-gray-400 uppercase tracking-wider block mb-1">Status Code</label>
                    <div className="text-sm font-medium text-red-600">HTTP {selectedLog.statusCode}</div>
                  </div>
                </div>
                
                <div className="bg-white p-4 rounded-xl shadow-sm border border-apple-gray-100">
                    <label className="text-xs font-semibold text-apple-gray-400 uppercase tracking-wider block mb-1">Trajectory ID</label>
                    <div className="text-sm font-mono text-apple-gray-700">{selectedLog.trajectoryId}</div>
                </div>

                <div className="bg-white rounded-xl shadow-sm border border-apple-gray-100 overflow-hidden">
                  <div className="px-4 py-3 border-b border-apple-gray-100 flex items-center justify-between">
                    <h3 className="text-sm font-medium text-apple-gray-900">Response Headers</h3>
                  </div>
                  <pre className="p-4 bg-apple-gray-50 text-xs font-mono text-apple-gray-700 overflow-x-auto m-0 whitespace-pre-wrap">
                    {(() => {
                        try {
                            return JSON.stringify(JSON.parse(selectedLog.headers), null, 2);
                        } catch {
                            return selectedLog.headers;
                        }
                    })()}
                  </pre>
                </div>

                <div className="bg-white rounded-xl shadow-sm border border-apple-gray-100 overflow-hidden">
                  <div className="px-4 py-3 border-b border-apple-gray-100 flex items-center justify-between bg-[#1E1E1E]">
                    <h3 className="text-sm font-medium text-[#D4D4D4]">Original Response Body</h3>
                    <button
                      onClick={() => copySherlog(selectedLog)}
                      className="text-xs font-medium text-apple-blue hover:text-blue-400 transition-colors"
                    >
                       Copy Sherlog
                    </button>
                  </div>
                  <pre className="p-4 bg-[#1E1E1E] text-[#D4D4D4] text-xs font-mono overflow-x-auto m-0 whitespace-pre-wrap rounded-b-xl border-t border-[#333]">
                    {(() => {
                        try {
                            return JSON.stringify(JSON.parse(selectedLog.responseBody), null, 2);
                        } catch {
                            return selectedLog.responseBody;
                        }
                    })()}
                  </pre>
                </div>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </div>
  );
}
