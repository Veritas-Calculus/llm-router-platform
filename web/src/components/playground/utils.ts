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

/** Check if a model is an STT (speech-to-text / whisper) model. */
export function isSTTModel(m: ModelRef): boolean {
  const lower = m.id.toLowerCase();
  return ['whisper', 'stt', 'speech-to-text', 'transcri'].some(p => lower.includes(p));
}

/** Check if a model is a TTS (text-to-speech) model. */
export function isTTSModel(m: ModelRef): boolean {
  const lower = m.id.toLowerCase();
  return ['tts', 'text-to-speech', 'speech', 'cosyvoice', 'bark', 'parler'].some(p => lower.includes(p));
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
    tokensPerSec: totalMs > 0 ? Math.round((completionTokens / (totalMs / 1000)) * 10) / 10 : 0,
  };
}
