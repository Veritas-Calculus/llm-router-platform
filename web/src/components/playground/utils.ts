import type { Message, ModelRef } from './types';

/** Extracts text from a message regardless of content format. */
export function getMessageText(msg: Message): string {
  if (typeof msg.content === 'string') return msg.content;
  return msg.content.filter(p => p.type === 'text').map(p => p.text || '').join('');
}

/** Extracts image URLs from a multimodal message. */
export function getMessageImages(msg: Message): string[] {
  if (typeof msg.content === 'string') return [];
  return msg.content.filter(p => p.type === 'image_url').map(p => p.image_url?.url || '').filter(Boolean);
}

/** Rough token estimator (GPT-style: ~4 chars per token). */
export function estimateTokens(text: string): number {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

export function estimateMessageTokens(msg: Message): number {
  if (typeof msg.content === 'string') return estimateTokens(msg.content);
  return msg.content.reduce((sum, p) => {
    if (p.type === 'text') return sum + estimateTokens(p.text || '');
    if (p.type === 'image_url') return sum + 85; // ~85 tokens per low-detail image
    return sum;
  }, 0);
}

/** Check if a model supports vision based on its metadata or name. */
export function isVisionModel(m: ModelRef): boolean {
  if (m.type === 'vlm') return true;
  if (m.capabilities?.vision) return true;
  if (m.input_modalities?.includes('image')) return true;
  const lower = m.id.toLowerCase();
  return ['-vl-', '-vl/', '/vl-', '-vision', 'vision-', '4o', 'gemini-1.5', 'gemini-2', 'claude-3', 'claude-4', 'pixtral', 'llava', 'cogvlm', 'internvl', 'minicpm-v', 'glm-4v', 'glm-4.6v', 'glm-4.7v'].some(p => lower.includes(p));
}

/**
 * Check if a model is an STT model. Prefers the backend-supplied
 * model_kind (audit M-02) and falls back to a name-based heuristic so
 * older /v1/models responses still get filtered correctly.
 */
export function isSTTModel(m: ModelRef): boolean {
  if (m.model_kind === 'STT') return true;
  if (m.model_kind && m.model_kind !== 'UNKNOWN') return false;
  const lower = m.id.toLowerCase();
  return ['whisper', 'stt', 'speech-to-text', 'transcri'].some(p => lower.includes(p));
}

/** Check if a model is a TTS model. Same priority order as isSTTModel. */
export function isTTSModel(m: ModelRef): boolean {
  if (m.model_kind === 'TTS') return true;
  if (m.model_kind && m.model_kind !== 'UNKNOWN') return false;
  const lower = m.id.toLowerCase();
  return ['tts', 'text-to-speech', 'cosyvoice', 'bark', 'parler', 'kokoro', 'elevenlabs'].some(p => lower.includes(p));
}

/**
 * Check if a model is a chat / completion model — i.e. an LLM the
 * Playground's main "Model" dropdown should let users pick. We treat
 * UNKNOWN as chat to preserve usability when a freshly-imported provider
 * hasn't been re-classified yet. Non-chat kinds (embedding, image,
 * stt/tts, rerank) are explicitly excluded so the dropdown stops being a
 * dumping ground for every /v1/models entry.
 */
export function isChatModel(m: ModelRef): boolean {
  if (m.model_kind === 'CHAT' || m.model_kind === 'UNKNOWN' || !m.model_kind) {
    // Trust the backend classifier when it says "chat" or "unknown".
    return !isSTTModel(m) && !isTTSModel(m) && !isEmbeddingModel(m) && !isImageModel(m);
  }
  return false;
}

/** Check if a model is an embedding model. */
export function isEmbeddingModel(m: ModelRef): boolean {
  if (m.model_kind === 'EMBEDDING') return true;
  if (m.model_kind && m.model_kind !== 'UNKNOWN') return false;
  const lower = m.id.toLowerCase();
  return ['embedding', 'embed-', 'bge-', 'nomic-embed'].some(p => lower.includes(p));
}

/** Check if a model is an image-generation model. */
export function isImageModel(m: ModelRef): boolean {
  if (m.model_kind === 'IMAGE') return true;
  if (m.model_kind && m.model_kind !== 'UNKNOWN') return false;
  const lower = m.id.toLowerCase();
  return ['dall-e', 'stable-diffusion', 'sd-', 'flux-', 'midjourney'].some(p => lower.includes(p));
}

/**
 * Conservative output-token cap for the Max Tokens slider (audit M-07).
 * Prefers the upstream max_output_tokens, then falls back to a static
 * 4096 — never to the context_window, which would let users set max_tokens
 * to e.g. 262144 and immediately blow past the model's real output cap.
 */
export function getMaxOutputTokensCap(m: ModelRef | undefined): number {
  const fromModel = m?.max_output_tokens;
  if (typeof fromModel === 'number' && fromModel > 0) return fromModel;
  return 4096;
}

/** Streaming completion runner — sends messages to the API and streams back responses. */
export async function runCompletion(
  apiKey: string,
  model: string,
  messages: Message[],
  temperature: number,
  maxTokens: number,
  signal: AbortSignal,
  onDelta: (content: string) => void,
): Promise<import('./types').UsageStats> {
  const t0 = performance.now();
  let ttfb = 0;
  let promptTokens = 0;
  let completionTokens = 0;

  const apiMessages = messages.map(m => ({
    role: m.role,
    content: m.content,
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
    // M-03: the rate is generation throughput AFTER first byte. Total -
    // TTFB excludes connection setup so "0 tok/s with 1 token out" no
    // longer happens. We persist a number here and let the renderer
    // decide formatting / "too fast to measure" display.
    tokensPerSec: computeTokensPerSec(completionTokens, ttfb, totalMs),
  };
}

// M-03: returns NaN when the generation window is too short to time
// (<100ms post-TTFB). The renderer treats NaN as "—" rather than 0.
export function computeTokensPerSec(completionTokens: number, ttfbMs: number, totalMs: number): number {
  if (completionTokens <= 0) return 0;
  const elapsed = totalMs - ttfbMs;
  if (elapsed < 100) return Number.NaN;
  return completionTokens / (elapsed / 1000);
}

// M-03: render a tok/s number with 2 significant figures when the raw
// value is below 10 (so 0.35 stays "0.35", not "0"), and 0 sig figs above
// 10 (so 124.6 becomes "125"). NaN renders as the em-dash "too fast to
// measure" sentinel.
export function formatTokensPerSec(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '—';
  if (value < 10) {
    return (Math.round(value * 10) / 10).toFixed(1);
  }
  return String(Math.round(value));
}
