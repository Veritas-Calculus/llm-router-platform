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

export interface ModelRef {
  id: string;
  object: string;
  type?: string;
  capabilities?: { vision?: boolean; chat?: boolean; completion?: boolean };
  input_modalities?: string[];
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
