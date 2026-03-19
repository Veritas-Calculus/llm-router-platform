import { motion, AnimatePresence } from 'framer-motion';
import {
  PlayIcon,
  PencilIcon,
  TrashIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  XCircleIcon,
  LockClosedIcon,
  LinkIcon,
} from '@heroicons/react/24/outline';
import { Proxy } from '@/lib/types';

interface TestResult {
  id: string;
  is_healthy: boolean;
  latency_ms: number;
  error?: string;
}

interface ProxyTableProps {
  proxies: Proxy[];
  testResults: Record<string, TestResult>;
  testingId: string | null;
  deleteConfirmId: string | null;
  deleting: boolean;
  onTest: (id: string) => void;
  onEdit: (proxy: Proxy) => void;
  onToggle: (id: string) => void;
  onDeleteClick: (id: string) => void;
  onConfirmDelete: (id: string) => void;
  onCancelDelete: () => void;
}

export default function ProxyTable({
  proxies,
  testResults,
  testingId,
  deleteConfirmId,
  deleting,
  onTest,
  onEdit,
  onToggle,
  onDeleteClick,
  onConfirmDelete,
  onCancelDelete,
}: ProxyTableProps) {
  const formatDate = (dateString: string): string => {
    if (!dateString) return 'Never';
    const date = new Date(dateString);
    if (date.getTime() === 0) return 'Never';
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getSuccessRate = (proxy: Proxy): string => {
    const total = proxy.success_count + proxy.failure_count;
    if (total === 0) return '-';
    return ((proxy.success_count / total) * 100).toFixed(1) + '%';
  };

  const getProxyHealth = (proxy: Proxy) => {
    const testResult = testResults[proxy.id];
    if (testResult) {
      return testResult.is_healthy;
    }
    const total = proxy.success_count + proxy.failure_count;
    if (total === 0) return null;
    return (proxy.success_count / total) > 0.5;
  };

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-apple-gray-200">
        <thead>
          <tr>
            <th className="table-header">URL</th>
            <th className="table-header">Type</th>
            <th className="table-header">Region</th>
            <th className="table-header">Status</th>
            <th className="table-header">Health</th>
            <th className="table-header">Success Rate</th>
            <th className="table-header">Latency</th>
            <th className="table-header">Last Checked</th>
            <th className="table-header">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-apple-gray-100">
          {proxies.map((proxy) => {
            const healthStatus = getProxyHealth(proxy);
            const testResult = testResults[proxy.id];
            const upstreamProxy = proxy.upstream_proxy_id
              ? proxies.find((p) => p.id === proxy.upstream_proxy_id)
              : null;

            return (
              <tr key={proxy.id} className="hover:bg-apple-gray-50">
                <td className="table-cell font-medium font-mono text-sm">
                  <div className="flex items-center gap-2">
                    {proxy.url}
                    {proxy.has_auth && (
                      <LockClosedIcon className="w-4 h-4 text-apple-blue" title="Authenticated" />
                    )}
                    {upstreamProxy && (
                      <span
                        className="inline-flex items-center gap-1 text-xs text-apple-gray-500"
                        title={`Via: ${upstreamProxy.url}`}
                      >
                        <LinkIcon className="w-3 h-3" />
                        <span className="max-w-[100px] truncate">{upstreamProxy.url.split('://').pop()?.split(':')[0] || upstreamProxy.url}</span>
                      </span>
                    )}
                  </div>
                </td>
                <td className="table-cell uppercase text-sm">{proxy.type}</td>
                <td className="table-cell">{proxy.region || '-'}</td>
                <td className="table-cell">
                  <button
                    onClick={() => onToggle(proxy.id)}
                    className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium transition-colors ${
                      proxy.is_active
                        ? 'bg-green-100 text-apple-green hover:bg-green-200'
                        : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
                    }`}
                  >
                    {proxy.is_active ? 'Active' : 'Inactive'}
                  </button>
                </td>
                <td className="table-cell">
                  {healthStatus === null ? (
                    <span className="text-apple-gray-400">-</span>
                  ) : healthStatus ? (
                    <CheckCircleIcon className="w-5 h-5 text-apple-green" />
                  ) : (
                    <XCircleIcon className="w-5 h-5 text-apple-red" />
                  )}
                </td>
                <td className="table-cell">{getSuccessRate(proxy)}</td>
                <td className="table-cell">
                  {testResult
                    ? `${testResult.latency_ms}ms`
                    : proxy.avg_latency > 0
                    ? `${proxy.avg_latency.toFixed(0)}ms`
                    : '-'}
                </td>
                <td className="table-cell text-apple-gray-500 text-sm">
                  {formatDate(proxy.last_checked)}
                </td>
                <td className="table-cell">
                  <AnimatePresence mode="wait">
                    {deleteConfirmId === proxy.id ? (
                      <motion.div
                        key="confirm"
                        initial={{ opacity: 0, x: 10 }}
                        animate={{ opacity: 1, x: 0 }}
                        exit={{ opacity: 0, x: -10 }}
                        className="flex items-center gap-2"
                      >
                        <span className="text-sm text-apple-gray-600">Delete?</span>
                        <button
                          onClick={() => onConfirmDelete(proxy.id)}
                          className="px-2 py-1 text-xs bg-apple-red text-white rounded hover:bg-red-600 transition-colors"
                          disabled={deleting}
                        >
                          {deleting ? 'Deleting...' : 'Yes'}
                        </button>
                        <button
                          onClick={onCancelDelete}
                          className="px-2 py-1 text-xs bg-apple-gray-200 text-apple-gray-700 rounded hover:bg-apple-gray-300 transition-colors"
                          disabled={deleting}
                        >
                          No
                        </button>
                      </motion.div>
                    ) : (
                      <motion.div
                        key="actions"
                        initial={{ opacity: 0, x: -10 }}
                        animate={{ opacity: 1, x: 0 }}
                        exit={{ opacity: 0, x: 10 }}
                        className="flex items-center gap-2"
                      >
                        <button
                          onClick={() => onTest(proxy.id)}
                          className="text-apple-blue hover:text-blue-600 transition-colors"
                          title="Test proxy"
                          disabled={testingId === proxy.id}
                        >
                          {testingId === proxy.id ? (
                            <ArrowPathIcon className="w-5 h-5 animate-spin" />
                          ) : (
                            <PlayIcon className="w-5 h-5" />
                          )}
                        </button>
                        <button
                          onClick={() => onEdit(proxy)}
                          className="text-apple-gray-500 hover:text-apple-gray-700 transition-colors"
                          title="Edit proxy"
                        >
                          <PencilIcon className="w-5 h-5" />
                        </button>
                        <button
                          onClick={() => onDeleteClick(proxy.id)}
                          className="text-apple-red hover:text-red-600 transition-colors"
                          title="Delete proxy"
                        >
                          <TrashIcon className="w-5 h-5" />
                        </button>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
