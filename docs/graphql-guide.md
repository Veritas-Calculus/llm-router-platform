# GraphQL 管理 API 使用指南

所有管理操作 (用户管理、Provider 配置、计费、MCP 等) 通过 GraphQL API 提供。

## 接入信息

| 项目 | 值 |
|------|-----|
| 端点 | `POST /graphql` |
| Playground | `GET /graphql` (需开启 `GraphQLPlayground` Feature Gate) |
| 认证 | `Authorization: Bearer <JWT_TOKEN>` |
| 公开操作 | `login`, `register`, `registrationMode`, `siteConfig` 无需认证 |

---

## 认证流程

### 1. 登录获取 JWT

```graphql
mutation {
  login(input: { email: "admin@example.com", password: "DevAdmin123!" }) {
    token
    refreshToken
    user { id name email role }
  }
}
```

### 2. 携带 Token 请求

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbG..." \
  -d '{"query": "{ me { id name email role } }"}'
```

### 3. 刷新 Token

```graphql
mutation {
  refreshToken {
    token
    refreshToken
  }
}
```

---

## 权限模型

使用 `@auth` 指令控制字段级别权限：

| 指令 | 说明 |
|------|------|
| `@auth` | 需要有效 JWT (任何角色) |
| `@auth(role: ADMIN)` | 仅限 `admin` 角色 |
| `@rateLimit(max: 5, window: "1m")` | 字段级限流 |

---

## 常用查询

### Dashboard 数据

```graphql
query {
  dashboard(projectId: "xxx", channel: "api") {
    totalRequests
    totalTokens
    totalCost
    cacheHitRate
    avgLatency
    activeModels
  }
  usageChart(days: 30, projectId: "xxx") {
    date
    requests
    tokens
    cost
  }
}
```

### 我的 API Keys

```graphql
query {
  myProjects(orgId: "org-uuid") {
    id name
  }
  myApiKeys(projectId: "proj-uuid") {
    id name prefix isActive
    rateLimit tokenLimit
    totalTokensUsed totalRequests
    createdAt lastUsedAt
  }
}
```

### Provider 列表 (Admin)

```graphql
query {
  providers {
    id name type baseUrl
    isActive proxyEnabled
    modelCount
    healthStatus
  }
}
```

---

## 常用 Mutation

### 创建 API Key

```graphql
mutation {
  createApiKey(
    projectId: "proj-uuid"
    name: "Production Key"
    rateLimit: 100
    tokenLimit: 1000000
  ) {
    apiKey { id name prefix }
    secret  # 仅返回一次
  }
}
```

### 管理 Provider (Admin)

```graphql
mutation {
  updateProvider(id: "provider-uuid", input: {
    name: "OpenAI"
    baseUrl: "https://api.openai.com/v1"
    isActive: true
    proxyEnabled: false
  }) { id name isActive }
}

mutation {
  syncProviderModels(providerId: "provider-uuid") {
    id name isActive
  }
}
```

### Feature Gate 开关 (Admin)

```graphql
query {
  systemSettings {
    featureGates {
      name enabled category description source
    }
  }
}

mutation {
  updateSystemSettings(input: {
    featureGates: [
      { name: "SemanticCache", enabled: true }
      { name: "GraphQLPlayground", enabled: false }
    ]
  }) {
    featureGates { name enabled }
  }
}
```

### MCP Server 管理 (Admin)

```graphql
mutation {
  createMcpServer(input: {
    name: "Google Search"
    transport: "stdio"
    command: "npx"
    args: "-y,@modelcontextprotocol/server-google-search"
  }) { id name status tools { name description } }
}
```

---

## Schema 概览

| 域 | Query | Mutation | 角色 |
|----|-------|----------|------|
| 认证 | — | `login`, `register`, `refreshToken`, `logout`, `forgotPassword` 等 | Public/User |
| 个人数据 | `me`, `myOrganizations`, `myApiKeys`, `myUsageSummary` 等 | `createApiKey`, `updateProfile`, `changePassword` 等 | User |
| Dashboard | `dashboard`, `usageChart`, `providerStats`, `modelStats` | — | User |
| 组织管理 | `organizationMembers`, `identityProviders` | `addOrganizationMember`, `createIdentityProvider` 等 | User |
| MFA | — | `generateMfaSecret`, `verifyAndEnableMfa`, `disableMfa` | User |
| Admin: Users | `users`, `user`, `userUsage` | `toggleUser`, `updateUserRole`, `updateUserQuota` | Admin |
| Admin: Providers | `providers`, `models`, `providerHealth` | `updateProvider`, `syncProviderModels`, CRUD API Keys | Admin |
| Admin: Proxies | `proxies` | `createProxy`, `testProxy`, `testAllProxies` 等 | Admin |
| Admin: Health | `healthApiKeys`, `healthProxies`, `healthProviders` | `checkApiKeyHealth`, `checkAllProviderHealth` 等 | Admin |
| Admin: MCP | `mcpServers`, `mcpTools` | `createMcpServer`, `refreshMcpTools` 等 | Admin |
| Admin: Routing | `routingRules` | `createRoutingRule`, `updateRoutingRule` 等 | Admin |
| Admin: Prompts | `promptTemplates`, `promptVersions` | CRUD + `setActivePromptVersion` | Admin |
| Admin: Settings | `systemSettings`, `systemStatus` | `updateSystemSettings`, `sendTestEmail` | Admin |
| Admin: FinOps | `adminDashboard`, `adminRevenueChart` | `exportSystemUsageCsv` | Admin |
| Admin: Announcements | `announcements` | CRUD | Admin |
| Admin: Coupons | `coupons` | CRUD | Admin |
| Admin: Documents | `documents` | CRUD | Admin |

完整 Schema 定义位于 `server/internal/graphql/schema/*.graphqls`。

---

## 安全特性

| 特性 | 配置 |
|------|------|
| 查询深度限制 | 7 层 |
| 查询复杂度限制 | 200 |
| Introspection | 默认关闭 (Feature Gate: `GraphQLIntrospection`) |
| 错误消息 | Release 模式下自动脱敏 |
| Dataloaders | N+1 自动优化 |

---

## 前端集成 (Apollo Client)

前端 GraphQL 操作集中管理在 `web/src/lib/graphql/operations/` 目录下，按域名组织：

```
operations/
├── auth.ts          # login, register, refreshToken
├── apikeys.ts       # CRUD API Keys
├── dashboard.ts     # dashboard, usageChart
├── providers.ts     # providers, models, health
├── featuregates.ts  # systemSettings queries
└── ...
```

Apollo Client 配置位于 `web/src/lib/graphql/client.ts`。
