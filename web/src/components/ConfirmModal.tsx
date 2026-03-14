import { motion } from 'framer-motion';
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline';

interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText: string;
  confirmColor: 'red' | 'orange';
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

export default function ConfirmModal({
  isOpen,
  title,
  message,
  confirmText,
  confirmColor,
  onConfirm,
  onCancel,
  loading,
}: ConfirmModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-md mx-4"
      >
        <div className="flex items-start gap-4">
          <div
            className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${confirmColor === 'red' ? 'bg-red-100' : 'bg-orange-100'
              }`}
          >
            <ExclamationTriangleIcon
              className={`w-6 h-6 ${confirmColor === 'red' ? 'text-apple-red' : 'text-apple-orange'}`}
            />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-apple-gray-900">{title}</h3>
            <p className="mt-2 text-sm text-apple-gray-600">{message}</p>
          </div>
        </div>
        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onCancel} className="btn btn-secondary" disabled={loading}>
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className={`btn ${confirmColor === 'red' ? 'btn-danger' : 'bg-apple-orange text-white hover:opacity-90'}`}
            disabled={loading}
          >
            {loading ? 'Processing...' : confirmText}
          </button>
        </div>
      </motion.div>
    </div>
  );
}
