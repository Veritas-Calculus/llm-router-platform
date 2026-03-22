import { useState, useEffect, useRef } from 'react';
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
} from '@heroicons/react/24/outline';
import clsx from 'clsx';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface Message {
  role: 'system' | 'user' | 'assistant';
  content: string;
}

interface ModelRef {
  id: string;
  object: string;
}

interface UsageStats {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  ttfbMs: number;     // Time to first byte
  totalMs: number;    // Total response time
  tokensPerSec: number;
}

// Rough token estimator (GPT-style: ~4 chars per token)
function estimateTokens(text: string): number {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

/** Runs one streaming chat completion and collects stats */
async function runCompletion(
  apiKey: string,
  model: string,
  messages: Array<{ role: string; content: string }>,
  temperature: number,
  maxTokens: number,
  signal: AbortSignal,
  onDelta: (content: string) => void,
): Promise<UsageStats> {
  const t0 = performance.now();
  let ttfb = 0;
  let promptTokens = 0;
  let completionTokens = 0;

  const response = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${apiKey}` },
    body: JSON.stringify({ model, messages, temperature, max_tokens: maxTokens, stream: true }),
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
      if (!line.startsWith('data: ') || line === 'data: [DONE]') {
        // Try to capture usage from [DONE] alternative format
        continue;
      }
      try {
        const data = JSON.parse(line.slice(6));
        const delta = data.choices?.[0]?.delta?.content || '';
        if (delta && !ttfb) ttfb = performance.now() - t0;
        full += delta;
        onDelta(full);
        // Capture usage if provided (OpenAI sends it in the last chunk)
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
  // If backend didn't provide usage, estimate client-side
  if (!promptTokens) {
    promptTokens = messages.reduce((s, m) => s + estimateTokens(m.content), 0);
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
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 bg-apple-gray-50 border-t border-apple-gray-100 text-[11px] text-apple-gray-500 font-mono">
      <span className="font-semibold text-apple-gray-700">{model}</span>
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
    <div className={clsx("flex flex-col bg-white rounded-2xl border border-apple-gray-200 overflow-hidden", compact ? "h-full" : "flex-1")}>
      {/* Model header for compare mode */}
      {compact && (
        <div className="h-10 bg-apple-gray-50 border-b border-apple-gray-100 flex items-center px-3">
          <span className="text-xs font-semibold text-apple-gray-700 truncate">{model}</span>
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
        {messages.map((msg, i) => (
          <div key={i} className={clsx("flex items-start gap-3 max-w-2xl", msg.role === 'user' ? "ml-auto flex-row-reverse" : "")}>
            <div className={clsx(
              "w-7 h-7 rounded-full flex items-center justify-center shrink-0 uppercase text-[10px] font-bold shadow-sm",
              msg.role === 'user' ? "bg-apple-blue text-white" : "bg-apple-gray-100 text-apple-gray-600"
            )}>
              {msg.role === 'user' ? 'U' : 'AI'}
            </div>
            <div className={clsx(
              "px-3 py-2.5 rounded-2xl text-sm leading-relaxed",
              msg.role === 'user'
                ? "bg-apple-blue text-white rounded-tr-sm"
                : "bg-apple-gray-50 text-apple-gray-800 rounded-tl-sm border border-apple-gray-100 prose prose-sm prose-p:my-1 prose-pre:bg-apple-gray-800 prose-pre:text-apple-gray-100 prose-pre:py-2 prose-pre:px-3 prose-pre:rounded-xl prose-pre:my-2 prose-code:text-xs"
            )}>
              {msg.role === 'user' ? (
                <div className="whitespace-pre-wrap">{msg.content}</div>
              ) : (
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
              )}
            </div>
          </div>
        ))}
        <div ref={endRef} />
      </div>
      <StatsBar stats={stats} model={model} />
    </div>
  );
}

/* ── Main Playground ─────────────────────────────────────────── */

export default function PlaygroundPage() {
  const [apiKey, setApiKey] = useState(() => localStorage.getItem('playground_api_key') || '');
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
  const [isStreaming, setIsStreaming] = useState(false);
  const [isStreamingB, setIsStreamingB] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const [showSettings, setShowSettings] = useState(true);
  const [stats, setStats] = useState<UsageStats | null>(null);
  const [statsB, setStatsB] = useState<UsageStats | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const abortControllerBRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Input token estimation (live)
  const inputTokenEstimate = estimateTokens(input) + estimateTokens(systemPrompt) +
    messages.reduce((s, m) => s + estimateTokens(m.content), 0);

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  useEffect(() => {
    if (apiKey) {
      localStorage.setItem('playground_api_key', apiKey);
      fetchModels(apiKey);
    } else {
      localStorage.removeItem('playground_api_key');
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


  const handleSend = async () => {
    if (!input.trim() || isStreaming) return;
    if (!apiKey) { setErrorMsg('Configure an API Key first.'); if (!showSettings) setShowSettings(true); return; }
    if (!selectedModel) { setErrorMsg('Select a model.'); return; }
    if (compareMode && !compareModel) { setErrorMsg('Select a comparison model.'); return; }

    const userMsg: Message = { role: 'user', content: input.trim() };
    const newMessages = [...messages, userMsg];
    setMessages(newMessages);
    if (compareMode) setMessagesB([...messagesB, userMsg]);
    setInput('');
    setErrorMsg('');
    setStats(null);
    setStatsB(null);

    // Build API messages
    const apiMsgs: Array<{ role: string; content: string }> = [];
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
        if (err.name !== 'AbortError') setErrorMsg(prev => prev ? prev + ' | Model B: ' + err.message : 'Model B: ' + err.message);
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
    setMessages([]); setMessagesB([]); setErrorMsg(''); setStats(null); setStatsB(null);
  };

  const toggleCompareMode = () => {
    setCompareMode(prev => {
      if (!prev && models.length > 1 && !compareModel) {
        // Auto-select a different model for comparison
        const other = models.find(m => m.id !== selectedModel);
        if (other) setCompareModel(other.id);
      }
      handleClear();
      return !prev;
    });
  };

  return (
    <div className="h-[calc(100vh-8rem)] flex flex-col lg:flex-row gap-4">
      {/* Settings Sidebar */}
      <div className={clsx(
        "bg-white rounded-3xl shadow-sm border border-apple-gray-200 overflow-y-auto transition-all duration-300",
        "lg:w-72 shrink-0",
        showSettings ? "h-auto p-4" : "hidden lg:block lg:h-auto lg:p-4"
      )}>
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-base font-semibold text-apple-gray-900">Settings</h2>
          <Cog6ToothIcon className="w-4 h-4 text-apple-gray-400" />
        </div>

        <div className="space-y-5">
          {/* API Key */}
          <div>
            <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">
              <KeyIcon className="w-3.5 h-3.5 inline-block mr-1" />API Key
            </label>
            <input
              type="password"
              placeholder="sk-..."
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm"
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
              className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm disabled:opacity-50"
            >
              {models.length === 0
                ? <option value="">No models</option>
                : models.map(m => <option key={m.id} value={m.id}>{m.id}</option>)}
            </select>
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
                className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm disabled:opacity-50"
              >
                {models.filter(m => m.id !== selectedModel).map(m => (
                  <option key={m.id} value={m.id}>{m.id}</option>
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
              className="w-full px-3 py-2 bg-apple-gray-50 border border-apple-gray-200 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm resize-none"
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
          <div className="flex items-center gap-2 px-3 py-2 bg-apple-gray-50 rounded-xl border border-apple-gray-100">
            <DocumentDuplicateIcon className="w-3.5 h-3.5 text-apple-gray-400" />
            <span className="text-[11px] text-apple-gray-500 font-mono">
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
            <div className="flex-1 flex flex-col bg-white rounded-3xl shadow-sm border border-apple-gray-200 overflow-hidden">
              <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-6">
                {messages.length === 0 && (
                  <div className="h-full flex flex-col items-center justify-center text-apple-gray-400">
                    <PlayIcon className="w-12 h-12 mb-4 opacity-50" />
                    <p>Send a message to start playing around.</p>
                    {models.length > 1 && (
                      <button onClick={toggleCompareMode}
                        className="mt-3 flex items-center gap-1.5 text-sm text-apple-blue hover:underline">
                        <ArrowsRightLeftIcon className="w-4 h-4" />
                        Try Compare Mode
                      </button>
                    )}
                  </div>
                )}
                {messages.map((msg, i) => (
                  <div key={i} className={clsx("flex items-start gap-4 max-w-3xl", msg.role === 'user' ? "ml-auto flex-row-reverse" : "")}>
                    <div className={clsx(
                      "w-8 h-8 rounded-full flex items-center justify-center shrink-0 uppercase text-xs font-bold shadow-sm",
                      msg.role === 'user' ? "bg-apple-blue text-white" : "bg-apple-gray-100 text-apple-gray-600"
                    )}>
                      {msg.role === 'user' ? 'U' : 'AI'}
                    </div>
                    <div className={clsx(
                      "px-4 py-3 rounded-2xl text-sm leading-relaxed",
                      msg.role === 'user'
                        ? "bg-apple-blue text-white rounded-tr-sm"
                        : "bg-apple-gray-50 text-apple-gray-800 rounded-tl-sm border border-apple-gray-100 prose prose-sm prose-p:my-1 prose-pre:bg-apple-gray-800 prose-pre:text-apple-gray-100 prose-pre:py-2 prose-pre:px-3 prose-pre:rounded-xl prose-pre:my-2 prose-code:text-xs"
                    )}>
                      {msg.role === 'user' ? (
                        <div className="whitespace-pre-wrap">{msg.content}</div>
                      ) : (
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                      )}
                    </div>
                  </div>
                ))}
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
          <div className="relative flex items-end">
            <textarea
              rows={input.split('\n').length > 1 ? Math.min(input.split('\n').length, 5) : 1}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); } }}
              placeholder="Type a message..."
              className="w-full py-3.5 pl-4 pr-24 bg-white border border-apple-gray-200 rounded-2xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm resize-none shadow-sm"
              style={{ minHeight: '52px' }}
            />
            <div className="absolute right-2 bottom-2 flex items-center gap-1.5">
              <span className="text-[10px] text-apple-gray-400 font-mono mr-1">~{estimateTokens(input)} tok</span>
              {isStreaming || isStreamingB ? (
                <button onClick={handleStop}
                  className="p-2 bg-red-500 text-white rounded-xl hover:bg-red-600 transition-colors shadow-sm">
                  <div className="w-4 h-4 rounded-sm bg-white" />
                </button>
              ) : (
                <button onClick={handleSend} disabled={!input.trim()}
                  className="p-2 bg-apple-blue text-white rounded-xl hover:bg-blue-600 transition-colors disabled:opacity-50 shadow-sm">
                  <PaperAirplaneIcon className="w-4 h-4" />
                </button>
              )}
            </div>
          </div>
          <div className="text-center mt-1.5">
            <span className="text-[10px] text-apple-gray-400">Enter to send · Shift+Enter for new line</span>
          </div>
        </div>
      </div>
    </div>
  );
}
