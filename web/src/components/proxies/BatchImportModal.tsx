import { motion } from 'framer-motion';
import { ArrowPathIcon, DocumentArrowUpIcon } from '@heroicons/react/24/outline';

interface BatchImportModalProps {
  isOpen: boolean;
  batchInput: string;
  importing: boolean;
  onInputChange: (value: string) => void;
  onImport: () => void;
  onClose: () => void;
}

export default function BatchImportModal({
  isOpen,
  batchInput,
  importing,
  onInputChange,
  onImport,
  onClose,
}: BatchImportModalProps) {
  if (!isOpen) return null;

  const lineCount = batchInput.split('\n').filter((l) => l.trim() && !l.trim().startsWith('#')).length;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-2xl mx-4"
      >
        <h2 className="text-xl font-semibold text-apple-gray-900 mb-2">
          Batch Import Proxies
        </h2>
        <p className="text-sm text-apple-gray-500 mb-4">
          Enter one proxy per line. Format: <code className="bg-apple-gray-100 px-1 rounded">URL [type] [region]</code>
        </p>
        <div className="space-y-4">
          <div>
            <textarea
              value={batchInput}
              onChange={(e) => onInputChange(e.target.value)}
              className="input font-mono text-sm"
              rows={12}
              placeholder={`# Examples (lines starting with # are ignored):
http://proxy1.example.com:8080
http://proxy2.example.com:8080 http US-West
socks5://proxy3.example.com:1080 socks5 EU
https://user:pass@proxy4.example.com:8080 https Asia

# You can also just paste a list of URLs:
http://1.2.3.4:8080
http://5.6.7.8:3128
socks5://9.10.11.12:1080`}
            />
          </div>
          <div className="bg-apple-gray-50 p-3 rounded-apple">
            <p className="text-xs text-apple-gray-600">
              <strong>Supported formats:</strong>
            </p>
            <ul className="text-xs text-apple-gray-500 mt-1 space-y-1">
              <li>• <code>http://host:port</code> - HTTP proxy</li>
              <li>• <code>https://host:port</code> - HTTPS proxy</li>
              <li>• <code>socks5://host:port</code> - SOCKS5 proxy</li>
              <li>• <code>http://user:pass@host:port</code> - With authentication</li>
            </ul>
          </div>
        </div>
        <div className="flex justify-between items-center mt-6">
          <p className="text-sm text-apple-gray-500">
            {lineCount} proxies to import
          </p>
          <div className="flex gap-3">
            <button onClick={onClose} className="btn btn-secondary">Cancel</button>
            <button
              onClick={onImport}
              className="btn btn-primary"
              disabled={importing}
            >
              {importing ? (
                <>
                  <ArrowPathIcon className="w-5 h-5 mr-2 animate-spin" />
                  Importing...
                </>
              ) : (
                <>
                  <DocumentArrowUpIcon className="w-5 h-5 mr-2" />
                  Import
                </>
              )}
            </button>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
