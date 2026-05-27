/** A single content part (text or image_url) within a multimodal message. */
export interface ContentPart {
  type: 'text' | 'image_url';
  text?: string;
  image_url?: { url: string; detail?: 'auto' | 'low' | 'high' };
}

export interface Message {
  role: 'system' | 'user' | 'assistant';
  content: string | ContentPart[];
}

export type StreamPhase = 'idle' | 'waiting' | 'streaming';

export interface ModelRef {
  id: string;
  object: string;
  type?: string;
  /**
   * Capability classification from the backend, mirroring the GraphQL
   * ModelKind enum. The /v1/models endpoint now stamps this on every
   * entry (audit M-02). UNKNOWN means the backend couldn't classify the
   * model and falls back to "treat it as chat" when filtering.
   */
  model_kind?: 'CHAT' | 'EMBEDDING' | 'IMAGE' | 'STT' | 'TTS' | 'RERANK' | 'UNKNOWN';
  capabilities?: { vision?: boolean; chat?: boolean; completion?: boolean };
  input_modalities?: string[];
  /**
   * Total prompt+completion window for the model. Sourced from upstream
   * (e.g. LM Studio's max_context_length) or the admin override.
   */
  max_context_length?: number;
  context_window?: number;
  /**
   * Per-request output cap. May be absent — Playground falls back to a
   * conservative default (4096) rather than reusing the context window.
   */
  max_output_tokens?: number;
}

export interface UsageStats {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  ttfbMs: number;
  totalMs: number;
  tokensPerSec: number;
}

/** Pending image attachment (base64 data URL). */
export interface ImageAttachment {
  id: string;
  dataUrl: string;
  name: string;
}
