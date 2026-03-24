# API Reference — OpenAI 兼容端点

VC LLM Router 提供完全兼容 OpenAI SDK 的 REST API。所有 LLM 端点均支持 `Authorization: Bearer <API_KEY>` 或 `X-API-Key: <API_KEY>` 认证。

## 基础信息

| 项目 | 值 |
|------|-----|
| Base URL | `http://<host>:8080/v1` |
| 认证方式 | `Authorization: Bearer <YOUR_API_KEY>` |
| 内容类型 | `application/json` |
| 管理 API | GraphQL (`POST /graphql`)，详见 [GraphQL Guide](graphql-guide.md) |

> **兼容模式**: 端点同时注册在 `/v1/`、`/api/v1/` 和根路径 `/`，以适配不同 SDK 的 base URL 配置方式。

---

## 认证

API Key 通过管理后台 (Dashboard → API Keys) 或 GraphQL `createApiKey` mutation 创建。

```bash
# OpenAI SDK 标准方式
curl -H "Authorization: Bearer llm_xxxxx" ...

# 备选 Header
curl -H "X-API-Key: llm_xxxxx" ...
```

每个 API Key 绑定到一个 **Project**，Project 属于一个 **Organization**。计费和用量跟踪均按 Project 维度隔离。

---

## Chat Completions

创建聊天补全，支持流式和非流式。

```
POST /v1/chat/completions
```

### 请求体

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 1024
}
```

### 流式响应 (SSE)

设置 `"stream": true`，响应为 Server-Sent Events 格式：

```
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"Hello"}}]}
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"!"}}]}
data: [DONE]
```

### 多模态

通过 messages 中嵌入图片/视频 URL 实现多模态请求：

```json
{
  "model": "gpt-4o",
  "messages": [{
    "role": "user",
    "content": [
      {"type": "text", "text": "What's in this image?"},
      {"type": "image_url", "image_url": {"url": "https://..."}}
    ]
  }]
}
```

### Tool Call (Function Calling)

支持 OpenAI-compatible Tool Call，包括 MCP 自动注入的工具。

---

## Embeddings

```
POST /v1/embeddings
```

```json
{
  "model": "text-embedding-3-small",
  "input": "Hello world"
}
```

支持 Provider: OpenAI, Gemini, Mistral, 及自部署模型。

---

## Image Generation

```
POST /v1/images/generations
```

```json
{
  "model": "dall-e-3",
  "prompt": "A sunset over mountains",
  "size": "1024x1024",
  "n": 1
}
```

支持 Provider: OpenAI (DALL-E), Google (Imagen)。

---

## Audio Transcription (STT)

```
POST /v1/audio/transcriptions
Content-Type: multipart/form-data
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `file` | binary | 音频文件 |
| `model` | string | 模型名 (e.g. `whisper-1`) |

---

## Text-to-Speech (TTS)

```
POST /v1/audio/speech
```

```json
{
  "model": "tts-1",
  "input": "Hello, how are you?",
  "voice": "alloy"
}
```

响应为音频流 (`audio/mpeg`)。支持 OpenAI TTS 和本地兼容服务 (CosyVoice, ChatTTS)。

---

## Models

```
GET /v1/models              # 列出所有可用模型
GET /v1/models/providers    # 按 Provider 分组列出
GET /v1/models/{model_id}   # 获取单个模型详情
```

模型列表通过上游 Provider 实时同步。

---

## Anthropic 兼容路由

```
POST /v1/v1/messages
```

兼容 Anthropic SDK 的 Messages API 格式，自动映射到路由引擎。

---

## 运维端点

| 端点 | 说明 | 认证 |
|------|------|------|
| `GET /health` | Liveness (K8s) — 始终返回 200 | 无 |
| `GET /healthz` | Deep Health — 检查 PG、Redis、迁移版本 | 无 |
| `GET /readyz` | Readiness (K8s) — 检查 PG 连通性 | 无 |
| `GET /version` | 版本/构建信息 | 无 |
| `GET /metrics` | Prometheus 指标 | JWT + Admin |
| `GET /internal/metrics` | Prometheus (无认证，需开启 Feature Gate) | 无 |
| `GET /openapi.json` | OpenAPI 3.0 规范 | 无 |
| `GET /swagger/*` | Swagger UI (需开启 Feature Gate) | 无 |

---

## 错误格式

所有错误均返回 OpenAI-compatible JSON：

```json
{
  "error": {
    "message": "具体错误描述",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
```

## Rate Limiting

- **全局限流**: 按 `RATE_LIMIT_REQUESTS_PER_MINUTE` 配置
- **Per-Key 限流**: 每个 API Key 可独立设置 `rateLimit` (次/分钟)
- **Token 配额**: 每个 API Key 可设置 `tokenLimit` (月)
- **背压保护**: 当数据库连接池负载过高时自动返回 503

响应头包含限流信息：
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 58
Retry-After: 30
```
