import { useState, useEffect, useRef, useCallback } from 'react';
import type { Message, ModelRef, UsageStats, ImageAttachment, ContentPart } from './types';
import { estimateTokens, estimateMessageTokens, isVisionModel, isTTSModel, runCompletion } from './utils';

export interface PlaygroundState {
  apiKey: string; setApiKey: (v: string) => void;
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
  const [apiKey, setApiKey] = useState('');
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

  const selectedModelRef = models.find(m => m.id === selectedModel);
  const modelSupportsVision = selectedModelRef ? isVisionModel(selectedModelRef) : false;

  const inputTokenEstimate = estimateTokens(input) + estimateTokens(systemPrompt) +
    messages.reduce((s, m) => s + estimateMessageTokens(m), 0) + attachments.length * 85;

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  useEffect(() => {
    if (apiKey) {
      fetchModels(apiKey);
    } else {
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
          const ttsCandidate = data.data.find((m: ModelRef) => isTTSModel(m));
          if (ttsCandidate && !ttsModel) setTtsModel(ttsCandidate.id);
        }
      } else {
        setErrorMsg('Failed to fetch models. Check your API Key.');
      }
    } catch {
      setErrorMsg('Network error while fetching models.');
    }
  };

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
      const formData = new FormData();
      const ext = blob.type.includes('webm') ? 'webm' : blob.type.includes('mp4') ? 'm4a' : 'wav';
      formData.append('file', blob, `recording.${ext}`);
      formData.append('model', sttModel || 'whisper-1');
      formData.append('response_format', 'json');
      const res = await fetch('/v1/audio/transcriptions', {
        method: 'POST',
        headers: { Authorization: `Bearer ${apiKey}` },
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
  }, [apiKey, sttModel]);

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
      if (!apiKey) { setErrorMsg('Configure an API Key first.'); return; }
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
  }, [apiKey, sttModel, hasBrowserSTT, transcribeAudio]);

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
    if (!apiKey || !text.trim()) return;
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
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${apiKey}` },
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
  }, [apiKey, playingTTSIdx, ttsModel]);

  /* ── Send / Stop / Clear ── */

  const handleSend = async () => {
    if ((!input.trim() && attachments.length === 0) || isStreaming) return;
    if (!apiKey) { setErrorMsg('Configure an API Key first.'); if (!showSettings) setShowSettings(true); return; }
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
    abortControllerRef.current = new AbortController();
    setMessages(prev => [...prev, { role: 'assistant', content: '' }]);

    const runA = runCompletion(apiKey, selectedModel, apiMsgs, temperature, maxTokens, abortControllerRef.current.signal, (content) => {
      setMessages(prev => { const u = [...prev]; u[u.length - 1] = { role: 'assistant', content }; return u; });
    }).then(s => { setStats(s); }).catch(err => {
      if (err.name !== 'AbortError') setErrorMsg(err.message);
    }).finally(() => { setIsStreaming(false); abortControllerRef.current = null; });

    if (compareMode && compareModel) {
      setIsStreamingB(true);
      abortControllerBRef.current = new AbortController();
      setMessagesB(prev => [...prev, { role: 'assistant', content: '' }]);

      const runB = runCompletion(apiKey, compareModel, apiMsgs, temperature, maxTokens, abortControllerBRef.current.signal, (content) => {
        setMessagesB(prev => { const u = [...prev]; u[u.length - 1] = { role: 'assistant', content }; return u; });
      }).then(s => { setStatsB(s); }).catch(err => {
        if (err.name !== 'AbortError') setErrorMsg(err.message);
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

  return {
    apiKey, setApiKey,
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
