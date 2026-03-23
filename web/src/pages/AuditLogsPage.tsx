import { useState, useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { GET_AUDIT_LOGS } from '@/lib/graphql/operations/audit';
import {
  ShieldExclamationIcon,
} from '@heroicons/react/24/outline';
import { motion } from 'framer-motion';

export default function AuditLogsPage() {
  const [page, setPage] = useState(1);
  const pageSize = 20;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const { data, loading } = useQuery<any>(GET_AUDIT_LOGS, {
    variables: { page, pageSize },
    fetchPolicy: 'cache-and-network',
  });

  const logs = useMemo(() => data?.auditLogs?.data || [], [data]);
  const total = data?.auditLogs?.total || 0;
  const totalPages = Math.ceil(total / pageSize);

  const formatDate = (dateStr: string) => {
    if (!dateStr || dateStr === '0001-01-01T00:00:00Z') return 'Unknown';
    return new Date(dateStr).toLocaleString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Audit Logs</h1>
        <p className="text-apple-gray-500 mt-1">Review system security and access logs</p>
      </div>

      <div className="card overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-apple-gray-200">
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Time</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Action</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Actor ID</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Target ID</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">IP / User Agent</th>
              <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Details</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} className="py-12 text-center text-apple-gray-400">
                  <div className="flex justify-center items-center h-full">
                    <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
                  </div>
                </td>
              </tr>
            ) : logs.length === 0 ? (
              <tr>
                <td colSpan={6} className="py-16 text-center">
                  <div className="w-16 h-16 bg-gray-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
                    <ShieldExclamationIcon className="w-8 h-8 text-apple-gray-400" />
                  </div>
                  <p className="text-apple-gray-500">No audit logs found.</p>
                </td>
              </tr>
            ) : (
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              logs.map((log: any, idx: number) => (
                <motion.tr
                  key={log.id}
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: idx * 0.02 }}
                  className="border-b border-apple-gray-100 hover:bg-apple-gray-50 transition-colors"
                >
                  <td className="py-3 px-4 text-sm text-apple-gray-500 whitespace-nowrap">
                    {formatDate(log.createdAt)}
                  </td>
                  <td className="py-3 px-4">
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-apple-gray-700">
                      {log.action}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-sm text-apple-gray-600 font-mono text-xs">
                    {log.actorId}
                  </td>
                  <td className="py-3 px-4 text-sm text-apple-gray-600 font-mono text-xs">
                    {log.targetId !== '00000000-0000-0000-0000-000000000000' ? log.targetId : '-'}
                  </td>
                  <td className="py-3 px-4 text-xs text-apple-gray-500 max-w-xs truncate" title={`IP: ${log.ip}\nUA: ${log.userAgent}`}>
                    <div>{log.ip}</div>
                    <div className="truncate opacity-75">{log.userAgent}</div>
                  </td>
                  <td className="py-3 px-4 text-xs text-apple-gray-500 max-w-sm truncate" title={log.detail}>
                    {log.detail || '-'}
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
    </div>
  );
}
