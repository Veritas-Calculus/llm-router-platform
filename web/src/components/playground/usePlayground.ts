import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useMutation, useQuery } from '@apollo/client/react';
import { CREATE_PLAYGROUND_TOKEN, MY_API_KEYS, MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { useAuthHydrated } from '@/hooks/useAuthHydrated';
import type { Message, ModelRef, UsageStats, ImageAttachment, ContentPart, StreamPhase } from './types';
import { estimateTokens, estimateMessageTokens, getMessageText, isVisionModel, isTTSModel, runCompletion } from './utils';

export const MANUAL_API_KEY_VALUE = '__manual__';

async function readResponseError(res: Response): Promise<string> {
  const text = (await res.text().catch(() => '')).trim();
  if (!text) return res.statusText || 'request failed';

  try {
    const payload = JSON.parse(text) as {
      error?: string | { message?: string };
      message?: string;
    };
    if (typeof payload.error === 'string') return payload.error;
    if (payload.error?.message) return payload.error.message;
    if (payload.message) return payload.message;
  } catch {
    // Plain text errors are common for gateway/proxy 404s.
  }

  return text.length > 180 ? `${text.slice(0, 180)}...` : text;
}

interface PlaygroundOrganization {
  id: string;
  name: string;
}

interface PlaygroundProject {
  id: string;
  orgId: string;
  name: string;
}

interface PlaygroundApiKey {
  id: string;
  projectId: string;
  name: string;
  keyPrefix: string;
  isActive: boolean;
  scopes: string;
  expiresAt: string | null;
}

export interface PlaygroundState {
  apiKey: string; setApiKey: (v: string) => void;
  apiKeyMode: 'saved' | 'manual'; setApiKeyMode: (v: 'saved' | 'manual') => void;
  orgs: PlaygroundOrganization[];
  projects: PlaygroundProject[];
  apiKeys: PlaygroundApiKey[];
  activeApiKeys: PlaygroundApiKey[];
  selectedOrgId: string; setSelectedOrgId: (v: string) => void;
  selectedProjectId: string; setSelectedProjectId: (v: string) => void;
  selectedApiKeyId: string; setSelectedApiKeyId: (v: string) => void;
  selectedApiKey: PlaygroundApiKey | undefined;
  apiKeysLoading: boolean;
  tokenLoading: boolean;
  modelsLoading: boolean;
  models: ModelRef[]; selectedModel: string; setSelectedModel: (v: string) => void;
  compareModel: string; setCompareModel: (v: string) => void;
  compareMode: boolean;
  systemPrompt: string; setSystemPrompt: (v: string) => void;
  temperature: number; setTemperature: (v: number) => void;
  maxTokens: number; setMaxTokens: (v: number) => void;
  messages: Message[]; messagesB: Message[];
  input: string; setInput: (v: string) => void;
  attachments: ImageAttachment[];
  isStreaming: boolean; isStreamingB: boolean;
  streamPhase: StreamPhase; streamPhaseB: StreamPhase;
  streamElapsedSec: number; streamElapsedSecB: number;
  errorMsg: string; setErrorMsg: (v: string) => void;
  showSettings: boolean; setShowSettings: (v: boolean) => void;
  stats: UsageStats | null; statsB: UsageStats | null;
  isDragOver: boolean;
  sttModel: string; setSttModel: (v: string) => void;
  ttsModel: string; setTtsModel: (v: string) => void;
  isRecording: boolean; isTranscribing: boolean;
  playingTTSIdx: number | null; loadingTTSIdx: number | null;

  selectedModelRef: ModelRef | undefined;
  modelSupportsVision: boolean;
  inputTokenEstimate: number;

  // Refs
  messagesEndRef: React.RefObject<HTMLDivElement | null>;
  fileInputRef: React.RefObject<HTMLInputElement | null>;

  // Handlers
  handleSend: () => Promise<void>;
  handleStop: () => void;
  handleClear: () => void;
  toggleCompareMode: () => void;
  addImageFiles: (files: FileList | File[]) => void;
  removeAttachment: (id: string) => void;
  handleDragOver: (e: React.DragEvent) => void;
  handleDragLeave: () => void;
  handleDrop: (e: React.DragEvent) => void;
  startRecording: () => Promise<void>;
  stopRecording: () => void;
  playTTS: (text: string, msgIdx: number) => Promise<void>;
}

export function usePlayground(): PlaygroundState {
  const [apiKeyMode, setApiKeyMode] = useState<'saved' | 'manual'>('saved');
  const [manualApiKey, setManualApiKey] = useState('');
  const [playgroundToken, setPlaygroundToken] = useState('');
  const [playgroundTokenExpiresAt, setPlaygroundTokenExpiresAt] = useState('');
  // Org selection lives in the persisted auth store so OrgSwitcher
  // selections take effect on every page without page-local drift.
  const selectedOrgId = useAuthStore((s) => s.selectedOrgId) ?? '';
  const setSelectedOrgId = useAuthStore((s) => s.setSelectedOrgId);
  const hydrated = useAuthHydrated();
  const [selectedProjectId, setSelectedProjectId] = useState('');
  const [selectedApiKeyId, setSelectedApiKeyId] = useState('');
  const [tokenLoading, setTokenLoading] = useState(false);
  const [modelsLoading, setModelsLoading] = useState(false);
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
  const [streamStartedAt, setStreamStartedAt] = useState<number | null>(null);
  const [streamStartedAtB, setStreamStartedAtB] = useState<number | null>(null);
  const [streamClock, setStreamClock] = useState(Date.now());
  const [errorMsg, setErrorMsg] = useState('');
  const [showSettings, setShowSettings] = useState(true);
  const [stats, setStats] = useState<UsageStats | null>(null);
  const [statsB, setStatsB] = useState<UsageStats | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);
  const [sttModel, setSttModel] = useState('');
  const [ttsModel, setTtsModel] = useState('');

  // STT state
  const [isRecording, setIsRecording] = useState(false);
  const [isTranscribing, setIsTranscribing] = useState(false);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const audioChunksRef = useRef<Blob[]>([]);

  // TTS state
  const [playingTTSIdx, setPlayingTTSIdx] = useState<number | null>(null);
  const [loadingTTSIdx, setLoadingTTSIdx] = useState<number | null>(null);
  const ttsAudioRef = useRef<HTMLAudioElement | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const abortControllerBRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { data: orgData, loading: orgsLoading } = useQuery<{ myOrganizations: PlaygroundOrganization[] }>(MY_ORGANIZATIONS);
  const orgs = useMemo(() => orgData?.myOrganizations ?? [], [orgData]);

  const { data: projectData, loading: projectsLoading } = useQuery<{ myProjects: PlaygroundProject[] }>(MY_PROJECTS, {
    variables: { orgId: selectedOrgId },
    skip: !selectedOrgId,
  });
  const projects = useMemo(() => projectData?.myProjects ?? [], [projectData]);

  const { data: apiKeyData, loading: keysLoading } = useQuery<{ myApiKeys: PlaygroundApiKey[] }>(MY_API_KEYS, {
    variables: { projectId: selectedProjectId },
    skip: !selectedProjectId,
  });
  const apiKeys = useMemo(() => apiKeyData?.myApiKeys ?? [], [apiKeyData]);
  const activeApiKeys = useMemo(() => {
    const now = Date.now();
    return apiKeys.filter(key => {
      if (!key.isActive) return false;
      if (!key.expiresAt) return true;
      const expiresAt = Date.parse(key.expiresAt);
      return Number.isNaN(expiresAt) || expiresAt > now;
    });
  }, [apiKeys]);
  const selectedApiKey = activeApiKeys.find(key => key.id === selectedApiKeyId);
  const apiKeysLoading = orgsLoading || projectsLoading || keysLoading;
  const apiKey = apiKeyMode === 'manual' ? manualApiKey : playgroundToken;
  const [createPlaygroundToken] = useMutation<{
    createPlaygroundToken: {
      token: string;
      expiresAt: string;
      apiKeyId: string;
      projectId: string;
    };
  }>(CREATE_PLAYGROUND_TOKEN);

  const selectedModelRef = models.find(m => m.id === selectedModel);
  const modelSupportsVision = selectedModelRef ? isVisionModel(selectedModelRef) : false;
  const getStreamPhase = (list: Message[], active: boolean): StreamPhase => {
    if (!active) return 'idle';
    const last = list[list.length - 1];
    if (last?.role === 'assistant' && getMessageText(last).trim()) return 'streaming';
    return 'waiting';
  };
  const streamPhase = getStreamPhase(messages, isStreaming);
  const streamPhaseB = getStreamPhase(messagesB, isStreamingB);
  const streamElapsedSec = streamStartedAt ? Math.max(0, Math.floor((streamClock - streamStartedAt) / 1000)) : 0;
  const streamElapsedSecB = streamStartedAtB ? Math.max(0, Math.floor((streamClock - streamStartedAtB) / 1000)) : 0;

  const inputTokenEstimate = estimateTokens(input) + estimateTokens(systemPrompt) +
    messages.reduce((s, m) => s + estimateMessageTokens(m), 0) + attachments.length * 85;

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  useEffect(() => {
    if (!isStreaming && !isStreamingB) return;
    setStreamClock(Date.now());
    const timer = window.setInterval(() => setStreamClock(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [isStreaming, isStreamingB]);

  useEffect(() => {
    if (!hydrated) return;
    if (orgs.length > 0 && !selectedOrgId) {
      setSelectedOrgId(orgs[0].id);
    }
  }, [hydrated, orgs, selectedOrgId, setSelectedOrgId]);

  useEffect(() => {
    if (projects.length > 0) {
      if (!selectedProjectId || !projects.some(project => project.id === selectedProjectId)) {
        setSelectedProjectId(projects[0].id);
      }
    } else if (selectedProjectId) {
      setSelectedProjectId('');
    }
  }, [projects, selectedProjectId]);

  useEffect(() => {
    if (apiKeyMode === 'manual') return;
    if (activeApiKeys.length > 0) {
      if (!selectedApiKeyId || !activeApiKeys.some(key => key.id === selectedApiKeyId)) {
        setSelectedApiKeyId(activeApiKeys[0].id);
      }
    } else if (apiKeyData) {
      setSelectedApiKeyId('');
      setPlaygroundToken('');
      setPlaygroundTokenExpiresAt('');
    }
  }, [activeApiKeys, apiKeyData, apiKeyMode, selectedApiKeyId]);

  const refreshPlaygroundToken = useCallback(async (keyId = selectedApiKeyId) => {
    if (!keyId) return '';
    setTokenLoading(true);
    try {
      const { data } = await createPlaygroundToken({ variables: { apiKeyId: keyId } });
      const tokenInfo = data?.createPlaygroundToken;
      if (!tokenInfo?.token) {
        throw new Error('Unable to prepare the selected API key for Playground.');
      }
      setPlaygroundToken(tokenInfo.token);
      setPlaygroundTokenExpiresAt(tokenInfo.expiresAt);
      return tokenInfo.token;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to prepare the selected API key.';
      setErrorMsg(msg);
      setPlaygroundToken('');
      setPlaygroundTokenExpiresAt('');
      return '';
    } finally {
      setTokenLoading(false);
    }
  }, [createPlaygroundToken, selectedApiKeyId]);

  useEffect(() => {
    if (apiKeyMode !== 'saved') {
      setPlaygroundToken('');
      setPlaygroundTokenExpiresAt('');
      return;
    }
    if (!selectedApiKeyId) {
      setPlaygroundToken('');
      setPlaygroundTokenExpiresAt('');
      return;
    }
    void refreshPlaygroundToken(selectedApiKeyId);
  }, [apiKeyMode, refreshPlaygroundToken, selectedApiKeyId]);

  const getRequestToken = useCallback(async () => {
    if (apiKeyMode === 'manual') return manualApiKey.trim();
    if (!selectedApiKeyId) return '';
    const expiresAt = Date.parse(playgroundTokenExpiresAt);
    const shouldRefresh = !playgroundToken || !expiresAt || Number.isNaN(expiresAt) || expiresAt - Date.now() < 60_000;
    return shouldRefresh ? refreshPlaygroundToken(selectedApiKeyId) : playgroundToken;
  }, [apiKeyMode, manualApiKey, playgroundToken, playgroundTokenExpiresAt, refreshPlaygroundToken, selectedApiKeyId]);

  const fetchModels = useCallback(async (key: string) => {
    setModelsLoading(true);
    try {
      const res = await fetch('/v1/models', { headers: { Authorization: `Bearer ${key}` } });
      if (res.ok) {
        const data = await res.json();
        if (data.data) {
          const nextModels = data.data as ModelRef[];
          setModels(nextModels);
          setSelectedModel(prev => {
            if (prev && nextModels.some(m => m.id === prev)) return prev;
            return nextModels[0]?.id ?? '';
          });
          setCompareModel(prev => {
            if (prev && nextModels.some(m => m.id === prev)) return prev;
            return '';
          });
          setTtsModel(prev => {
            if (prev && nextModels.some(m => m.id === prev)) return prev;
            const ttsCandidate = nextModels.find((m: ModelRef) => isTTSModel(m));
            return ttsCandidate?.id ?? '';
          });
        }
      } else {
        const reason = await readResponseError(res);
        setErrorMsg(`Failed to fetch models (${res.status}): ${reason}`);
        setModels([]);
        setSelectedModel('');
      }
    } catch (err: unknown) {
      const reason = err instanceof Error ? err.message : 'request failed';
      setErrorMsg(`Network error while fetching models: ${reason}`);
      setModels([]);
      setSelectedModel('');
    } finally {
      setModelsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (apiKey) {
      void fetchModels(apiKey);
    } else {
      setModels([]);
      setSelectedModel('');
    }
  }, [apiKey, fetchModels]);

  /* ── Image handling ── */

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

  /* ── STT ── */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const speechRecognitionRef = useRef<any>(null);
  const hasBrowserSTT = typeof window !== 'undefined' && ('SpeechRecognition' in window || 'webkitSpeechRecognition' in window);

  const transcribeAudio = useCallback(async (blob: Blob) => {
    setIsTranscribing(true);
    try {
      const requestToken = await getRequestToken();
      if (!requestToken) throw new Error('Configure an API Key first.');
      const formData = new FormData();
      const ext = blob.type.includes('webm') ? 'webm' : blob.type.includes('mp4') ? 'm4a' : 'wav';
      formData.append('file', blob, `recording.${ext}`);
      formData.append('model', sttModel || 'whisper-1');
      formData.append('response_format', 'json');
      const res = await fetch('/v1/audio/transcriptions', {
        method: 'POST',
        headers: { Authorization: `Bearer ${requestToken}` },
        body: formData,
      });
      if (!res.ok) {
        const errBody = await res.text();
        throw new Error(`Transcription failed (${res.status}): ${errBody}`);
      }
      const data = await res.json();
      if (data.text) {
        setInput(prev => prev ? prev + ' ' + data.text : data.text);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Transcription failed';
      setErrorMsg(msg);
    } finally {
      setIsTranscribing(false);
    }
  }, [getRequestToken, sttModel]);

  const startRecording = useCallback(async () => {
    if (hasBrowserSTT && !sttModel) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const W = window as any;
      const SRClass = W.SpeechRecognition || W.webkitSpeechRecognition;
      const recognition = new SRClass();
      recognition.continuous = true;
      recognition.interimResults = false;
      recognition.lang = navigator.language || 'en-US';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      recognition.onresult = (event: any) => {
        let transcript = '';
        for (let i = 0; i < event.results.length; i++) {
          transcript += event.results[i][0].transcript;
        }
        transcript = transcript.trim();
        if (transcript) setInput(prev => prev ? prev + ' ' + transcript : transcript);
      };
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      recognition.onerror = (event: any) => {
        if (event.error !== 'aborted') setErrorMsg(`Speech recognition error: ${event.error}`);
        setIsRecording(false);
      };
      recognition.onend = () => setIsRecording(false);
      speechRecognitionRef.current = recognition;
      recognition.start();
      setIsRecording(true);
    } else {
      if (apiKeyMode === 'manual' && !manualApiKey.trim()) { setErrorMsg('Configure an API Key first.'); return; }
      if (apiKeyMode === 'saved' && !selectedApiKeyId) { setErrorMsg('Select an API Key first.'); return; }
      if (!sttModel) { setErrorMsg('Select an STT model in Settings, or use a browser that supports Web Speech API.'); return; }
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        const mediaRecorder = new MediaRecorder(stream, { mimeType: MediaRecorder.isTypeSupported('audio/webm') ? 'audio/webm' : 'audio/mp4' });
        audioChunksRef.current = [];
        mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data); };
        mediaRecorder.onstop = async () => {
          stream.getTracks().forEach(t => t.stop());
          const blob = new Blob(audioChunksRef.current, { type: mediaRecorder.mimeType });
          if (blob.size < 100) { setErrorMsg('Recording is too short.'); return; }
          await transcribeAudio(blob);
        };
        mediaRecorder.start();
        mediaRecorderRef.current = mediaRecorder;
        setIsRecording(true);
      } catch (err) {
        setErrorMsg('Microphone access denied. Please allow microphone access in browser settings.');
        console.error('Microphone access error:', err);
      }
    }
  }, [apiKeyMode, manualApiKey, selectedApiKeyId, sttModel, hasBrowserSTT, transcribeAudio]);

  const stopRecording = useCallback(() => {
    if (speechRecognitionRef.current) {
      speechRecognitionRef.current.stop();
      speechRecognitionRef.current = null;
    }
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
      mediaRecorderRef.current.stop();
    }
    setIsRecording(false);
  }, []);

  /* ── TTS ── */

  const playTTS = useCallback(async (text: string, msgIdx: number) => {
    if (!text.trim()) return;
    const requestToken = await getRequestToken();
    if (!requestToken) { setErrorMsg('Configure an API Key first.'); return; }
    if (ttsAudioRef.current) {
      ttsAudioRef.current.pause();
      ttsAudioRef.current = null;
    }
    if (playingTTSIdx === msgIdx) {
      setPlayingTTSIdx(null);
      return;
    }
    setLoadingTTSIdx(msgIdx);
    try {
      const res = await fetch('/v1/audio/speech', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${requestToken}` },
        body: JSON.stringify({ model: ttsModel || 'tts-1', input: text.slice(0, 4096), voice: 'alloy' }),
      });
      if (!res.ok) {
        const errBody = await res.text();
        throw new Error(`TTS failed (${res.status}): ${errBody}`);
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const audio = new Audio(url);
      audio.onended = () => { setPlayingTTSIdx(null); URL.revokeObjectURL(url); ttsAudioRef.current = null; };
      audio.onerror = () => { setPlayingTTSIdx(null); URL.revokeObjectURL(url); ttsAudioRef.current = null; };
      ttsAudioRef.current = audio;
      setPlayingTTSIdx(msgIdx);
      await audio.play();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'TTS failed';
      setErrorMsg(msg);
      setPlayingTTSIdx(null);
    } finally {
      setLoadingTTSIdx(null);
    }
  }, [getRequestToken, playingTTSIdx, ttsModel]);

  /* ── Send / Stop / Clear ── */

  const removeEmptyAssistantTail = useCallback((setList: typeof setMessages) => {
    setList(prev => {
      const last = prev[prev.length - 1];
      if (last?.role === 'assistant' && !getMessageText(last).trim()) {
        return prev.slice(0, -1);
      }
      return prev;
    });
  }, []);

  const handleSend = async () => {
    if ((!input.trim() && attachments.length === 0) || isStreaming) return;
    const requestToken = await getRequestToken();
    if (!requestToken) { setErrorMsg('Configure an API Key first.'); if (!showSettings) setShowSettings(true); return; }
    if (!selectedModel) { setErrorMsg('Select a model.'); return; }
    if (compareMode && !compareModel) { setErrorMsg('Select a comparison model.'); return; }

    let userContent: string | ContentPart[];
    if (attachments.length > 0) {
      const parts: ContentPart[] = [];
      attachments.forEach(att => {
        parts.push({ type: 'image_url', image_url: { url: att.dataUrl, detail: 'auto' } });
      });
      if (input.trim()) parts.push({ type: 'text', text: input.trim() });
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

    const apiMsgs: Message[] = [];
    if (systemPrompt.trim()) apiMsgs.push({ role: 'system', content: systemPrompt.trim() });
    apiMsgs.push(...newMessages);

    setIsStreaming(true);
    setStreamStartedAt(Date.now());
    abortControllerRef.current = new AbortController();
    setMessages(prev => [...prev, { role: 'assistant', content: '' }]);

    const runA = runCompletion(requestToken, selectedModel, apiMsgs, temperature, maxTokens, abortControllerRef.current.signal, (content) => {
      setMessages(prev => { const u = [...prev]; u[u.length - 1] = { role: 'assistant', content }; return u; });
    }).then(s => { setStats(s); }).catch(err => {
      if (err.name !== 'AbortError') {
        setErrorMsg(err.message);
        removeEmptyAssistantTail(setMessages);
      }
    }).finally(() => { setIsStreaming(false); setStreamStartedAt(null); abortControllerRef.current = null; });

    if (compareMode && compareModel) {
      setIsStreamingB(true);
      setStreamStartedAtB(Date.now());
      abortControllerBRef.current = new AbortController();
      setMessagesB(prev => [...prev, { role: 'assistant', content: '' }]);

      const runB = runCompletion(requestToken, compareModel, apiMsgs, temperature, maxTokens, abortControllerBRef.current.signal, (content) => {
        setMessagesB(prev => { const u = [...prev]; u[u.length - 1] = { role: 'assistant', content }; return u; });
      }).then(s => { setStatsB(s); }).catch(err => {
        if (err.name !== 'AbortError') {
          setErrorMsg(err.message);
          removeEmptyAssistantTail(setMessagesB);
        }
      }).finally(() => { setIsStreamingB(false); setStreamStartedAtB(null); abortControllerBRef.current = null; });

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
    setStreamStartedAt(null);
    setStreamStartedAtB(null);
    removeEmptyAssistantTail(setMessages);
    removeEmptyAssistantTail(setMessagesB);
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

  return {
    apiKey, setApiKey: setManualApiKey,
    apiKeyMode, setApiKeyMode,
    orgs, projects,
    apiKeys, activeApiKeys,
    selectedOrgId, setSelectedOrgId,
    selectedProjectId, setSelectedProjectId,
    selectedApiKeyId, setSelectedApiKeyId,
    selectedApiKey,
    apiKeysLoading,
    tokenLoading,
    modelsLoading,
    models, selectedModel, setSelectedModel,
    compareModel, setCompareModel,
    compareMode,
    systemPrompt, setSystemPrompt,
    temperature, setTemperature,
    maxTokens, setMaxTokens,
    messages, messagesB,
    input, setInput,
    attachments,
    isStreaming, isStreamingB,
    streamPhase, streamPhaseB,
    streamElapsedSec, streamElapsedSecB,
    errorMsg, setErrorMsg,
    showSettings, setShowSettings,
    stats, statsB,
    isDragOver,
    sttModel, setSttModel,
    ttsModel, setTtsModel,
    isRecording, isTranscribing,
    playingTTSIdx, loadingTTSIdx,

    selectedModelRef,
    modelSupportsVision,
    inputTokenEstimate,

    messagesEndRef,
    fileInputRef,

    handleSend,
    handleStop,
    handleClear,
    toggleCompareMode,
    addImageFiles,
    removeAttachment,
    handleDragOver,
    handleDragLeave,
    handleDrop,
    startRecording,
    stopRecording,
    playTTS,
  };
}
