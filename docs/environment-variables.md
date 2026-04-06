# 环境变量参考

> 所有变量通过 `server/.env` 文件或容器环境变量配置。示例文件: `server/.env.example`

## 目录

- [Server](#server)
- [Database](#database)
- [Redis](#redis)
- [Security & Encryption](#security--encryption)
- [JWT & Auth](#jwt--auth)
- [Admin](#admin)
- [Registration](#registration)
- [Rate Limiting](#rate-limiting)
- [Health Check](#health-check)
- [Email](#email)
- [Alerts](#alerts)
- [Payments](#payments)
- [CAPTCHA](#captcha)
- [OAuth2 / SSO](#oauth2--sso)
- [Observability](#observability)
- [Data Retention](#data-retention)
- [Feature Gates](#feature-gates)

---

## Server

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8080` | HTTP 监听端口 |
| `GIN_MODE` | `release` | Gin 运行模式 (`debug` / `release`) |
| `CORS_ORIGINS` | _(空)_ | 允许的 CORS 源，逗号分隔。空=禁止跨域，`*`=全部允许 |
| `SERVER_READ_TIMEOUT_SECONDS` | `30` | HTTP 读超时 |
| `SERVER_WRITE_TIMEOUT_SECONDS` | `600` | HTTP 写超时 (需大于 LLM 流式最长回复) |
| `ALLOW_LOCAL_PROVIDERS` | `false` | 允许 Provider URL 指向私有 IP (开发环境可设为 true) |
| `FRONTEND_URL` | `http://localhost:5173` | 前端地址 (用于邮件中的链接等) |

## Logging

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LOG_LEVEL` | `info` | 日志级别 (`debug` / `info` / `warn` / `error`) |
| `LOG_FORMAT` | `json` | 日志格式 (`json` / `text`) |
| `LOKI_URL` | _(空)_ | Loki 推送地址 (如 `http://loki:3100`) |

## Proxy Pool

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PROXY_POOL_ENABLED` | `false` | 启用代理池 |
| `PROXY_POOL_URL` | _(空)_ | 代理池获取 URL |

## Database

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_HOST` | `localhost` | PostgreSQL 地址 |
| `DB_PORT` | `5432` | PostgreSQL 端口 |
| `DB_USER` | — | 数据库用户 |
| `DB_PASSWORD` | — | 数据库密码 |
| `DB_NAME` | — | 数据库名 |
| `DB_SSL_MODE` | `require` | SSL 模式 (`disable` / `prefer` / `require` / `verify-full`) |
| `DB_MAX_OPEN_CONNS` | `100` | 最大打开连接数 |
| `DB_MAX_IDLE_CONNS` | `10` | 最大空闲连接数 |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `60` | 连接最大生存时间 (分钟) |

## Redis

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_HOST` | `localhost` | Redis 地址 |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `REDIS_PASSWORD` | — | Redis 密码 |
| `REDIS_DB` | `0` | Redis 数据库编号 |
| `REDIS_TLS_ENABLED` | `false` | 启用 Redis TLS 连接 |

> Redis 为可选依赖。不可用时 Rate Limit 禁用、语义缓存跳过。详见 [graceful-degradation.md](graceful-degradation.md)。

## Security & Encryption

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ENCRYPTION_KEY` | — | **必填**。32 字节 AES-256 密钥，用于加密 Provider API Key |
| `VAULT_ADDR` | _(空)_ | HashiCorp Vault 地址 (可选替代本地 AES) |
| `VAULT_TOKEN` | _(空)_ | Vault 认证 Token |
| `VAULT_TRANSIT_KEY` | _(空)_ | Vault Transit Engine 密钥名 |
| `ADMIN_IP_WHITELIST` | _(空)_ | Admin API 的 IP 白名单 (逗号分隔 CIDR) |

## JWT & Auth

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JWT_SECRET` | — | **必填**。JWT 签名密钥 (≥32 字符) |
| `JWT_EXPIRES_IN` | `1h` | Access Token 有效期 |
| `JWT_REFRESH_EXPIRES_IN` | `168h` | Refresh Token 有效期 (默认 7 天) |

## Admin

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ADMIN_EMAIL` | — | 初始管理员邮箱 (首次启动自动创建) |
| `ADMIN_PASSWORD` | — | 初始管理员密码 |
| `ADMIN_NAME` | `Administrator` | 初始管理员名称 |

## Registration

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REGISTRATION_MODE` | `open` | 注册模式: `open` / `invite` / `closed` |
| `INVITE_CODE` | _(空)_ | 当 mode=`invite` 时必填 |

## Rate Limiting

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `RATE_LIMIT_ENABLED` | `true` | 启用全局限流 |
| `RATE_LIMIT_REQUESTS_PER_MINUTE` | `60` | 每分钟最大请求数 |

## Health Check

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HEALTH_CHECK_ENABLED` | `true` | 启用 Provider 健康探测 |
| `HEALTH_CHECK_INTERVAL` | `60` | 探测间隔 (秒) |
| `HEALTH_CHECK_TIMEOUT` | `10` | 探测超时 (秒) |
| `HEALTH_CHECK_RETRY_COUNT` | `3` | 失败恢复重试次数 |
| `HEALTH_CHECK_FAILURE_THRESHOLD` | `3` | 连续失败次数触发熔断 |

## Email

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `EMAIL_ENABLED` | `false` | 启用事务邮件 |
| `EMAIL_SMTP_HOST` | — | SMTP 服务器 |
| `EMAIL_SMTP_PORT` | `587` | SMTP 端口 |
| `EMAIL_SMTP_USER` | — | SMTP 用户 |
| `EMAIL_SMTP_PASS` | — | SMTP 密码 |
| `EMAIL_FROM` | — | 发件人地址 |
| `EMAIL_FROM_NAME` | `LLM Router` | 发件人名称 |
| `EMAIL_SMTP_TLS` | `true` | 强制 TLS |

## Alerts

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ALERT_ENABLED` | `true` | 启用告警 |
| `ALERT_WEBHOOK_URL` | — | Webhook 告警地址 |
| `ALERT_EMAIL_ENABLED` | `false` | 启用邮件告警 |

## Payments

### Stripe

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `STRIPE_ENABLED` | `false` | 启用 Stripe |
| `STRIPE_SECRET_KEY` | — | Stripe Secret Key |
| `STRIPE_PUBLISHABLE_KEY` | — | Stripe Publishable Key |
| `STRIPE_WEBHOOK_SECRET` | — | Stripe Webhook 签名密钥 |

### WeChat Pay

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `WECHAT_PAY_ENABLED` | `false` | 启用微信支付 |
| `WECHAT_PAY_APP_ID` | — | 微信应用 App ID |
| `WECHAT_PAY_MCH_ID` | — | 商户号 |
| `WECHAT_PAY_API_V3_KEY` | — | API v3 密钥 |
| `WECHAT_PAY_SERIAL_NO` | — | 商户证书序列号 |
| `WECHAT_PAY_PRIVATE_KEY` | — | 商户私钥 (PEM) |
| `WECHAT_PAY_NOTIFY_URL` | — | 异步通知回调地址 |
| `WECHAT_PAY_PLATFORM_CERT` | — | 微信支付平台证书 PEM (用于验证回调签名) |

### Alipay

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ALIPAY_ENABLED` | `false` | 启用支付宝 |
| `ALIPAY_APP_ID` | — | 支付宝应用 ID |
| `ALIPAY_PRIVATE_KEY` | — | 应用私钥 (PEM) |
| `ALIPAY_PUBLIC_KEY` | — | 支付宝公钥 |
| `ALIPAY_NOTIFY_URL` | — | 异步通知回调地址 |
| `ALIPAY_SANDBOX` | `false` | 沙箱测试模式 |

## CAPTCHA

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TURNSTILE_ENABLED` | `false` | 启用 Cloudflare Turnstile |
| `TURNSTILE_SITE_KEY` | — | 前端 Site Key |
| `TURNSTILE_SECRET_KEY` | — | 后端 Secret Key |

## OAuth2 / SSO

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GITHUB_CLIENT_ID` | — | GitHub OAuth2 Client ID |
| `GITHUB_CLIENT_SECRET` | — | GitHub OAuth2 Client Secret |
| `GOOGLE_CLIENT_ID` | — | Google OAuth2 Client ID |
| `GOOGLE_CLIENT_SECRET` | — | Google OAuth2 Client Secret |

> 更多企业 SSO (OIDC/SAML) 通过管理后台 Identity Providers 配置。

## Observability

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LANGFUSE_ENABLED` | `false` | 启用 Langfuse 追踪 |
| `LANGFUSE_PUBLIC_KEY` | — | Langfuse Public Key |
| `LANGFUSE_SECRET_KEY` | — | Langfuse Secret Key |
| `LANGFUSE_HOST` | `https://cloud.langfuse.com` | Langfuse 服务地址 |
| `SENTRY_ENABLED` | `false` | 启用 Sentry 错误追踪 |
| `SENTRY_DSN` | — | Sentry DSN |
| `SENTRY_ENVIRONMENT` | `production` | Sentry 环境标签 |
| `SENTRY_SAMPLE_RATE` | `1.0` | Sentry 采样率 (0.0-1.0) |
| `METRICS_ALLOW_UNAUTHENTICATED` | `false` | 允许无认证访问 Prometheus 指标端点 |
| `OTEL_ENABLED` | `false` | 启用 OpenTelemetry 分布式追踪 |
| `OTEL_ENDPOINT` | _(空)_ | OTLP Exporter 地址 |
| `OTEL_SERVICE_NAME` | `llm-router-platform` | 服务名 |

## Cache

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CACHE_HIT_COST_RATIO` | `0.1` | 缓存命中时的成本比例 (0.0-1.0) |

## Data Retention

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLEANUP_HEALTH_RETENTION_DAYS` | `30` | 健康检查记录保留天数 |
| `CLEANUP_ALERT_RETENTION_DAYS` | `90` | 已解决告警保留天数 |
| `CLEANUP_AUDIT_RETENTION_DAYS` | `90` | 审计日志保留天数 |

## Feature Gates

Feature Gates 通过管理后台 (**Settings → Feature Gates**) 动态管理，存储在数据库中。代码默认值如下:

| Gate | 类别 | 默认 | 说明 |
|------|------|:----:|------|
| `GraphQLIntrospection` | Security | OFF | GraphQL Schema Introspection |
| `GraphQLPlayground` | Security | OFF | GraphQL 交互式 Playground |
| `SwaggerDocs` | Security | OFF | Swagger/OpenAPI 文档端点 |
| `PprofDebug` | Security | OFF | Go pprof 性能剖析端点 |
| `AutoMigrate` | Security | OFF | GORM 自动迁移 (生产禁用) |
| `SemanticCache` | Feature | ON | 语义响应缓存 |
| `ConversationMemory` | Feature | ON | 服务端对话记忆 |
| `PromptSafety` | Feature | ON | Prompt 注入检测 |
| `MCPIntegration` | Feature | ON | MCP 工具集成 |
| `WebhookNotify` | Feature | ON | Webhook 事件通知 |
| `MetricsUnauthenticated` | Observability | OFF | 无认证 Prometheus 端点 |
| `OTelTracing` | Observability | OFF | OpenTelemetry 分布式追踪 |

> 安全类 Gate 默认关闭 (opt-in)，功能类默认开启，可观测性类需外部基础设施支持故默认关闭。
