import { useState, useEffect, useRef } from 'react';
import {
  ClockIcon,
  CurrencyDollarIcon,
  XMarkIcon,
  PlayIcon,
  EyeIcon,
} from '@heroicons/react/24/outline';
import clsx from 'clsx';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { Message, UsageStats } from './types';
import { getMessageText, getMessageImages } from './utils';

/* ── Stats display ──────────────────────────────────────────────── */

export function StatsBar({ stats, model }: { stats: UsageStats | null; model: string }) {
  if (!stats) return null;
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 bg-apple-gray-50 dark:bg-white/5 border-t border-apple-gray-100 dark:border-white/10 text-[11px] text-apple-gray-500 dark:text-gray-400 font-mono">
      <span className="font-semibold text-apple-gray-700 dark:text-gray-200">{model}</span>
      <span className="flex items-center gap-1">
        <ClockIcon className="w-3 h-3" />
        TTFB {stats.ttfbMs}ms · Total {stats.totalMs}ms
      </span>
      <span>{stats.tokensPerSec} tok/s</span>
      <span>
        Tokens: {stats.promptTokens} in / {stats.completionTokens} out = {stats.totalTokens}
      </span>
      <span className="flex items-center gap-0.5">
        <CurrencyDollarIcon className="w-3 h-3" />
        ~${((stats.promptTokens * 0.003 + stats.completionTokens * 0.006) / 1000).toFixed(4)}
      </span>
    </div>
  );
}

/* ── Image thumbnail in chat ───────────────────────────────────── */

export function ChatImageThumbnail({ url }: { url: string }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <>
      <button onClick={() => setExpanded(true)} className="block rounded-xl overflow-hidden border border-white/20 hover:ring-2 hover:ring-apple-blue/50 transition-all max-w-[180px]">
        <img src={url} alt="Uploaded" className="w-full h-auto max-h-32 object-cover" />
      </button>
      {expanded && (
        <div className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-8" onClick={() => setExpanded(false)}>
          <img src={url} alt="Full size" className="max-w-full max-h-full object-contain rounded-xl shadow-2xl" />
          <button onClick={() => setExpanded(false)} className="absolute top-4 right-4 text-white hover:text-gray-300">
            <XMarkIcon className="w-8 h-8" />
          </button>
        </div>
      )}
    </>
  );
}

/* ── Chat pane (reusable for single & compare modes) ────────────── */

interface ChatPaneProps {
  messages: Message[];
  isStreaming: boolean;
  stats: UsageStats | null;
  model: string;
  compact?: boolean;
}

export default function ChatPane({ messages, isStreaming, stats, model, compact }: ChatPaneProps) {
  const endRef = useRef<HTMLDivElement>(null);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  return (
    <div className={clsx("flex flex-col bg-white dark:bg-[#1C1C1E] rounded-2xl border border-apple-gray-200 dark:border-white/10 overflow-hidden", compact ? "h-full" : "flex-1")}>
      {compact && (
        <div className="h-10 bg-apple-gray-50 dark:bg-white/5 border-b border-apple-gray-100 dark:border-white/10 flex items-center px-3">
          <span className="text-xs font-semibold text-apple-gray-700 dark:text-gray-200 truncate">{model}</span>
          {isStreaming && <span className="ml-2 w-2 h-2 bg-green-400 rounded-full animate-pulse" />}
        </div>
      )}
      <div className={clsx("flex-1 overflow-y-auto p-4 space-y-4", compact && "text-sm")}>
        {messages.length === 0 && (
          <div className="h-full flex flex-col items-center justify-center text-apple-gray-400">
            <PlayIcon className="w-10 h-10 mb-3 opacity-50" />
            <p className="text-sm">Send a message to start.</p>
          </div>
        )}
        {messages.map((msg, i) => {
          const text = getMessageText(msg);
          const images = getMessageImages(msg);
          return (
            <div key={i} className={clsx("flex items-start gap-3 max-w-2xl", msg.role === 'user' ? "ml-auto flex-row-reverse" : "")}>
              <div className={clsx(
                "w-7 h-7 rounded-full flex items-center justify-center shrink-0 uppercase text-[10px] font-bold shadow-sm",
                msg.role === 'user' ? "bg-apple-blue text-white" : "bg-apple-gray-100 dark:bg-white/10 text-apple-gray-600 dark:text-gray-300"
              )}>
                {msg.role === 'user' ? 'U' : 'AI'}
              </div>
              <div className={clsx(
                "px-3 py-2.5 rounded-2xl text-sm leading-relaxed",
                msg.role === 'user'
                  ? "bg-apple-blue text-white rounded-tr-sm"
                  : "bg-apple-gray-50 dark:bg-white/5 text-apple-gray-800 dark:text-gray-100 rounded-tl-sm border border-apple-gray-100 dark:border-white/10 prose prose-sm dark:prose-invert prose-p:my-1 prose-pre:bg-apple-gray-800 prose-pre:text-apple-gray-100 prose-pre:py-2 prose-pre:px-3 prose-pre:rounded-xl prose-pre:my-2 prose-code:text-xs"
              )}>
                {images.length > 0 && (
                  <div className="flex flex-wrap gap-2 mb-2">
                    {images.map((url, j) => <ChatImageThumbnail key={j} url={url} />)}
                  </div>
                )}
                {msg.role === 'user' ? (
                  <div className="whitespace-pre-wrap">{text}</div>
                ) : (
                  <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
                )}
              </div>
            </div>
          );
        })}
        <div ref={endRef} />
      </div>
      <StatsBar stats={stats} model={model} />
    </div>
  );
}

// Re-export for convenience
export { EyeIcon };
