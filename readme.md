# VC LLM Router Platform

一个统一的大语言模型 (LLM) 多模态网关平台，支持多 Provider 接入（公网 API + 本地自建服务）、API Key 池化与故障切换、代理池管理、计费统计、异步任务与 Webhook 回调、**组织级多租户**、**语义缓存 (Semantic Cache)**、**数据防泄漏 (DLP)**、以及 **MCP (Model Context Protocol)** 扩展。

## 功能特性

### 核心路由与扩展

- **统一 API 接口**: 完全兼容 OpenAI，一套 SDK 对接所有主流 LLM 平台
- **多 Provider 支持**: OpenAI、Anthropic Claude、Google Gemini、DeepSeek、Mistral、Ollama、LM Studio、vLLM
- **智能路由**: 轮询、加权、最低延迟、成本优化、前缀及组织级启发式匹配
- **API Key 池化**: 多 Key 轮转、失败自动健康剔除并秒级切换、配额自动隔离
- **代理池管理**: HTTP/SOCKS5 代理负载均衡与故障转移
- **语义缓存 (Semantic Cache)**: 基于 Redis 和轻量级嵌入向量，实现请求复用，命中后延迟至毫秒级并免除 LLM 成本
- **MCP (Model Context Protocol)**: 原生集成 stdio 和 SSE 客户端，无缝桥接任何标准化本地数据源和外部工具

### 多模态能力

| 能力 | 端点 | Provider |
|------|------|----------|
| Chat Completions | `/v1/chat/completions` | 当前支持的所有供应商及本地部署 |
| Streaming (SSE) | `/v1/chat/completions` (stream) | 全部 Provider 支持 |
| Embeddings | `/v1/embeddings` | OpenAI, Gemini, Mistral 及自部署 |
| Image Generation | `/v1/images/generations` | OpenAI (DALL-E), Google (Imagen) |
| Audio (STT) | `/v1/audio/transcriptions` | OpenAI (Whisper) |
| **TTS** | `/v1/audio/speech` | OpenAI TTS (本地兼容: CosyVoice, ChatTTS 等) |
| **多模态解析** | `/v1/chat/completions` | 多路透传支持 (Image/Video_url 等) |
| Model Discovery | `/v1/models` | 动态路由聚合及模型信息上游实时同步 |

### 组织与多租户 (Workspaces)

- **多租户隔离**: 支持单实例虚拟成多个平行的组织空间 (Organization) 和工作区，不同租户数据硬隔离
- **RBAC 人员管理**: 支持细粒度成员角色（Admin, Member），在组织内共享计费与基础资源
- **配置隔离**: 针对每个组织配置专属 API Key、路由优先级、缓存容忍度以及模型准入黑名单
- **SSO 企业联邦认证**: 原生融合 Google、GitHub，基于 OIDC 和 JWT 支持跨域单点登录

### 计费 & FinOps

- **实时 Token 计费**: 精确记录 Prompt 和 Completion 消耗并折算成本
- **多维度计费**: 支持按时间(秒)、按内容尺寸(图片)、按量计算
- **预算控制**: 以租户组织为中心划分财务边界，可单独设施日/月度用量限额 (PostgreSQL 锁账)
- **异动检测**: 针对极速拉升的 Token 消耗速率进行多阶梯度风控拦截

### 会话与任务

- **对话记忆**: 保存历史上下文状态追踪并在服务端做智能长文本压缩，降低消耗
- **异步批处理**: 针对大规模并行生成（如图片跑批、TTS、多视频理解等长耗时作业）管理任务池生命周期
- **Webhook 回调**: 异步执行完成后（或失败时），向下游回调并支持验签重试

### 安全与合规 (Security & DLP)

- **DLP (数据防泄漏)**: 企业级双向内容安检，根据策略实时拦截敏感词汇 (Block) 或进行就地数据脱敏打码 (Mask/Redact)
- **身份保护**: 强制 JWT (Access + Refresh 双重验证)，提供强制一次性令牌与失效重认证功能
- **加密落盘**: Provider 鉴权凭证强制 AES-256-GCM 高度算法库软加密回源
- **安全防线**: 自动跳变限流桶、全局背压(Backpressure)防雪崩、SSRF（服务器端请求伪造）严格防护 (针对 Webhook 和多模态链接）
- **日志审计**: Webhook、人员登录操作与 Token 重置关键流程被安全审计中心长时持久化保存

### 健康监控 & 可观测性

- **实时健康探针**: 全局异步探测 Provider API Endpoint 连通性，异常时驱逐下线，复效时重新汇聚
- **多通道告警推送**: 支持 SMTP 邮件、企业微信、钉钉通知 (按规则带 HMAC 签名校验)，在链路预警时发出
- **Prometheus + Langfuse 集成**: 对系统全线做 X-Request-ID 染色链路跟踪。按需提供 CPU/MEM `/pprof` 全栈压测

### 管理后台 (Apple Design Style)

- **极简拟物视效**: 以“平滑与简洁”为主基调，浅色/深色/自适应全态环境的高端质感交互 UI
- **动静结合的路由配置**: 支持通过可视化网格进行高阶 `Routing Engine` 规则的拖拽化与权重调整
- **管理分界**:
  - **User Dashboard** (普通用户/组织管理员): 管理自己租户内的 API Key、Prompt 广场、账单开销与缓存命中率
  - **Admin Control Panel** (平台总超管): 审阅全局基建(Provider池/Proxy代理池)，干预整体 DLP 和 SSO 配置以及资源水位监控

## 技术架构

### 后端 (Go)
| Core | Use |
|------|-----|
| Go 1.24+ | 后端核心引擎 |
| **gqlgen** | API 交付主层级（Schema First 的 GraphQL 端点） |
| Gin | REST 降级及原生 `/v1` 的 LLM 接口实现 |
| GORM | 强类型数据存储对接层 (PostgreSQL 16) |
| go-redis | RateLimiting / TokenBucket / SemanticCache 层数据桥接 |
| Zap | Async JSON 结构化高性能日志 |

### 前端 (React)
| Stack | Use |
|-------|-----|
| React 19 + TypeScript | 前级界面 |
| Vite | 秒级打包构建引擎 |
| TailwindCSS v4 | 标准化原子化 CSS (配合 `lucide`/`heroicons` 剔除表情符号) |
| **Apollo Client** | GQL 同步请求框架与全局本地 Cache 联表 |
| Zustand | 用户登录状态与其他轻量化 UI 全局环境 |

## 项目结构 (Core)

```
llm-router-platform/
├── server/                          # Go Backend
│   ├── cmd/server/                  # 主程序入口 
│   ├── internal/
│   │   ├── api/
│   │   │   ├── handlers/            # LLM Proxy REST Handler (OpenAI 兼容转发)
│   │   │   ├── middleware/          # Security (CORS, Backpressure, JWT, RateLimit, Sentinal)
│   │   │   └── routes/             
│   │   ├── graphql/                 # ★ 运营后台入口 (schema / resolvers / dataloaders)
│   │   ├── models/                  # E-R 引擎关系实体
│   │   ├── repository/              # Repository 数据层模式
│   │   └── service/                 # 15个微型逻辑子模块集合
│   │       ├── audit/               # 审计模块
│   │       ├── billing/             # 支付与额度
│   │       ├── cache/               # 重构后的语义缓存 (向量+Redis)
│   │       ├── dlp/                 # 核心数据防泄漏管道
│   │       ├── health/              # 服务监控存活探针
│   │       ├── mcp/                 # Model Context Protocol 子客户端
│   │       ├── org/                 # 多租户管理组织形态
│   │       ├── router/              # 重型策略和权重负载均衡机制
│   │       ├── sso/                 # OIDC 企业联邦身份入口
│   │       └── ...
│   ├── pkg/
│   └── docs/                        # Swagger/OpenAPI
│
├── web/                             # React Frontend
│   ├── src/
│   │   ├── pages/                   # User / Admin Console 拆分的多页面 (SSO, Routing, Org 等20+业务面板)
│   │   ├── components/              
│   │   ├── lib/graphql/             # Apollo Client GQL 操作语句集中管理
│   │   └── stores/                  # Zustand
│   └── package.json
└── docker-compose.yml               # 本地/发布容器化编排环境
```

## 开发计划

- [x] 多 LLM Provider 支持 (OpenAI, Claude, Gemini, DeepSeek 等等)
- [x] 流式响应 (SSE) + Tool Call 桥接
- [x] 管理后台 Apple Design 高端流体设计翻新
- [x] API Key 池化 + 自动故障切换探测
- [x] **语义层缓存 (Semantic Cache) + Embeddings 复用**
- [x] **企业级安全网关 DLP (敏感打码/拦截)**
- [x] **用户多中心联邦身份 (OIDC/SAML, Google/GitHub SSO)**
- [x] **MCP (Model Context Protocol)** 官方子协议接入和 Std/SSE 工具调用联动
- [x] **多租户数据中心级别组织 (Orgs & Workspaces)**
- [x] GraphQL 强类型声明式管理全端重构
- [x] 安全加固 (SSRF 抑制，后端资源竞态修复，SQLI，Gosec/Lint 安全性验证)
- [ ] Kubernetes 针对企业内网的大规模发布 (Helm Chart & Rolling Update)
- [ ] iOS/Android 原生多形态客户端扩展

## 快速指南

### 推荐：直接拉起 Docker Compose

```bash
git clone https://github.com/Veritas-Calculus/llm-router-platform.git
cd llm-router-platform

# 配置后端安全密钥
cp server/.env.example server/.env
# 修改 ENCRYPTION_KEY, JWT_SECRET, 配置相关 Redis 等连接串

# 拉起包含 Postgres, Redis 在内的一整套生态
docker-compose up -d
```
控制台可以通过 `http://localhost` 访问，网关代理的 API 地址在 `http://localhost:8080` (端口映射一致)。

### 手动构建 (研发阶段)

```bash
# Backend (提供 API Endpoint 和 GQL 管理接口)
cd server
go mod download
go run cmd/server/main.go
# Frontend (Web UI)
cd web
npm install
npm run dev
```

### 数据库迁移

如果通过 `release` 模式或生产级别打包，会自动锁定 GORM 隐式建表：
```bash
# 手动控制迁移 (确保生产的 Schema 不被程序污染)
go run cmd/migrate/main.go up
```

## 测试与质量

- 所有的开发合并受到严格自动化 CI 管控：
- Go 后端需要通过完整的 `golangci-lint` 及 `gosec` (Go安全静态审计)。
- Frontend React 前端需保持 0 ERROR / 0 WARNING 强约束的 `eslint (npm run lint)` 规则扫描。

详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

本项目由 MIT License 授权。
