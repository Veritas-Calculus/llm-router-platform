import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Link } from 'react-router-dom';
import {
  BookOpenIcon,
  CodeBracketIcon,
  CommandLineIcon,
  CubeIcon,
  ClipboardDocumentIcon,
  CheckIcon,
  PlayIcon,
  ArrowTopRightOnSquareIcon,
} from '@heroicons/react/24/outline';
import clsx from 'clsx';
import { useTranslation } from '@/lib/i18n';

/* ── Copy button ───────────────────────────────────────────────── */
function CopyButton({ text }: { text: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000); }}
      className="absolute top-2 right-2 p-1.5 rounded-lg bg-white/10 hover:bg-white/20 text-apple-gray-400 hover:text-white transition-all"
      title="Copy"
    >
      {copied ? <CheckIcon className="w-4 h-4 text-green-400" /> : <ClipboardDocumentIcon className="w-4 h-4" />}
    </button>
  );
}

/* ── Code block with copy ──────────────────────────────────────── */
function CodeBlock({ code }: { code: string; lang?: string }) {
  return (
    <div className="relative group">
      <pre className="bg-apple-gray-900 text-apple-gray-100 text-sm rounded-2xl p-4 overflow-x-auto font-mono leading-relaxed">
        <code>{code}</code>
      </pre>
      <CopyButton text={code} />
    </div>
  );
}

/* ── Tab interface ─────────────────────────────────────────────── */
const BASE_URL = `${window.location.protocol}//${window.location.host}/v1`;

const tabs = [
  { id: 'quickstart', label: 'Quick Start', icon: BookOpenIcon },
  { id: 'api', label: 'API Reference', icon: CodeBracketIcon },
  { id: 'sdk', label: 'SDKs & Examples', icon: CubeIcon },
  { id: 'mcp', label: 'MCP Protocol', icon: CommandLineIcon },
] as const;

type TabId = (typeof tabs)[number]['id'];

/* ── Endpoint card ─────────────────────────────────────────────── */
function EndpointCard({ method, path, description, children }: {
  method: string; path: string; description: string; children?: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="border border-apple-gray-200 rounded-2xl overflow-hidden">
      <button onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 px-5 py-4 hover:bg-apple-gray-50 transition-colors text-left">
        <span className={clsx(
          "px-2.5 py-0.5 rounded-lg text-xs font-bold uppercase tracking-wide",
          method === 'POST' ? 'bg-green-100 text-green-700' : 'bg-blue-100 text-blue-700'
        )}>
          {method}
        </span>
        <code className="text-sm font-mono text-apple-gray-800 font-semibold">{path}</code>
        <span className="text-sm text-apple-gray-500 ml-auto">{description}</span>
      </button>
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="border-t border-apple-gray-100 overflow-hidden"
          >
            <div className="p-5 space-y-4">{children}</div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

/* ── Main component ────────────────────────────────────────────── */
function DocsPage() {
  const [activeTab, setActiveTab] = useState<TabId>('quickstart');

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-apple-gray-900">Documentation</h1>
          <p className="text-apple-gray-500 mt-1 text-sm">API reference, SDKs, and integration guides</p>
        </div>
        <Link to="/playground"
          className="flex items-center gap-2 px-4 py-2 bg-apple-blue text-white rounded-xl text-sm font-semibold hover:bg-blue-600 transition-colors shadow-sm">
          <PlayIcon className="w-4 h-4" /> Open Playground
        </Link>
      </div>

      {/* Tab bar */}
      <div className="flex bg-apple-gray-100 rounded-xl p-1 border border-apple-gray-200">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={clsx(
              "flex-1 flex items-center justify-center gap-2 py-2.5 text-sm font-semibold rounded-lg transition-all duration-200",
              activeTab === tab.id
                ? 'bg-white text-apple-blue shadow-sm border border-apple-gray-200'
                : 'text-apple-gray-500 hover:text-apple-gray-700'
            )}
          >
            <tab.icon className="w-4 h-4" />
            <span className="hidden sm:inline">{tab.label}</span>
          </button>
        ))}
      </div>

      {/* Tab content */}
      <AnimatePresence mode="wait">
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          transition={{ duration: 0.2 }}
        >
          {activeTab === 'quickstart' && <QuickStartTab />}
          {activeTab === 'api' && <ApiReferenceTab />}
          {activeTab === 'sdk' && <SdkTab />}
          {activeTab === 'mcp' && <McpTab />}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}

/* ── Quick Start ───────────────────────────────────────────────── */
function QuickStartTab() {
  return (
    <div className="space-y-6">
      {/* Steps */}
      {[
        {
          step: 1,
          title: 'Get your API Key',
          desc: 'Navigate to the API Keys page to create a new key. Copy and save it — you won\'t see it again.',
          action: <Link to="/api-keys" className="text-sm text-apple-blue hover:underline font-medium flex items-center gap-1">Go to API Keys <ArrowTopRightOnSquareIcon className="w-3 h-3" /></Link>,
        },
        {
          step: 2,
          title: 'Make your first request',
          desc: 'Use any HTTP client or OpenAI-compatible SDK. Simply change the base URL.',
          code: `curl ${BASE_URL}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`,
        },
        {
          step: 3,
          title: 'Explore in the Playground',
          desc: 'Use our built-in Playground to test prompts, compare models side-by-side, and monitor token usage.',
          action: <Link to="/playground" className="text-sm text-apple-blue hover:underline font-medium flex items-center gap-1">Open Playground <ArrowTopRightOnSquareIcon className="w-3 h-3" /></Link>,
        },
      ].map(item => (
        <div key={item.step} className="bg-white rounded-2xl p-6 border border-apple-gray-200 shadow-sm">
          <div className="flex items-start gap-4">
            <div className="w-8 h-8 rounded-full bg-apple-blue text-white flex items-center justify-center text-sm font-bold shrink-0">
              {item.step}
            </div>
            <div className="flex-1 min-w-0">
              <h3 className="text-base font-semibold text-apple-gray-900 mb-1">{item.title}</h3>
              <p className="text-sm text-apple-gray-500 mb-3">{item.desc}</p>
              {item.code && <CodeBlock code={item.code} />}
              {item.action && <div className="mt-2">{item.action}</div>}
            </div>
          </div>
        </div>
      ))}

      {/* Key info */}
      <div className="bg-blue-50 rounded-2xl p-5 border border-blue-100">
        <h4 className="font-semibold text-blue-800 mb-2">Tip: OpenAI SDK Compatible</h4>
        <p className="text-sm text-blue-700">
          Any library that supports OpenAI's API format works out of the box.
          Just set <code className="bg-blue-100 px-1.5 py-0.5 rounded text-xs">base_url</code> to <code className="bg-blue-100 px-1.5 py-0.5 rounded text-xs">{BASE_URL}</code>.
        </p>
      </div>
    </div>
  );
}

/* ── API Reference ─────────────────────────────────────────────── */
function ApiReferenceTab() {
  return (
    <div className="space-y-4">
      <div className="bg-white rounded-2xl p-5 border border-apple-gray-200 shadow-sm mb-6">
        <h3 className="font-semibold text-apple-gray-900 mb-2">Authentication</h3>
        <p className="text-sm text-apple-gray-600 mb-3">All API requests require a Bearer token in the <code className="bg-apple-gray-100 px-1.5 py-0.5 rounded text-xs">Authorization</code> header.</p>
        <CodeBlock code={`Authorization: Bearer YOUR_API_KEY`} />
      </div>

      <EndpointCard method="POST" path="/v1/chat/completions" description="Create a chat completion">
        <p className="text-sm text-apple-gray-600">Generate a model response for a conversation. Supports streaming via SSE.</p>
        <h4 className="text-sm font-semibold text-apple-gray-800 mt-3">Request Body</h4>
        <div className="text-sm space-y-1 mt-2">
          <div className="flex gap-2"><code className="text-apple-blue font-mono text-xs">model</code> <span className="text-apple-gray-500">string, required</span></div>
          <div className="flex gap-2"><code className="text-apple-blue font-mono text-xs">messages</code> <span className="text-apple-gray-500">array of {`{role, content}`}, required</span></div>
          <div className="flex gap-2"><code className="text-apple-blue font-mono text-xs">temperature</code> <span className="text-apple-gray-500">float, 0-2, default 1</span></div>
          <div className="flex gap-2"><code className="text-apple-blue font-mono text-xs">max_tokens</code> <span className="text-apple-gray-500">integer, optional</span></div>
          <div className="flex gap-2"><code className="text-apple-blue font-mono text-xs">stream</code> <span className="text-apple-gray-500">boolean, default false</span></div>
          <div className="flex gap-2"><code className="text-apple-blue font-mono text-xs">tools</code> <span className="text-apple-gray-500">array, optional (MCP tools auto-injected)</span></div>
        </div>
        <CodeBlock lang="json" code={`{
  "model": "gpt-4o-mini",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user", "content": "What is 2+2?"}
  ],
  "temperature": 0.7,
  "stream": true
}`} />
      </EndpointCard>

      <EndpointCard method="POST" path="/v1/embeddings" description="Create embeddings">
        <p className="text-sm text-apple-gray-600">Generate vector embeddings for text input.</p>
        <CodeBlock code={`{
  "model": "text-embedding-3-small",
  "input": "Hello world"
}`} />
      </EndpointCard>

      <EndpointCard method="POST" path="/v1/images/generations" description="Generate images">
        <p className="text-sm text-apple-gray-600">Create images from text prompts.</p>
        <CodeBlock code={`{
  "model": "dall-e-3",
  "prompt": "A sunset over mountains",
  "size": "1024x1024"
}`} />
      </EndpointCard>

      <EndpointCard method="POST" path="/v1/audio/speech" description="Text to speech">
        <p className="text-sm text-apple-gray-600">Generate audio from text input.</p>
        <CodeBlock code={`{
  "model": "tts-1",
  "input": "Hello, how are you?",
  "voice": "alloy"
}`} />
      </EndpointCard>

      <EndpointCard method="GET" path="/v1/models" description="List available models">
        <p className="text-sm text-apple-gray-600">Returns a list of all active models available through the router.</p>
        <CodeBlock code={`curl ${BASE_URL}/models \\
  -H "Authorization: Bearer YOUR_API_KEY"`} />
      </EndpointCard>
    </div>
  );
}

/* ── SDKs & Examples ───────────────────────────────────────────── */
function SdkTab() {
  const [lang, setLang] = useState<'curl' | 'python' | 'node' | 'go'>('python');

  const examples: Record<string, { label: string; code: string }> = {
    curl: {
      label: 'cURL',
      code: `curl ${BASE_URL}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $API_KEY" \\
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'`,
    },
    python: {
      label: 'Python',
      code: `from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="${BASE_URL}"
)

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello!"}],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")`,
    },
    node: {
      label: 'Node.js',
      code: `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "YOUR_API_KEY",
  baseURL: "${BASE_URL}",
});

const stream = await client.chat.completions.create({
  model: "gpt-4o-mini",
  messages: [{ role: "user", content: "Hello!" }],
  stream: true,
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content || "");
}`,
    },
    go: {
      label: 'Go',
      code: `package main

import (
  "context"
  "fmt"
  openai "github.com/sashabaranov/go-openai"
)

func main() {
  cfg := openai.DefaultConfig("YOUR_API_KEY")
  cfg.BaseURL = "${BASE_URL}"
  client := openai.NewClientWithConfig(cfg)

  resp, _ := client.CreateChatCompletion(
    context.Background(),
    openai.ChatCompletionRequest{
      Model: "gpt-4o-mini",
      Messages: []openai.ChatCompletionMessage{
        {Role: "user", Content: "Hello!"},
      },
    },
  )
  fmt.Println(resp.Choices[0].Message.Content)
}`,
    },
  };

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-2xl p-5 border border-apple-gray-200 shadow-sm">
        <h3 className="font-semibold text-apple-gray-900 mb-1">SDK Compatibility</h3>
        <p className="text-sm text-apple-gray-500 mb-4">
          Use any OpenAI-compatible SDK — just change the <code className="bg-apple-gray-100 px-1.5 py-0.5 rounded text-xs">base_url</code> to point to this platform.
        </p>

        {/* Language tabs */}
        <div className="flex gap-1 bg-apple-gray-100 rounded-xl p-1 mb-4">
          {(['curl', 'python', 'node', 'go'] as const).map(l => (
            <button key={l} onClick={() => setLang(l)}
              className={clsx(
                "flex-1 py-2 text-sm font-semibold rounded-lg transition-all",
                lang === l ? 'bg-white text-apple-blue shadow-sm' : 'text-apple-gray-500 hover:text-apple-gray-700'
              )}>
              {examples[l].label}
            </button>
          ))}
        </div>

        <CodeBlock code={examples[lang].code} lang={lang} />
      </div>

      {/* Installation */}
      <div className="bg-white rounded-2xl p-5 border border-apple-gray-200 shadow-sm">
        <h3 className="font-semibold text-apple-gray-900 mb-3">Installation</h3>
        <div className="space-y-3">
          <div>
            <p className="text-xs font-medium text-apple-gray-500 mb-1">Python</p>
            <CodeBlock code="pip install openai" />
          </div>
          <div>
            <p className="text-xs font-medium text-apple-gray-500 mb-1">Node.js</p>
            <CodeBlock code="npm install openai" />
          </div>
          <div>
            <p className="text-xs font-medium text-apple-gray-500 mb-1">Go</p>
            <CodeBlock code="go get github.com/sashabaranov/go-openai" />
          </div>
        </div>
      </div>
    </div>
  );
}

/* ── MCP Protocol ──────────────────────────────────────────────── */
function McpTab() {
  return (
    <div className="space-y-6">
      <div className="bg-white rounded-2xl p-6 border border-apple-gray-200 shadow-sm">
        <h3 className="text-lg font-semibold text-apple-gray-900 mb-2">Model Context Protocol</h3>
        <p className="text-sm text-apple-gray-600 mb-4">
          MCP extends LLMs with real-world capabilities. When MCP servers are configured in the admin panel,
          tools are automatically injected into chat completion requests — no client-side changes required.
        </p>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          {[
            { label: 'Server Config', desc: 'Add MCP servers via admin panel with connection URLs' },
            { label: 'Auto Injection', desc: 'Tools are injected into /chat/completions automatically' },
            { label: 'Zero Changes', desc: 'Your existing OpenAI SDK code works without modification' },
          ].map(item => (
            <div key={item.label} className="bg-apple-gray-50 rounded-xl p-4 border border-apple-gray-100">
              <h4 className="text-sm font-semibold text-apple-gray-800 mb-1">{item.label}</h4>
              <p className="text-xs text-apple-gray-500">{item.desc}</p>
            </div>
          ))}
        </div>

        <h4 className="text-sm font-semibold text-apple-gray-800 mb-2">How it Works</h4>
        <CodeBlock code={`# 1. Admin adds MCP server (e.g., a web search tool)
# 2. Your regular API call automatically gets tool access:

curl ${BASE_URL}/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Search for latest AI news"}]
  }'
# The model can now use the search tool automatically`} />
      </div>
    </div>
  );
}

export default DocsPage;
