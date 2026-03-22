import { useState, useEffect, useRef, useCallback } from 'react';
import {
  Cog6ToothIcon,
  PaperAirplaneIcon,
  TrashIcon,
  PlayIcon,
  InformationCircleIcon,
  KeyIcon,
  ClockIcon,
  CurrencyDollarIcon,
  DocumentDuplicateIcon,
  ArrowsRightLeftIcon,
  PhotoIcon,
  XMarkIcon,
  EyeIcon,
} from '@heroicons/react/24/outline';
import clsx from 'clsx';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

/* ── Types ──────────────────────────────────────────────────────── */

/** A single content part (text or image_url) within a multimodal message. */
interface ContentPart {
  type: 'text' | 'image_url';
  text?: string;
  image_url?: { url: string; detail?: 'auto' | 'low' | 'high' };
}

interface Message {
  role: 'system' | 'user' | 'assistant';
  content: string | ContentPart[];
}

/** Extracts text from a message regardless of content format. */
function getMessageText(msg: Message): string {
  if (typeof msg.content === 'string') return msg.content;
  return msg.content.filter(p => p.type === 'text').map(p => p.text || '').join('');
}

/** Extracts image URLs from a multimodal message. */
function getMessageImages(msg: Message): string[] {
  if (typeof msg.content === 'string') return [];
  return msg.content.filter(p => p.type === 'image_url').map(p => p.image_url?.url || '').filter(Boolean);
}

interface ModelRef {
  id: string;
  object: string;
  type?: string;
  capabilities?: { vision?: boolean; chat?: boolean; completion?: boolean };
  input_modalities?: string[];
}

interface UsageStats {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  ttfbMs: number;
  totalMs: number;
  tokensPerSec: number;
}

/** Pending image attachment (base64 data URL). */
interface ImageAttachment {
  id: string;
  dataUrl: string;
  name: string;
}

// Rough token estimator (GPT-style: ~4 chars per token)
function estimateTokens(text: string): number {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

function estimateMessageTokens(msg: Message): number {
  if (typeof msg.content === 'string') return estimateTokens(msg.content);
  return msg.content.reduce((sum, p) => {
    if (p.type === 'text') return sum + estimateTokens(p.text || '');
    if (p.type === 'image_url') return sum + 85; // ~85 tokens per low-detail image
    return sum;
  }, 0);
}

/** Check if a model supports vision based on its metadata or name. */
function isVisionModel(m: ModelRef): boolean {
  if (m.type === 'vlm') return true;
  if (m.capabilities?.vision) return true;
  if (m.input_modalities?.includes('image')) return true;
  const lower = m.id.toLowerCase();
  return ['-vl-', '-vl/', '/vl-', '-vision', 'vision-', '4o', 'gemini-1.5', 'gemini-2', 'claude-3', 'claude-4', 'pixtral', 'llava', 'cogvlm', 'internvl', 'minicpm-v', 'glm-4v', 'glm-4.6v', 'glm-4.7v'].some(p => lower.includes(p));
}

/* ── Streaming completion runner ─────────────────────────────── */

async function runCompletion(
  apiKey: string,
  model: string,
  messages: Message[],
  temperature: number,
  maxTokens: number,
  signal: AbortSignal,
  onDelta: (content: string) => void,
): Promise<UsageStats> {
  const t0 = performance.now();
  let ttfb = 0;
  let promptTokens = 0;
  let completionTokens = 0;

  // Build API message payload — multimodal content is sent as content arrays
  const apiMessages = messages.map(m => ({
    role: m.role,
    content: m.content, // string OR ContentPart[] — OpenAI API accepts both
  }));

  const response = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${apiKey}` },
    body: JSON.stringify({ model, messages: apiMessages, temperature, max_tokens: maxTokens, stream: true }),
    signal,
  });

  if (!response.ok) {
    const err = await response.json();
    throw new Error(err.error?.message || response.statusText);
  }
  if (!response.body) throw new Error('No response body');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let full = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    const chunk = decoder.decode(value, { stream: true });
    for (const line of chunk.split('\n')) {
      if (!line.startsWith('data: ') || line === 'data: [DONE]') continue;
      try {
        const data = JSON.parse(line.slice(6));
        const delta = data.choices?.[0]?.delta?.content || '';
        if (delta && !ttfb) ttfb = performance.now() - t0;
        full += delta;
        onDelta(full);
        if (data.usage) {
          promptTokens = data.usage.prompt_tokens || 0;
          completionTokens = data.usage.completion_tokens || 0;
        }
      } catch {
        // partial chunk
      }
    }
  }

  const totalMs = performance.now() - t0;
  if (!promptTokens) {
    promptTokens = messages.reduce((s, m) => s + estimateMessageTokens(m), 0);
  }
  if (!completionTokens) {
    completionTokens = estimateTokens(full);
  }

  return {
    promptTokens,
    completionTokens,
    totalTokens: promptTokens + completionTokens,
    ttfbMs: Math.round(ttfb),
    totalMs: Math.round(totalMs),
    tokensPerSec: totalMs > 0 ? Math.round((completionTokens / (totalMs / 1000)) * 10) / 10 : 0,
  };
}

/* ── Stats display component ────────────────────────────────────── */
function StatsBar({ stats, model }: { stats: UsageStats | null; model: string }) {
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

/* ── Image thumbnail in chat ────────────────────────────────────── */
function ChatImageThumbnail({ url }: { url: string }) {
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

/* ── Chat pane (reusable for single & compare modes) ─────────── */
interface ChatPaneProps {
  messages: Message[];
  isStreaming: boolean;
  stats: UsageStats | null;
  model: string;
  compact?: boolean;
}
function ChatPane({ messages, isStreaming, stats, model, compact }: ChatPaneProps) {
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

/* ── Image attachment preview bar ─────────────────────────────── */
function AttachmentBar({ attachments, onRemove }: { attachments: ImageAttachment[]; onRemove: (id: string) => void }) {
  if (attachments.length === 0) return null;
  return (
    <div className="flex gap-2 px-3 py-2 flex-wrap">
      {attachments.map(att => (
        <div key={att.id} className="relative group">
          <img src={att.dataUrl} alt={att.name} className="h-16 w-16 object-cover rounded-xl border border-apple-gray-200 shadow-sm" />
          <button
            onClick={() => onRemove(att.id)}
            className="absolute -top-1.5 -right-1.5 w-5 h-5 bg-red-500 text-white rounded-full flex items-center justify-center text-[10px] opacity-0 group-hover:opacity-100 transition-opacity shadow"
          >
            <XMarkIcon className="w-3 h-3" />
          </button>
          <div className="absolute bottom-0 left-0 right-0 bg-black/50 text-white text-[8px] text-center rounded-b-xl truncate px-1">
            {att.name}
          </div>
        </div>
      ))}
    </div>
  );
}

/* ── Main Playground ─────────────────────────────────────────── */

export default function PlaygroundPage() {
  const [apiKey, setApiKey] = useState(() => sessionStorage.getItem('playground_api_key') || '');
  const [models, setModels] = useState<ModelRef[]>([]);
  const [selectedModel, setSelectedModel] = useState('');
  const [compareModel, setCompareModel] = useState('');
  const [compareMode, setCompareMode] = useState(false);
  const [systemPrompt, setSystemPrompt] = useState('You are a helpful assistant.');
  const [temperature, setTemperature] = useState(0.7);
  const [maxTokens, setMaxTokens] = useState(2000);
  const [messages, setMessages] = useState<Message[]>([]);
  const [messagesB, setMessagesB] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [attachments, setAttachments] = useState<ImageAttachment[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [isStreamingB, setIsStreamingB] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const [showSettings, setShowSettings] = useState(true);
  const [stats, setStats] = useState<UsageStats | null>(null);
  const [statsB, setStatsB] = useState<UsageStats | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);

  const abortControllerRef = useRef<AbortController | null>(null);
  const abortControllerBRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const selectedModelRef = models.find(m => m.id === selectedModel);
  const modelSupportsVision = selectedModelRef ? isVisionModel(selectedModelRef) : false;

  // Input token estimation (live)
  const inputTokenEstimate = estimateTokens(input) + estimateTokens(systemPrompt) +
    messages.reduce((s, m) => s + estimateMessageTokens(m), 0) + attachments.length * 85;

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  useEffect(() => {
    if (apiKey) {
      sessionStorage.setItem('playground_api_key', apiKey);
      fetchModels(apiKey);
    } else {
      sessionStorage.removeItem('playground_api_key');
      setModels([]);
      setSelectedModel('');
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [apiKey]);

  const fetchModels = async (key: string) => {
    try {
      const res = await fetch('/v1/models', { headers: { Authorization: `Bearer ${key}` } });
      if (res.ok) {
        const data = await res.json();
        if (data.data) {
          setModels(data.data);
          if (data.data.length > 0 && !selectedModel) setSelectedModel(data.data[0].id);
        }
      } else {
        setErrorMsg('Failed to fetch models. Check your API Key.');
      }
    } catch {
      setErrorMsg('Network error while fetching models.');
    }
  };

  /* ── Image handling ──────────────────────────────────────────── */

  const addImageFiles = useCallback((files: FileList | File[]) => {
    Array.from(files).forEach(file => {
      if (!file.type.startsWith('image/')) return;
      const reader = new FileReader();
      reader.onload = () => {
        setAttachments(prev => [...prev, {
          id: crypto.randomUUID(),
          dataUrl: reader.result as string,
          name: file.name,
        }]);
      };
      reader.readAsDataURL(file);
    });
  }, []);

  const removeAttachment = useCallback((id: string) => {
    setAttachments(prev => prev.filter(a => a.id !== id));
  }, []);

  // Paste handler (Ctrl+V)
  useEffect(() => {
    const handler = (e: ClipboardEvent) => {
      const items = e.clipboardData?.items;
      if (!items) return;
      const imageFiles: File[] = [];
      for (let i = 0; i < items.length; i++) {
        if (items[i].type.startsWith('image/')) {
          const file = items[i].getAsFile();
          if (file) imageFiles.push(file);
        }
      }
      if (imageFiles.length > 0) {
        e.preventDefault();
        addImageFiles(imageFiles);
      }
    };
    document.addEventListener('paste', handler);
    return () => document.removeEventListener('paste', handler);
  }, [addImageFiles]);

  // Drag-and-drop handlers
  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(true);
  }, []);
  const handleDragLeave = useCallback(() => setIsDragOver(false), []);
  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
    if (e.dataTransfer.files.length > 0) {
      addImageFiles(Array.from(e.dataTransfer.files));
    }
  }, [addImageFiles]);

  /* ── Send message ─────────────────────────────────────────────── */

  const handleSend = async () => {
    if ((!input.trim() && attachments.length === 0) || isStreaming) return;
    if (!apiKey) { setErrorMsg('Configure an API Key first.'); if (!showSettings) setShowSettings(true); return; }
    if (!selectedModel) { setErrorMsg('Select a model.'); return; }
    if (compareMode && !compareModel) { setErrorMsg('Select a comparison model.'); return; }

    // Build user message content
    let userContent: string | ContentPart[];
    if (attachments.length > 0) {
      // Multimodal message with images
      const parts: ContentPart[] = [];
      attachments.forEach(att => {
        parts.push({ type: 'image_url', image_url: { url: att.dataUrl, detail: 'auto' } });
      });
      if (input.trim()) {
        parts.push({ type: 'text', text: input.trim() });
      }
      userContent = parts;
    } else {
      userContent = input.trim();
    }

    const userMsg: Message = { role: 'user', content: userContent };
    const newMessages = [...messages, userMsg];
    setMessages(newMessages);
    if (compareMode) setMessagesB([...messagesB, userMsg]);
    setInput('');
    setAttachments([]);
    setErrorMsg('');
    setStats(null);
    setStatsB(null);

    // Build API messages
    const apiMsgs: Message[] = [];
    if (systemPrompt.trim()) apiMsgs.push({ role: 'system', content: systemPrompt.trim() });
    apiMsgs.push(...newMessages);

    // Model A
    setIsStreaming(true);
    abortControllerRef.current = new AbortController();
    setMessages(prev => [...prev, { role: 'assistant', content: '' }]);

    const runA = runCompletion(apiKey, selectedModel, apiMsgs, temperature, maxTokens, abortControllerRef.current.signal, (content) => {
      setMessages(prev => { const u = [...prev]; u[u.length - 1] = { role: 'assistant', content }; return u; });
    }).then(s => { setStats(s); }).catch(err => {
      if (err.name !== 'AbortError') setErrorMsg(err.message);
    }).finally(() => { setIsStreaming(false); abortControllerRef.current = null; });

    // Model B (compare mode)
    if (compareMode && compareModel) {
      setIsStreamingB(true);
      abortControllerBRef.current = new AbortController();
      setMessagesB(prev => [...prev, { role: 'assistant', content: '' }]);

      const runB = runCompletion(apiKey, compareModel, apiMsgs, temperature, maxTokens, abortControllerBRef.current.signal, (content) => {
        setMessagesB(prev => { const u = [...prev]; u[u.length - 1] = { role: 'assistant', content }; return u; });
      }).then(s => { setStatsB(s); }).catch(err => {
        if (err.name !== 'AbortError') setErrorMsg(prev => typeof prev === 'string' && prev ? prev + ' | Model B: ' + err.message : 'Model B: ' + err.message);
      }).finally(() => { setIsStreamingB(false); abortControllerBRef.current = null; });

      await Promise.allSettled([runA, runB]);
    } else {
      await runA;
    }
  };

  const handleStop = () => {
    abortControllerRef.current?.abort();
    abortControllerBRef.current?.abort();
    setIsStreaming(false);
    setIsStreamingB(false);
  };

  const handleClear = () => {
    setMessages([]); setMessagesB([]); setErrorMsg(''); setStats(null); setStatsB(null); setAttachments([]);
  };

  const toggleCompareMode = () => {
    setCompareMode(prev => {
      if (!prev && models.length > 1 && !compareModel) {
        const other = models.find(m => m.id !== selectedModel);
        if (other) setCompareModel(other.id);
      }
      handleClear();
      return !prev;
    });
  };

  return (
    <div
      className={clsx("h-[calc(100vh-8rem)] flex flex-col lg:flex-row gap-4 relative", isDragOver && "ring-2 ring-apple-blue ring-inset rounded-3xl")}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {/* Drag overlay */}
      {isDragOver && (
        <div className="absolute inset-0 z-40 bg-apple-blue/10 flex items-center justify-center rounded-3xl pointer-events-none">
          <div className="bg-white px-8 py-6 rounded-2xl shadow-lg flex items-center gap-3">
            <PhotoIcon className="w-8 h-8 text-apple-blue" />
            <span className="text-lg font-medium text-apple-gray-800">Drop image here</span>
          </div>
        </div>
      )}

      {/* Hidden file input */}
      <input
        type="file"
        ref={fileInputRef}
        accept="image/*"
        multiple
        className="hidden"
        onChange={(e) => { if (e.target.files) addImageFiles(e.target.files); e.target.value = ''; }}
      />

      {/* Settings Sidebar */}
      <div className={clsx(
        "bg-white dark:bg-[#1C1C1E] rounded-3xl shadow-sm border border-apple-gray-200 dark:border-white/10 overflow-y-auto transition-all duration-300",
        "lg:w-72 shrink-0",
        showSettings ? "h-auto p-4" : "hidden lg:block lg:h-auto lg:p-4"
      )}>
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-base font-semibold text-apple-gray-900 dark:text-white">Settings</h2>
          <Cog6ToothIcon className="w-4 h-4 text-apple-gray-400 dark:text-gray-500" />
        </div>

        <div className="space-y-5">
          {/* API Key */}
          <div>
            <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">
              <KeyIcon className="w-3.5 h-3.5 inline-block mr-1" /><span className="dark:text-gray-300">API Key</span>
            </label>
            <input
              type="password"
              placeholder="sk-..."
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100"
            />
          </div>

          {/* Model A */}
          <div>
            <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">
              {compareMode ? 'Model A' : 'Model'}
            </label>
            <select
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              disabled={models.length === 0}
              className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 disabled:opacity-50"
            >
              {models.length === 0
                ? <option value="">No models</option>
                : models.map(m => (
                  <option key={m.id} value={m.id}>
                    {isVisionModel(m) ? '[VLM] ' : ''}{m.id}
                  </option>
                ))}
            </select>
            {selectedModelRef && isVisionModel(selectedModelRef) && (
              <div className="mt-1.5 flex items-center gap-1.5 text-[11px] text-green-600">
                <EyeIcon className="w-3.5 h-3.5" />
                <span>Vision model — image upload enabled</span>
              </div>
            )}
          </div>

          {/* Compare Mode Toggle */}
          <div>
            <button
              onClick={toggleCompareMode}
              className={clsx(
                "w-full flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-sm font-medium transition-all",
                compareMode
                  ? "bg-apple-blue/10 text-apple-blue border border-apple-blue/30"
                  : "bg-apple-gray-50 text-apple-gray-600 border border-apple-gray-200 hover:bg-apple-gray-100"
              )}
            >
              <ArrowsRightLeftIcon className="w-4 h-4" />
              {compareMode ? 'Compare ON' : 'Compare Models'}
            </button>
          </div>

          {/* Model B (compare mode) */}
          {compareMode && (
            <div>
              <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">Model B</label>
              <select
                value={compareModel}
                onChange={(e) => setCompareModel(e.target.value)}
                disabled={models.length < 2}
                className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 disabled:opacity-50"
              >
                {models.filter(m => m.id !== selectedModel).map(m => (
                  <option key={m.id} value={m.id}>
                    {isVisionModel(m) ? '[VLM] ' : ''}{m.id}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* System Prompt */}
          <div>
            <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">System Prompt</label>
            <textarea
              rows={3}
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 resize-none"
            />
          </div>

          {/* Temperature */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-apple-gray-700">Temperature</label>
              <span className="text-xs text-apple-gray-500">{temperature}</span>
            </div>
            <input type="range" min="0" max="2" step="0.1" value={temperature}
              onChange={(e) => setTemperature(parseFloat(e.target.value))} className="w-full accent-apple-blue" />
          </div>

          {/* Max Tokens */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-apple-gray-700">Max Tokens</label>
              <span className="text-xs text-apple-gray-500">{maxTokens}</span>
            </div>
            <input type="range" min="100" max="16000" step="100" value={maxTokens}
              onChange={(e) => setMaxTokens(parseInt(e.target.value))} className="w-full accent-apple-blue" />
          </div>

          {/* Live input token estimate */}
          <div className="flex items-center gap-2 px-3 py-2 bg-apple-gray-50 dark:bg-white/5 rounded-xl border border-apple-gray-100 dark:border-white/10">
            <DocumentDuplicateIcon className="w-3.5 h-3.5 text-apple-gray-400 dark:text-gray-500" />
            <span className="text-[11px] text-apple-gray-500 dark:text-gray-400 font-mono">
              ~{inputTokenEstimate} tokens in context
            </span>
          </div>
        </div>
      </div>

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        <div className="h-12 flex items-center justify-between px-4 shrink-0 mb-2">
          <button onClick={() => setShowSettings(!showSettings)}
            className="lg:hidden p-2 -ml-2 text-apple-gray-600 hover:bg-apple-gray-50 rounded-xl">
            <Cog6ToothIcon className="w-5 h-5" />
          </button>
          <div className="flex-1 text-center font-medium text-apple-gray-900 text-sm">
            {compareMode
              ? `${selectedModel} vs ${compareModel}`
              : selectedModel ? `Talking to ${selectedModel}` : 'Playground'}
            {modelSupportsVision && !compareMode && (
              <span className="ml-2 inline-flex items-center gap-1 text-[10px] text-green-600 bg-green-50 px-1.5 py-0.5 rounded-full font-medium">
                <EyeIcon className="w-3 h-3" /> Vision
              </span>
            )}
            {(isStreaming || isStreamingB) && (
              <span className="ml-2 inline-block w-2 h-2 bg-green-400 rounded-full animate-pulse" />
            )}
          </div>
          <button onClick={handleClear} disabled={messages.length === 0 || isStreaming}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-apple-gray-600 hover:text-red-500 hover:bg-red-50 rounded-xl transition-colors disabled:opacity-50">
            <TrashIcon className="w-4 h-4" /> Clear
          </button>
        </div>

        {/* Chat panes */}
        <div className={clsx("flex-1 overflow-hidden", compareMode ? "flex gap-3" : "flex flex-col")}>
          {compareMode ? (
            <>
              <div className="flex-1 flex flex-col min-w-0">
                <ChatPane messages={messages} isStreaming={isStreaming} stats={stats} model={selectedModel} compact />
              </div>
              <div className="flex-1 flex flex-col min-w-0">
                <ChatPane messages={messagesB} isStreaming={isStreamingB} stats={statsB} model={compareModel} compact />
              </div>
            </>
          ) : (
            <div className="flex-1 flex flex-col bg-white dark:bg-[#1C1C1E] rounded-3xl shadow-sm border border-apple-gray-200 dark:border-white/10 overflow-hidden">
              <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-6">
                {messages.length === 0 && (
                  <div className="h-full flex flex-col items-center justify-center text-apple-gray-400">
                    <PlayIcon className="w-12 h-12 mb-4 opacity-50" />
                    <p>Send a message to start playing around.</p>
                    {modelSupportsVision && (
                      <p className="mt-2 text-sm text-green-500 flex items-center gap-1.5">
                        <PhotoIcon className="w-4 h-4" />
                        Paste, drop, or click the attach button to add images
                      </p>
                    )}
                    {models.length > 1 && (
                      <button onClick={toggleCompareMode}
                        className="mt-3 flex items-center gap-1.5 text-sm text-apple-blue hover:underline">
                        <ArrowsRightLeftIcon className="w-4 h-4" />
                        Try Compare Mode
                      </button>
                    )}
                  </div>
                )}
                {messages.map((msg, i) => {
                  const text = getMessageText(msg);
                  const images = getMessageImages(msg);
                  return (
                    <div key={i} className={clsx("flex items-start gap-4 max-w-3xl", msg.role === 'user' ? "ml-auto flex-row-reverse" : "")}>
                      <div className={clsx(
                        "w-8 h-8 rounded-full flex items-center justify-center shrink-0 uppercase text-xs font-bold shadow-sm",
                        msg.role === 'user' ? "bg-apple-blue text-white" : "bg-apple-gray-100 dark:bg-white/10 text-apple-gray-600 dark:text-gray-300"
                      )}>
                        {msg.role === 'user' ? 'U' : 'AI'}
                      </div>
                      <div className={clsx(
                        "px-4 py-3 rounded-2xl text-sm leading-relaxed",
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
                <div ref={messagesEndRef} />
              </div>
              <StatsBar stats={stats} model={selectedModel} />
            </div>
          )}
        </div>

        {/* Error Banner */}
        {errorMsg && (
          <div className="mt-2 px-4 py-2.5 bg-red-50 border border-red-100 rounded-2xl text-red-600 text-sm flex items-center gap-2">
            <InformationCircleIcon className="w-5 h-5 shrink-0" />
            <span className="flex-1">{errorMsg}</span>
            <button onClick={() => setErrorMsg('')} className="text-red-400 hover:text-red-600">&times;</button>
          </div>
        )}

        {/* Input Area */}
        <div className="pt-3 shrink-0">
          {/* Attachment preview bar */}
          <AttachmentBar attachments={attachments} onRemove={removeAttachment} />
          <div className="relative flex items-end">
            <textarea
              rows={input.split('\n').length > 1 ? Math.min(input.split('\n').length, 5) : 1}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); } }}
              placeholder={modelSupportsVision ? "Type a message or paste/drop an image..." : "Type a message..."}
              className="w-full py-3.5 pl-12 pr-24 bg-white dark:bg-[#1C1C1E] border border-apple-gray-200 dark:border-white/10 rounded-2xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 resize-none shadow-sm"
              style={{ minHeight: '52px' }}
            />
            {/* Attach image button */}
            <button
              onClick={() => fileInputRef.current?.click()}
              disabled={!modelSupportsVision}
              title={modelSupportsVision ? 'Attach image' : 'Select a vision-capable model to enable image uploads'}
              className={clsx(
                "absolute left-3 bottom-3 p-1.5 rounded-lg transition-colors",
                modelSupportsVision
                  ? "text-apple-gray-400 hover:text-apple-blue hover:bg-apple-blue/10"
                  : "text-apple-gray-200 cursor-not-allowed"
              )}
            >
              <PhotoIcon className="w-5 h-5" />
            </button>
            <div className="absolute right-2 bottom-2 flex items-center gap-1.5">
              <span className="text-[10px] text-apple-gray-400 font-mono mr-1">~{estimateTokens(input)} tok</span>
              {isStreaming || isStreamingB ? (
                <button onClick={handleStop}
                  className="p-2 bg-red-500 text-white rounded-xl hover:bg-red-600 transition-colors shadow-sm">
                  <div className="w-4 h-4 rounded-sm bg-white" />
                </button>
              ) : (
                <button onClick={handleSend} disabled={!input.trim() && attachments.length === 0}
                  className="p-2 bg-apple-blue text-white rounded-xl hover:bg-blue-600 transition-colors disabled:opacity-50 shadow-sm">
                  <PaperAirplaneIcon className="w-4 h-4" />
                </button>
              )}
            </div>
          </div>
          <div className="text-center mt-1.5">
            <span className="text-[10px] text-apple-gray-400">
              Enter to send · Shift+Enter for new line
              {modelSupportsVision && ' · Ctrl+V to paste image · Drag & drop images'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
