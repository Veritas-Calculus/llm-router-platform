import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import {
  CheckCircleIcon,
  ClipboardDocumentIcon,
  RocketLaunchIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';

interface QuickStartGuideProps {
  onDismiss?: () => void;
  className?: string;
}

export default function QuickStartGuide({ onDismiss, className = '' }: QuickStartGuideProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [copied, setCopied] = useState<string | null>(null);
  const baseUrl = `${window.location.origin}/v1`;

  const copyText = useCallback((text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopied(id);
    setTimeout(() => setCopied(null), 2000);
  }, []);

  const curlExample = `curl ${baseUrl}/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello!"}]}'`;

  const steps = [
    {
      num: 1,
      title: t('user_dashboard.step1_title'),
      desc: t('user_dashboard.step1_desc'),
      action: (
        <button
          onClick={() => navigate('/api-keys')}
          className="px-4 py-2 bg-apple-blue text-white text-sm font-semibold rounded-xl hover:bg-blue-600 transition-colors"
        >
          {t('user_dashboard.go_to_api_keys')}
        </button>
      ),
    },
    {
      num: 2,
      title: t('user_dashboard.step2_title'),
      desc: t('user_dashboard.step2_desc'),
      action: (
        <button
          onClick={() => copyText(baseUrl, 'url')}
          className="flex items-center gap-2 px-4 py-2 text-sm font-mono rounded-xl transition-colors hover:opacity-80"
          style={{
            backgroundColor: 'var(--theme-bg-input)',
            color: 'var(--theme-text)',
            border: '1px solid var(--theme-border)'
          }}
        >
          <span className="truncate max-w-[260px]">{baseUrl}</span>
          {copied === 'url' ? (
            <CheckCircleIcon className="w-4 h-4 " style={{ color: 'var(--color-green-500, #22c55e)' }} />
          ) : (
            <ClipboardDocumentIcon className="w-4 h-4 flex-shrink-0" style={{ color: 'var(--theme-text-muted)' }} />
          )}
        </button>
      ),
    },
    {
      num: 3,
      title: t('user_dashboard.step3_title'),
      desc: t('user_dashboard.step3_desc'),
      action: (
        <div className="relative">
          <pre className="text-xs bg-[#1C1C1E] border border-transparent dark:border-[var(--theme-border)] text-green-400 p-3 rounded-xl overflow-x-auto whitespace-pre-wrap leading-relaxed">
            {curlExample}
          </pre>
          <button
            onClick={() => copyText(curlExample, 'curl')}
            className="absolute top-2 right-2 p-1.5 bg-[#2C2C2E] rounded-lg hover:bg-[#3A3A3C] transition-colors"
          >
            {copied === 'curl' ? (
              <CheckCircleIcon className="w-3.5 h-3.5 text-green-400" />
            ) : (
              <ClipboardDocumentIcon className="w-3.5 h-3.5 text-[#AEAEB2]" />
            )}
          </button>
        </div>
      ),
    },
  ];

  return (
    <motion.div
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, height: 0, marginBottom: 0 }}
      className={`card relative overflow-hidden border border-blue-100 dark:border-[var(--theme-border)] quick-start-bg ${className}`}
    >
      {onDismiss && (
        <button
          onClick={onDismiss}
          className="absolute top-4 right-4 p-1.5 rounded-lg hover:bg-black/5 dark:hover:bg-white/10 transition-colors z-10"
          style={{ color: 'var(--theme-text-muted)' }}
        >
          <XMarkIcon className="w-5 h-5" />
        </button>
      )}

      <div className="flex items-center gap-3 mb-5">
        <div className="p-2.5 bg-gradient-to-br from-blue-500 to-purple-500 rounded-2xl">
          <RocketLaunchIcon className="w-6 h-6 text-white" />
        </div>
        <div>
          <h2 className="text-lg font-bold" style={{ color: 'var(--theme-text)' }}>{t('user_dashboard.quick_start')}</h2>
          <p className="text-sm" style={{ color: 'var(--theme-text-secondary)' }}>{t('user_dashboard.quick_start_desc')}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        {steps.map((step) => (
          <div key={step.num} className="space-y-3">
            <div className="flex items-center gap-2.5">
              <span className="flex items-center justify-center w-7 h-7 rounded-full bg-apple-blue text-white text-xs font-bold">
                {step.num}
              </span>
              <h3 className="text-sm font-semibold" style={{ color: 'var(--theme-text)' }}>{step.title}</h3>
            </div>
            <p className="text-xs leading-relaxed" style={{ color: 'var(--theme-text-secondary)' }}>{step.desc}</p>
            {step.action}
          </div>
        ))}
      </div>
    </motion.div>
  );
}
