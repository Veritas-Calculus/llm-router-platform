import { ArrowPathIcon } from '@heroicons/react/24/outline';
import clsx from 'clsx';
import { useTranslation } from '@/lib/i18n';
import type { StreamPhase } from './types';

function elapsedLabel(seconds: number) {
  return `${Math.max(0, seconds)}s`;
}

export function StreamingStatusBadge({
  phase,
  elapsedSec,
  className,
}: {
  phase: StreamPhase;
  elapsedSec: number;
  className?: string;
}) {
  const { t } = useTranslation();
  if (phase === 'idle') return null;

  const label = phase === 'waiting'
    ? t('playground.status_waiting_short')
    : t('playground.status_generating_short');

  return (
    <span className={clsx(
      'inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[10px] font-semibold',
      phase === 'waiting'
        ? 'border-amber-300/40 bg-amber-500/10 text-amber-600 dark:text-amber-300'
        : 'border-green-300/40 bg-green-500/10 text-green-600 dark:text-green-300',
      className
    )}>
      <ArrowPathIcon className="h-3 w-3 animate-spin" />
      <span>{label}</span>
      <span className="font-mono opacity-75">{elapsedLabel(elapsedSec)}</span>
    </span>
  );
}

export function StreamingPlaceholder({
  phase,
  elapsedSec,
  compact = false,
}: {
  phase: StreamPhase;
  elapsedSec: number;
  compact?: boolean;
}) {
  const { t } = useTranslation();
  const waiting = phase === 'waiting';
  const title = waiting
    ? t('playground.status_waiting_first_token')
    : t('playground.status_generating');
  const hint = waiting && elapsedSec >= 5
    ? t('playground.status_model_loading_hint')
    : t('playground.status_request_active');

  return (
    <div className={clsx('min-w-0', compact ? 'w-52' : 'w-72 max-w-full')}>
      <div className="flex items-center gap-2">
        <span className={clsx(
          'flex h-6 w-6 shrink-0 items-center justify-center rounded-full',
          waiting ? 'bg-amber-500/10 text-amber-500' : 'bg-green-500/10 text-green-500'
        )}>
          <ArrowPathIcon className="h-3.5 w-3.5 animate-spin" />
        </span>
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium">
            <span className="truncate">{title}</span>
            <span className="shrink-0 font-mono text-[11px] text-apple-gray-400 dark:text-gray-500">
              {elapsedLabel(elapsedSec)}
            </span>
          </div>
          <p className="mt-0.5 truncate text-[11px] text-apple-gray-500 dark:text-gray-400">
            {hint}
          </p>
        </div>
      </div>
      <div className="mt-3 flex items-center gap-1.5">
        <span className="h-1.5 w-14 rounded-full bg-apple-blue/60 animate-pulse" />
        <span className="h-1.5 w-8 rounded-full bg-apple-gray-300/70 dark:bg-white/20 animate-pulse" />
        <span className="h-1.5 w-11 rounded-full bg-apple-gray-200/80 dark:bg-white/10 animate-pulse" />
      </div>
    </div>
  );
}
