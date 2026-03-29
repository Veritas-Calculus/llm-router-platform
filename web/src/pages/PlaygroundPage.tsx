import {
  Cog6ToothIcon,
  PaperAirplaneIcon,
  TrashIcon,
  PlayIcon,
  StopIcon,
  InformationCircleIcon,
  KeyIcon,
  DocumentDuplicateIcon,
  ArrowsRightLeftIcon,
  PhotoIcon,
  EyeIcon,
  MicrophoneIcon,
  SpeakerWaveIcon,
} from '@heroicons/react/24/outline';
import clsx from 'clsx';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useTranslation } from '@/lib/i18n';

import {
  usePlayground,
  ChatPane,
  StatsBar,
  ChatImageThumbnail,
  AttachmentBar,
  isVisionModel,
  isSTTModel,
  isTTSModel,
  estimateTokens,
  getMessageText,
  getMessageImages,
} from '@/components/playground';

export default function PlaygroundPage() {
  const { t } = useTranslation();
  const pg = usePlayground();

  return (
    <div
      className={clsx("h-[calc(100vh-8rem)] flex flex-col lg:flex-row gap-4 relative", pg.isDragOver && "ring-2 ring-apple-blue ring-inset rounded-3xl")}
      onDragOver={pg.handleDragOver}
      onDragLeave={pg.handleDragLeave}
      onDrop={pg.handleDrop}
    >
      {/* Drag overlay */}
      {pg.isDragOver && (
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
        ref={pg.fileInputRef}
        accept="image/*"
        multiple
        className="hidden"
        onChange={(e) => { if (e.target.files) pg.addImageFiles(e.target.files); e.target.value = ''; }}
      />

      {/* ── Settings Sidebar ── */}
      <div className={clsx(
        "bg-white dark:bg-[#1C1C1E] rounded-3xl shadow-sm border border-apple-gray-200 dark:border-white/10 overflow-y-auto transition-all duration-300",
        "lg:w-72 shrink-0",
        pg.showSettings ? "h-auto p-4" : "hidden lg:block lg:h-auto lg:p-4"
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
              type="password" placeholder="sk-..." value={pg.apiKey}
              onChange={(e) => pg.setApiKey(e.target.value)}
              className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100"
            />
          </div>

          {/* Model A */}
          <div>
            <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">
              {pg.compareMode ? 'Model A' : 'Model'}
            </label>
            <select value={pg.selectedModel} onChange={(e) => pg.setSelectedModel(e.target.value)} disabled={pg.models.length === 0}
              className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 disabled:opacity-50">
              {pg.models.length === 0
                ? <option value="">{t('playground.no_models')}</option>
                : pg.models.map(m => (
                  <option key={m.id} value={m.id}>
                    {isVisionModel(m) ? '[VLM] ' : ''}{m.id}
                  </option>
                ))}
            </select>
            {pg.selectedModelRef && isVisionModel(pg.selectedModelRef) && (
              <div className="mt-1.5 flex items-center gap-1.5 text-[11px] text-green-600">
                <EyeIcon className="w-3.5 h-3.5" />
                <span>Vision model — image upload enabled</span>
              </div>
            )}
          </div>

          {/* Compare Mode Toggle */}
          <div>
            <button onClick={pg.toggleCompareMode}
              className={clsx(
                "w-full flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-sm font-medium transition-all",
                pg.compareMode
                  ? "bg-apple-blue/10 text-apple-blue border border-apple-blue/30"
                  : "bg-apple-gray-50 text-apple-gray-600 border border-apple-gray-200 hover:bg-apple-gray-100"
              )}>
              <ArrowsRightLeftIcon className="w-4 h-4" />
              {pg.compareMode ? 'Compare ON' : 'Compare Models'}
            </button>
          </div>

          {/* Model B */}
          {pg.compareMode && (
            <div>
              <label className="block text-xs font-medium text-apple-gray-700 mb-1.5">Model B</label>
              <select value={pg.compareModel} onChange={(e) => pg.setCompareModel(e.target.value)} disabled={pg.models.length < 2}
                className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 disabled:opacity-50">
                {pg.models.filter(m => m.id !== pg.selectedModel).map(m => (
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
            <textarea rows={3} value={pg.systemPrompt} onChange={(e) => pg.setSystemPrompt(e.target.value)}
              className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 resize-none" />
          </div>

          {/* Temperature */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-apple-gray-700">Temperature</label>
              <span className="text-xs text-apple-gray-500">{pg.temperature}</span>
            </div>
            <input type="range" min="0" max="2" step="0.1" value={pg.temperature}
              onChange={(e) => pg.setTemperature(parseFloat(e.target.value))} className="w-full accent-apple-blue" />
          </div>

          {/* Max Tokens */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-apple-gray-700">Max Tokens</label>
              <span className="text-xs text-apple-gray-500">{pg.maxTokens}</span>
            </div>
            <input type="range" min="100" max="16000" step="100" value={pg.maxTokens}
              onChange={(e) => pg.setMaxTokens(parseInt(e.target.value))} className="w-full accent-apple-blue" />
          </div>

          {/* Token estimate */}
          <div className="flex items-center gap-2 px-3 py-2 bg-apple-gray-50 dark:bg-white/5 rounded-xl border border-apple-gray-100 dark:border-white/10">
            <DocumentDuplicateIcon className="w-3.5 h-3.5 text-apple-gray-400 dark:text-gray-500" />
            <span className="text-[11px] text-apple-gray-500 dark:text-gray-400 font-mono">
              ~{pg.inputTokenEstimate} tokens in context
            </span>
          </div>

          {/* STT / TTS Models */}
          {pg.models.length > 0 && (
            <div>
              <label className="block text-xs font-medium text-apple-gray-700 dark:text-gray-300 mb-1.5">
                <MicrophoneIcon className="w-3.5 h-3.5 inline-block mr-1" />STT Model
              </label>
              <select value={pg.sttModel} onChange={(e) => pg.setSttModel(e.target.value)}
                className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100">
                <option value="">Browser built-in (default)</option>
                {pg.models.map(m => (
                  <option key={m.id} value={m.id}>{isSTTModel(m) ? '[STT] ' : ''}{m.id}</option>
                ))}
              </select>
              <label className="block text-xs font-medium text-apple-gray-700 dark:text-gray-300 mb-1.5 mt-3">
                <SpeakerWaveIcon className="w-3.5 h-3.5 inline-block mr-1" />TTS Model
              </label>
              <select value={pg.ttsModel} onChange={(e) => pg.setTtsModel(e.target.value)}
                className="w-full px-3 py-2 bg-apple-gray-50 dark:bg-white/5 border border-apple-gray-200 dark:border-white/10 rounded-xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100">
                <option value="">{t('common.not_configured')}</option>
                {pg.models.map(m => (
                  <option key={m.id} value={m.id}>{isTTSModel(m) ? '[TTS] ' : ''}{m.id}</option>
                ))}
              </select>
            </div>
          )}
        </div>
      </div>

      {/* ── Main Chat Area ── */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        <div className="h-12 flex items-center justify-between px-4 shrink-0 mb-2">
          <button onClick={() => pg.setShowSettings(!pg.showSettings)}
            className="lg:hidden p-2 -ml-2 text-apple-gray-600 hover:bg-apple-gray-50 rounded-xl">
            <Cog6ToothIcon className="w-5 h-5" />
          </button>
          <div className="flex-1 text-center font-medium text-apple-gray-900 text-sm">
            {pg.compareMode
              ? `${pg.selectedModel} vs ${pg.compareModel}`
              : pg.selectedModel ? `Talking to ${pg.selectedModel}` : 'Playground'}
            {pg.modelSupportsVision && !pg.compareMode && (
              <span className="ml-2 inline-flex items-center gap-1 text-[10px] text-green-600 bg-green-50 px-1.5 py-0.5 rounded-full font-medium">
                <EyeIcon className="w-3 h-3" /> Vision
              </span>
            )}
            {(pg.isStreaming || pg.isStreamingB) && (
              <span className="ml-2 inline-block w-2 h-2 bg-green-400 rounded-full animate-pulse" />
            )}
          </div>
          <button onClick={pg.handleClear} disabled={pg.messages.length === 0 || pg.isStreaming}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-apple-gray-600 hover:text-red-500 hover:bg-red-50 rounded-xl transition-colors disabled:opacity-50">
            <TrashIcon className="w-4 h-4" /> Clear
          </button>
        </div>

        {/* Chat panes */}
        <div className={clsx("flex-1 overflow-hidden", pg.compareMode ? "flex gap-3" : "flex flex-col")}>
          {pg.compareMode ? (
            <>
              <div className="flex-1 flex flex-col min-w-0">
                <ChatPane messages={pg.messages} isStreaming={pg.isStreaming} stats={pg.stats} model={pg.selectedModel} compact />
              </div>
              <div className="flex-1 flex flex-col min-w-0">
                <ChatPane messages={pg.messagesB} isStreaming={pg.isStreamingB} stats={pg.statsB} model={pg.compareModel} compact />
              </div>
            </>
          ) : (
            <div className="flex-1 flex flex-col bg-white dark:bg-[#1C1C1E] rounded-3xl shadow-sm border border-apple-gray-200 dark:border-white/10 overflow-hidden">
              <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-6">
                {pg.messages.length === 0 && (
                  <div className="h-full flex flex-col items-center justify-center text-apple-gray-400">
                    <PlayIcon className="w-12 h-12 mb-4 opacity-50" />
                    <p>Send a message to start playing around.</p>
                    {pg.modelSupportsVision && (
                      <p className="mt-2 text-sm text-green-500 flex items-center gap-1.5">
                        <PhotoIcon className="w-4 h-4" />
                        Paste, drop, or click the attach button to add images
                      </p>
                    )}
                    {pg.models.length > 1 && (
                      <button onClick={pg.toggleCompareMode}
                        className="mt-3 flex items-center gap-1.5 text-sm text-apple-blue hover:underline">
                        <ArrowsRightLeftIcon className="w-4 h-4" />
                        Try Compare Mode
                      </button>
                    )}
                  </div>
                )}
                {pg.messages.map((msg, i) => {
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
                        {msg.role === 'assistant' && text && (
                          <button
                            onClick={() => pg.playTTS(text, i)}
                            disabled={pg.loadingTTSIdx === i}
                            className="mt-2 flex items-center gap-1.5 text-[11px] text-apple-gray-400 dark:text-gray-500 hover:text-apple-blue dark:hover:text-blue-400 transition-colors not-prose"
                            title={pg.playingTTSIdx === i ? 'Stop playback' : 'Read aloud'}
                          >
                            {pg.loadingTTSIdx === i ? (
                              <><span className="w-3.5 h-3.5 border-2 border-apple-gray-300 border-t-transparent rounded-full animate-spin" /> Loading...</>
                            ) : pg.playingTTSIdx === i ? (
                              <><StopIcon className="w-3.5 h-3.5" /> Stop</>
                            ) : (
                              <><SpeakerWaveIcon className="w-3.5 h-3.5" /> Read aloud</>
                            )}
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })}
                <div ref={pg.messagesEndRef} />
              </div>
              <StatsBar stats={pg.stats} model={pg.selectedModel} />
            </div>
          )}
        </div>

        {/* Error Banner */}
        {pg.errorMsg && (
          <div className="mt-2 px-4 py-2.5 bg-red-50 border border-red-100 rounded-2xl text-red-600 text-sm flex items-center gap-2">
            <InformationCircleIcon className="w-5 h-5 shrink-0" />
            <span className="flex-1">{pg.errorMsg}</span>
            <button onClick={() => pg.setErrorMsg('')} className="text-red-400 hover:text-red-600">&times;</button>
          </div>
        )}

        {/* Input Area */}
        <div className="pt-3 shrink-0">
          <AttachmentBar attachments={pg.attachments} onRemove={pg.removeAttachment} />
          <div className="relative flex items-end">
            <textarea
              rows={pg.input.split('\n').length > 1 ? Math.min(pg.input.split('\n').length, 5) : 1}
              value={pg.input}
              onChange={(e) => pg.setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); pg.handleSend(); } }}
              placeholder={pg.isRecording ? 'Recording... click mic to stop' : pg.isTranscribing ? 'Transcribing audio...' : pg.modelSupportsVision ? 'Type a message or paste/drop an image...' : 'Type a message...'}
              className="w-full py-3.5 pl-[4.5rem] pr-24 bg-white dark:bg-[#1C1C1E] border border-apple-gray-200 dark:border-white/10 rounded-2xl focus:ring-2 focus:ring-apple-blue focus:border-transparent text-sm dark:text-gray-100 resize-none shadow-sm"
              style={{ minHeight: '52px' }}
            />
            {/* Attach image button */}
            <button
              onClick={() => pg.fileInputRef.current?.click()}
              disabled={!pg.modelSupportsVision}
              title={pg.modelSupportsVision ? 'Attach image' : 'Select a vision-capable model to enable image uploads'}
              className={clsx(
                "absolute left-3 bottom-3 p-1.5 rounded-lg transition-colors",
                pg.modelSupportsVision
                  ? "text-apple-gray-400 hover:text-apple-blue hover:bg-apple-blue/10"
                  : "text-apple-gray-200 dark:text-gray-600 cursor-not-allowed"
              )}
            >
              <PhotoIcon className="w-5 h-5" />
            </button>
            {/* Microphone / STT button */}
            <button
              onClick={pg.isRecording ? pg.stopRecording : pg.startRecording}
              disabled={pg.isTranscribing}
              title={pg.isRecording ? 'Stop recording' : pg.isTranscribing ? 'Transcribing...' : 'Voice input (Speech-to-Text)'}
              className={clsx(
                "absolute left-10 bottom-3 p-1.5 rounded-lg transition-colors",
                pg.isRecording
                  ? "text-red-500 bg-red-50 dark:bg-red-500/20 animate-pulse"
                  : pg.isTranscribing
                    ? "text-amber-500"
                    : "text-apple-gray-400 hover:text-apple-blue hover:bg-apple-blue/10"
              )}
            >
              {pg.isTranscribing ? (
                <span className="w-5 h-5 flex items-center justify-center">
                  <span className="w-4 h-4 border-2 border-amber-400 border-t-transparent rounded-full animate-spin" />
                </span>
              ) : (
                <MicrophoneIcon className="w-5 h-5" />
              )}
            </button>
            <div className="absolute right-2 bottom-2 flex items-center gap-1.5">
              <span className="text-[10px] text-apple-gray-400 font-mono mr-1">~{estimateTokens(pg.input)} tok</span>
              {pg.isStreaming || pg.isStreamingB ? (
                <button onClick={pg.handleStop}
                  className="p-2 bg-red-500 text-white rounded-xl hover:bg-red-600 transition-colors shadow-sm">
                  <div className="w-4 h-4 rounded-sm bg-white" />
                </button>
              ) : (
                <button onClick={pg.handleSend} disabled={!pg.input.trim() && pg.attachments.length === 0}
                  className="p-2 bg-apple-blue text-white rounded-xl hover:bg-blue-600 transition-colors disabled:opacity-50 shadow-sm">
                  <PaperAirplaneIcon className="w-4 h-4" />
                </button>
              )}
            </div>
          </div>
          <div className="text-center mt-1.5">
            <span className="text-[10px] text-apple-gray-400">
              Enter to send · Shift+Enter for new line
              {pg.modelSupportsVision && ' · Ctrl+V to paste image · Drag & drop images'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
