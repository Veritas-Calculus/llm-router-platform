# 系统架构

## 总体架构

```mermaid
graph TB
    subgraph Clients
        SDK["OpenAI SDK / cURL"]
        WebUI["React Frontend<br/>(Apollo Client)"]
    end

    subgraph Gateway["VC LLM Router (Go)"]
        direction TB
        REST["/v1/* REST API<br/>(OpenAI 兼容)"]
        GQL["/graphql<br/>(管理 API)"]
        MW["Middleware Chain<br/>JWT · APIKey · CORS<br/>RateLimit · Backpressure"]
    end

    subgraph Services["Service Layer (26 模块)"]
        direction TB
        RouterSvc["Router<br/>策略负载均衡"]
        BillingSvc["Billing<br/>计费 · 预算 · 支付"]
        ProviderSvc["Provider<br/>注册 · 模型同步"]
        CacheSvc["Semantic Cache<br/>向量缓存"]
        DLPSvc["DLP<br/>数据防泄漏"]
        MCPsvc["MCP<br/>Tool Integration"]
        HealthSvc["Health<br/>探针 · 告警"]
        MemorySvc["Memory<br/>对话记忆"]
        AuditSvc["Audit<br/>安全审计"]
        UserSvc["User<br/>认证 · RBAC"]
    end

    subgraph Providers["LLM Providers"]
        OpenAI["OpenAI"]
        Claude["Anthropic Claude"]
        Gemini["Google Gemini"]
        DeepSeek["DeepSeek"]
        Local["Ollama / vLLM<br/>LM Studio"]
    end

    subgraph Infra["Infrastructure"]
        PG[("PostgreSQL 16")]
        Redis[("Redis 7")]
        ProxyPool["Proxy Pool<br/>(HTTP/SOCKS5)"]
    end

    subgraph Observability
        Langfuse["Langfuse"]
        Sentry["Sentry"]
        Prom["Prometheus"]
    end

    SDK -->|API Key| REST
    WebUI -->|JWT| GQL
    REST --> MW --> RouterSvc
    GQL --> MW --> UserSvc

    RouterSvc --> ProviderSvc
    RouterSvc --> CacheSvc
    RouterSvc --> DLPSvc
    RouterSvc --> MCPsvc
    RouterSvc --> BillingSvc
    RouterSvc --> MemorySvc

    ProviderSvc -->|API 转发| OpenAI
    ProviderSvc --> Claude
    ProviderSvc --> Gemini
    ProviderSvc --> DeepSeek
    ProviderSvc --> Local
    ProviderSvc -.->|可选| ProxyPool

    UserSvc --> PG
    BillingSvc --> PG
    AuditSvc --> PG
    CacheSvc --> Redis
    HealthSvc --> Redis

    RouterSvc -.-> Langfuse
    MW -.-> Sentry
    MW -.-> Prom
```

## 请求生命周期

```mermaid
sequenceDiagram
    participant Client as OpenAI SDK
    participant MW as Middleware
    participant Router as Router Service
    participant DLP as DLP Engine
    participant Cache as Semantic Cache
    participant MCP as MCP Service
    participant Provider as LLM Provider
    participant Billing as Billing

    Client->>MW: POST /v1/chat/completions
    MW->>MW: APIKey 认证
    MW->>MW: Rate Limit 检查
    MW->>MW: Quota 检查
    MW->>MW: Backpressure 检查

    MW->>Router: ChatCompletion()
    Router->>DLP: 入站内容扫描 (Block/Mask)

    alt DLP 拦截
        DLP-->>Client: 403 Content Blocked
    end

    Router->>Cache: 缓存查找 (精确 + 向量)
    alt 缓存命中
        Cache-->>Client: 200 Cached Response
        Router->>Billing: 记录缓存用量 (折扣)
    end

    Router->>MCP: 注入 MCP Tools
    Router->>Router: 策略选择 Provider + Key
    Router->>Provider: 转发请求

    alt Tool Call 循环
        Provider-->>Router: tool_call 响应
        Router->>MCP: 执行 Tool
        MCP-->>Router: Tool 结果
        Router->>Provider: 携带结果重新请求
    end

    Provider-->>Router: 最终响应
    Router->>DLP: 出站内容扫描
    Router->>Billing: 记录 Token 用量
    Router-->>Client: 200 Response (SSE 或 JSON)
```

## 数据模型概览

```mermaid
erDiagram
    User ||--o{ Organization : "belongs to"
    Organization ||--o{ Project : "contains"
    Project ||--o{ ApiKey : "has"
    ApiKey ||--o{ UsageLog : "generates"
    Organization ||--o{ OrganizationMember : "has"
    User ||--o{ OrganizationMember : "is"

    Provider ||--o{ ProviderApiKey : "has"
    Provider ||--o{ Model : "serves"
    Model ||--o{ UsageLog : "used by"

    User {
        uuid id PK
        string email
        string role
        bool is_active
        bool mfa_enabled
    }
    Organization {
        uuid id PK
        string name
        string slug
    }
    Project {
        uuid id PK
        string name
        uuid org_id FK
    }
    ApiKey {
        uuid id PK
        string prefix
        string hashed_key
        int rate_limit
        int token_limit
    }
    Provider {
        uuid id PK
        string name
        string type
        string base_url
        bool is_active
    }
    Model {
        uuid id PK
        string model_id
        float input_price
        float output_price
    }
    UsageLog {
        uuid id PK
        int prompt_tokens
        int completion_tokens
        float cost
        string channel
    }
```

## 分层架构

```
┌─────────────────────────────────────────────────────────┐
│                    Transport Layer                        │
│  /v1/* (REST, OpenAI-Compatible)  │  /graphql (GQL)      │
├───────────────────────────────────┼──────────────────────┤
│             Middleware Chain                               │
│  RequestID → Metrics → Security → CORS → Logging → Panic │
├───────────────────────────────────────────────────────────┤
│                    Handler Layer                           │
│  REST Handlers (LLM)  │  GraphQL Resolvers (Management)   │
├───────────────────────────────────────────────────────────┤
│                    Service Layer (26 modules)              │
│  router · billing · provider · cache · dlp · mcp · ...    │
├───────────────────────────────────────────────────────────┤
│                    Repository Layer                        │
│  GORM-based data access with interface abstraction         │
├───────────────────────────────────────────────────────────┤
│                    Infrastructure                          │
│  PostgreSQL 16  │  Redis 7  │  External APIs               │
└─────────────────────────────────────────────────────────┘
```

## 弹性机制

| 机制 | 说明 |
|------|------|
| **Circuit Breaking** | Provider 连续 5 次 5xx/超时 → 自动熔断剔除 |
| **API Key 轮转** | 429/Quota 错误 → 自动切换备用 Key 重试 |
| **背压保护** | DB 连接池 ≥80% → 返回 503 拒绝新请求 |
| **Redis 降级** | Redis 不可用 → Rate Limit 禁用，Cache 跳过 |
| **Context 取消** | 流式请求客户端断开 → 后台 goroutine 严格取消 |
| **预录计费** | 请求发起即记录 → 断连也能审计部分消耗 |
