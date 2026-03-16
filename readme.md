# VC LLM Router Platform

一个统一的大语言模型 (LLM) 多模态网关平台，支持多 Provider 接入（公网 API + 本地自建服务）、API Key 池化与故障切换、代理池管理、计费统计、异步任务与 Webhook 回调、会话记忆和可观测性。

## 功能特性

### 核心路由

- **统一 API 接口**: OpenAI-Compatible API，一套 SDK 对接所有 LLM 平台
- **多 Provider 支持**: OpenAI、Anthropic Claude、Google Gemini、DeepSeek、Mistral、Ollama、LM Studio、vLLM
- **智能路由**: 轮询、加权、最低延迟、成本优化、前缀启发式匹配
- **API Key 池化**: 多 Key 轮转、失败自动切换、配额/限流 error 自动跳过
- **代理池管理**: HTTP/SOCKS5 代理负载均衡与故障转移

### 多模态能力

| 能力 | 端点 | Provider |
|------|------|----------|
| Chat Completions | `/v1/chat/completions` | OpenAI, Claude, Gemini, DeepSeek, Mistral, Ollama, LM Studio, vLLM |
| Streaming (SSE) | `/v1/chat/completions` (stream=true) | 全部 Provider |
| Embeddings | `/v1/embeddings` | OpenAI, Gemini, Mistral |
| Image Generation | `/v1/images/generations` | OpenAI (DALL-E), Google (Imagen) |
| Audio Transcription (STT) | `/v1/audio/transcriptions` | OpenAI (Whisper) |
| **Text-to-Speech (TTS)** | `/v1/audio/speech` | OpenAI TTS (本地: CosyVoice, Fish-Speech, ChatTTS, Bark) |
| **Video Understanding** | `/v1/chat/completions` (multimodal) | Gemini 2.x, GPT-4o (video_url 透传) |
| Model Discovery | `/v1/models` | 聚合所有 Provider + 上游实时查询 |

### 计费 & FinOps

- **Token 计费**: 按 input/output token 分别计价
- **多维度计费**: 支持按秒 (TTS)、按张 (Image)、按分钟 (Video) 计费
- **预算控制**: 月度预算限额 + 阈值告警 (PostgreSQL 持久化)
- **异常检测**: 自动识别用量异常波动
- **用量导出**: CSV 导出，按用户/系统维度
- **多用户隔离**: 每用户独立配额与限流

### 异步任务 & Webhook

- **异步任务管理**: 支持 batch TTS、批量图像生成、视频分析等长耗时任务
- **状态追踪**: pending → running → completed/failed/cancelled 全生命周期
- **进度报告**: 0-100% 实时进度
- **Webhook 回调**: 任务完成/失败时自动 POST 通知到指定 URL

### 会话记忆

- 保存历史对话上下文
- 支持会话恢复和续写
- **会话压缩**: 自动将长对话历史压缩为摘要，降低 Token 消耗

### 健康检查 & 告警

- **Provider 健康检测**: 定时验证各 Provider API Key 可用性
- **代理池监测**: 检测 Proxy 节点连通性和响应延迟
- **自动故障转移**: API Key 失效或代理不可用时自动切换
- **多渠道告警**: Webhook、SMTP 邮件、钉钉 (HMAC 签名)、飞书 (Interactive Card)
- **Prometheus 指标**: `/metrics` 端点，预置 Grafana Dashboard 模板

### 安全

- **JWT 双 Token**: Access Token (15min) + Refresh Token (7d) 旋转机制
- **AES-256-GCM 加密**: Provider API Key 加密存储 (HMAC 完整性校验)
- **审计日志**: 登录、Key 管理、用户变更等关键操作全量记录
- **多级限流**: Global / Per-User / Per-Key / Backpressure 四级速率限制
- **安全头**: HSTS、X-Frame-Options、CSP 等安全响应头
- **注册模式**: open / invite (邀请码) / closed 三种注册策略

### 可观测性

- **Langfuse 集成**: 分布式 Trace + Generation 追踪
- **Prometheus Metrics**: 请求率、错误率、延迟 P95/P99、Token 用量
- **pprof**: 按需启用 (`PPROF_ENABLED=true`)，仅 Admin 可访问
- **结构化日志**: Zap JSON 格式，X-Request-ID 关联

### 管理后台 (Apple Design Style)

- **Dashboard**: 实时数据概览、Provider 状态、模型用量分布
- **Usage 统计**: Token 消耗趋势、调用次数、响应时间分析
- **Provider 管理**: Provider CRUD、API Key 管理、代理切换
- **Proxy 管理**: 代理节点 CRUD、批量导入、连通性测试
- **用户管理**: 角色分配、配额设置、API Key 查看、用量明细
- **Health 监控**: API Key/Provider/Proxy 健康状态、历史记录
- **Settings**: 个人资料、密码修改、主题切换
- **深色模式**: 浅色 / 深色 / 跟随系统
- **API 文档**: 内置交互式文档页 + Swagger UI

## 技术栈

### 后端 (Go)

| 技术 | 用途 |
|------|------|
| Go 1.24+ | 主开发语言 |
| Gin | Web 框架 |
| GORM | ORM (PostgreSQL) |
| go-redis | Redis 客户端 |
| Zap | 结构化日志 |
| Viper | 配置管理 |
| Swaggo | OpenAPI 3.0 文档 |
| jwt-go | JWT 认证 |
| Prometheus | 指标收集 |

### 前端 (React)

| 技术 | 用途 |
|------|------|
| React 19 | UI 框架 |
| TypeScript | 类型安全 |
| Vite | 构建工具 |
| TailwindCSS v4 | 样式框架 |
| Framer Motion | 动画效果 |
| Recharts | 图表可视化 |
| Axios | HTTP 客户端 |
| Zustand | 状态管理 |
| i18next | 国际化 (中/英) |

### 基础设施

| 技术 | 用途 |
|------|------|
| PostgreSQL 16 | 关系数据库 |
| Redis 7 | 缓存 / 限流 / 配额 |
| Docker Compose | 本地编排 |
| Nginx | 前端反代 |
| Helm | Kubernetes 部署 |

## 快速开始

### 环境要求

- Go >= 1.24
- Node.js >= 18.x
- PostgreSQL >= 16
- Redis >= 7.0

### Docker 一键部署 (推荐)

```bash
git clone https://github.com/Veritas-Calculus/llm-router-platform.git
cd llm-router-platform

# 配置环境变量
cp server/.env.example server/.env
# 编辑 server/.env 设置 ENCRYPTION_KEY, JWT_SECRET 等

# 启动所有服务
docker-compose up -d
```

访问 `http://localhost` 即可使用管理后台，API 端点默认在 `http://localhost:8080`。

### 本地开发

```bash
# 启动基础设施
docker-compose up -d postgres redis

# 后端
cd server
cp .env.example .env
go mod download
go run cmd/server/main.go

# 前端 (新终端)
cd web
npm install
npm run dev
```

或使用 Makefile:

```bash
make dev          # 启动后端 + 前端开发服务器
make test         # 运行全量测试 (Go + ESLint)
make lint         # golangci-lint + eslint
make build        # 生产构建
```

### 配置说明

后端配置 `server/.env`:

```env
# 必须配置
ENCRYPTION_KEY=your-32-byte-encryption-key    # AES-256 加密密钥
JWT_SECRET=your-jwt-secret-at-least-32-chars  # JWT 签名密钥

# 数据库
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=llm_router

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# 默认管理员 (首次启动自动创建)
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=changeme

# 注册模式: open / invite / closed
REGISTRATION_MODE=open

# 可观测性 (可选)
LANGFUSE_ENABLED=false
LANGFUSE_PUBLIC_KEY=pk-lf-...
LANGFUSE_SECRET_KEY=sk-lf-...
LANGFUSE_HOST=https://cloud.langfuse.com

# pprof 调试 (可选)
PPROF_ENABLED=false
```

前端配置 `web/.env`:

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_APP_TITLE=LLM Router Platform
```

## API 参考

### OpenAI-Compatible LLM 端点

这些端点支持三种 URL 前缀以兼容不同 SDK 配置：`/api/v1/`、`/v1/`、`/`

| 端点 | 方法 | 描述 | 认证 |
|------|------|------|------|
| `/v1/chat/completions` | POST | 对话补全 (支持 streaming) | API Key |
| `/v1/embeddings` | POST | 文本向量化 | API Key |
| `/v1/images/generations` | POST | 图像生成 | API Key |
| `/v1/audio/transcriptions` | POST | 语音转文字 (Whisper) | API Key |
| `/v1/audio/speech` | POST | 文字转语音 (TTS) | API Key |
| `/v1/models` | GET | 可用模型列表 | API Key |
| `/v1/models/providers` | GET | Provider 模型列表 | API Key |

### 管理 API (`/api/v1/`)

#### 认证

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/auth/register` | POST | 用户注册 |
| `/api/v1/auth/login` | POST | 登录 (返回 JWT) |
| `/api/v1/auth/refresh` | POST | 刷新 Access Token |
| `/api/v1/auth/token/rotate` | POST | Refresh Token 旋转 |

#### 用户功能 (JWT 认证)

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/user/profile` | GET/PUT | 个人资料 |
| `/api/v1/user/password` | PUT | 修改密码 |
| `/api/v1/api-keys` | GET/POST | API Key 管理 |
| `/api/v1/api-keys/:id/revoke` | POST | 吊销 Key |
| `/api/v1/usage/summary` | GET | 用量概览 |
| `/api/v1/usage/daily` | GET | 每日用量 |
| `/api/v1/usage/by-provider` | GET | 按 Provider 用量 |
| `/api/v1/usage/export/csv` | GET | 导出 CSV |
| `/api/v1/finops/budget` | GET/PUT/DELETE | 预算管理 |
| `/api/v1/finops/anomaly` | GET | 异常检测 |
| `/api/v1/tasks` | GET/POST | 异步任务管理 |
| `/api/v1/tasks/:id` | GET | 任务详情 |
| `/api/v1/tasks/:id/cancel` | POST | 取消任务 |
| `/api/v1/dashboard/*` | GET | Dashboard 数据 |

#### 管理员功能 (Admin Only)

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/users` | GET | 用户列表 |
| `/api/v1/users/:id` | GET | 用户详情 |
| `/api/v1/users/:id/toggle` | POST | 启用/禁用用户 |
| `/api/v1/users/:id/role` | PUT | 修改角色 |
| `/api/v1/users/:id/quota` | PUT | 设置配额 |
| `/api/v1/providers` | GET | Provider 列表 |
| `/api/v1/providers/:id` | PUT | 更新 Provider |
| `/api/v1/providers/:id/api-keys` | GET/POST | Provider API Key 管理 |
| `/api/v1/proxies` | GET/POST | 代理节点管理 |
| `/api/v1/proxies/batch` | POST | 批量导入代理 |
| `/api/v1/health/api-keys` | GET | API Key 健康状态 |
| `/api/v1/health/providers` | GET | Provider 健康状态 |
| `/api/v1/health/proxies` | GET | 代理健康状态 |
| `/api/v1/alerts` | GET | 告警列表 |
| `/api/v1/alerts/config` | GET/PUT | 告警配置 |

### 运维端点

| 端点 | 描述 |
|------|------|
| `/health` | Liveness 探针 |
| `/healthz` | Readiness 探针 (PG + Redis) |
| `/readyz` | Ready 探针 |
| `/version` | 版本信息 |
| `/metrics` | Prometheus 指标 |
| `/swagger/*` | OpenAPI 文档 |

### 使用示例

```bash
# Chat Completion
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello!"}]}'

# TTS 语音合成
curl -X POST http://localhost:8080/v1/audio/speech \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"tts-1","input":"Hello from LLM Router","voice":"alloy"}' \
  --output speech.mp3

# 创建异步任务
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer jwt-token" \
  -H "Content-Type: application/json" \
  -d '{"type":"batch_tts","input":"[{\"text\":\"hello\"}]","webhook_url":"https://example.com/callback"}'

# 视频理解 (multimodal)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-2.0-flash","messages":[{"role":"user","content":[{"type":"text","text":"描述这个视频"},{"type":"video_url","video_url":{"url":"https://example.com/video.mp4"}}]}]}'
```

## 项目结构

```
llm-router-platform/
├── server/                          # Go 后端
│   ├── cmd/
│   │   ├── server/                  # 主程序入口
│   │   └── migrate/                 # 数据库迁移工具
│   ├── internal/
│   │   ├── api/
│   │   │   ├── handlers/            # 请求处理器 (19 个 handler 文件)
│   │   │   │   ├── chat_handler.go          # Chat Completions
│   │   │   │   ├── streaming_handler.go     # SSE 流式响应
│   │   │   │   ├── tts_handler.go           # TTS 语音合成
│   │   │   │   ├── audio_handler.go         # 音频转写
│   │   │   │   ├── images_handler.go        # 图像生成
│   │   │   │   ├── embeddings_handler.go    # 文本向量化
│   │   │   │   ├── task_handler.go          # 异步任务管理
│   │   │   │   ├── auth_handler.go          # 认证 & 注册
│   │   │   │   ├── provider_handler.go      # Provider 管理
│   │   │   │   ├── dashboard_handler.go     # Dashboard 数据
│   │   │   │   ├── finops_handler.go        # 预算 & 异常检测
│   │   │   │   └── ...
│   │   │   ├── middleware/          # 中间件 (CORS, 限流, JWT, 安全头, Backpressure)
│   │   │   └── routes/             # 路由注册
│   │   ├── config/                  # Viper 配置管理
│   │   ├── crypto/                  # AES-GCM + HMAC 加密
│   │   ├── database/                # GORM AutoMigrate + 数据清理
│   │   ├── models/                  # 数据模型 (User, Provider, Model, UsageLog, AsyncTask, ...)
│   │   ├── repository/              # 数据访问层
│   │   └── service/                 # 业务逻辑层 (11 个子模块)
│   │       ├── provider/            # LLM Provider 适配 (7 个 Provider)
│   │       ├── router/              # 路由策略引擎
│   │       ├── billing/             # 计费 & FinOps & Budget
│   │       ├── task/                # 异步任务 & Webhook
│   │       ├── health/              # 健康检查 & 告警调度
│   │       ├── memory/              # 会话记忆 & 压缩
│   │       ├── audit/               # 审计日志
│   │       ├── notification/        # 多渠道通知
│   │       ├── observability/       # Langfuse + Prometheus
│   │       ├── proxy/               # 代理池管理
│   │       └── user/                # 用户服务
│   ├── pkg/                         # 公共工具包
│   ├── docs/                        # Swagger 文档
│   └── go.mod
│
├── web/                             # React 前端
│   ├── src/
│   │   ├── pages/                   # 页面 (12 个页面)
│   │   │   ├── DashboardPage.tsx
│   │   │   ├── UsagePage.tsx
│   │   │   ├── ProvidersPage.tsx
│   │   │   ├── ProxiesPage.tsx
│   │   │   ├── UsersPage.tsx
│   │   │   ├── UserDetailPage.tsx
│   │   │   ├── ApiKeysPage.tsx
│   │   │   ├── HealthPage.tsx
│   │   │   ├── SettingsPage.tsx
│   │   │   ├── DocsPage.tsx
│   │   │   ├── LoginPage.tsx
│   │   │   └── ForcePasswordChangePage.tsx
│   │   ├── components/              # 通用组件
│   │   ├── stores/                  # Zustand 状态管理
│   │   ├── hooks/                   # 自定义 Hooks
│   │   ├── lib/                     # 工具库 (API client, i18n)
│   │   ├── locales/                 # 国际化资源 (zh/en)
│   │   └── test/                    # 测试
│   ├── package.json
│   └── vite.config.ts
│
├── deploy/                          # 部署配置
├── helm/                            # Helm Chart
├── examples/                        # Python 客户端示例
├── docker-compose.yml               # 生产编排
├── docker-compose.dev.yml           # 开发编排
├── Makefile                         # 开发任务
└── CONTRIBUTING.md                  # 贡献指南
```

## 开发计划

- [x] 多 LLM Provider 支持 (OpenAI, Claude, Gemini, DeepSeek, Mistral, Ollama, LM Studio, vLLM)
- [x] 流式响应 (SSE) + Tool Call 支持
- [x] 管理后台 Dashboard (Apple Design)
- [x] API Key 池化 + 自动故障切换
- [x] 代理池管理 + 健康检查
- [x] 计费 & 用量统计 & FinOps
- [x] JWT 双 Token + Refresh 旋转
- [x] AES-256 加密存储 + 审计日志
- [x] 多级速率限制 (Global/Per-User/Per-Key/Backpressure)
- [x] 多渠道告警 (Webhook/Email/钉钉/飞书)
- [x] 会话记忆压缩
- [x] 预算持久化 + 异常检测
- [x] Prometheus + Grafana + Langfuse 可观测性
- [x] OpenAPI 3.0 文档 (Swagger UI)
- [x] TTS 多 Provider 接入 (OpenAI TTS + 本地 CosyVoice/Fish-Speech)
- [x] Video Understanding (Multimodal 透传)
- [x] 异步任务 + Webhook 回调系统
- [x] 多维度计费 (按秒/按张/按分钟)
- [x] 国际化 (i18n: 中/英)
- [ ] Kubernetes 生产部署 (Helm Chart)
- [ ] 移动端适配

## 贡献指南

欢迎提交 Issue 和 Pull Request。请确保：

1. 后端代码通过 `golangci-lint` 检查
2. 前端代码通过 `ESLint` 检查
3. 所有测试通过 (`cd server && go test ./...`)
4. 更新相关文档

详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

MIT License
