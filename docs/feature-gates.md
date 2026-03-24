# Feature Gate 操作手册

Feature Gate 系统提供运行时功能开关，无需重启服务即可启用/禁用功能。

## 架构

```
Code Defaults → DB Override (system_configs) → Runtime State
```

优先级: **DB 值 > 代码默认值**。通过管理后台修改的值持久化到 `system_configs` 表。

## 管理方式

### 管理后台 UI

**Admin → Settings → Feature Gates** — 可视化开关列表，显示当前值、类别、来源 (default / database)。

### GraphQL API

```graphql
# 查询所有 Gate
query {
  systemSettings {
    featureGates {
      name
      enabled
      category     # security / feature / observability
      description
      source       # "default" or "database"
    }
  }
}

# 批量更新
mutation {
  updateSystemSettings(input: {
    featureGates: [
      { name: "GraphQLPlayground", enabled: true }
      { name: "MetricsUnauthenticated", enabled: true }
    ]
  }) {
    featureGates { name enabled source }
  }
}
```

## Gate 列表

### Security Gates (默认: OFF)

| Gate | 说明 | 风险提示 |
|------|------|---------|
| `GraphQLIntrospection` | 开放 `__schema` / `__type` 查询 | ⚠️ 暴露 API 结构 |
| `GraphQLPlayground` | 开放 `GET /graphql` 交互式界面 | ⚠️ 仅限开发 |
| `SwaggerDocs` | 开放 `/swagger/*` API 文档 | ⚠️ 暴露端点清单 |
| `PprofDebug` | 开放 `/debug/pprof/*` 性能剖析 | ⚠️ 性能影响 + 信息泄露 |
| `AutoMigrate` | 启动时 GORM 自动建表 | ⚠️ 生产禁用 |

### Feature Gates (默认: ON)

| Gate | 说明 | 关闭影响 |
|------|------|---------|
| `SemanticCache` | 语义响应缓存 | 所有请求直达 Provider，延迟增加 |
| `ConversationMemory` | 服务端对话记忆 | 无法跨请求维护上下文 |
| `PromptSafety` | Prompt 注入检测 | 跳过安全检查 |
| `MCPIntegration` | MCP 工具自动注入 | 不注入外部工具 |
| `WebhookNotify` | Webhook 事件通知 | 停止推送回调 |

### Observability Gates (默认: OFF)

| Gate | 说明 | 前提 |
|------|------|------|
| `MetricsUnauthenticated` | `/internal/metrics` 无认证端点 | Prometheus 抓取需要 |
| `OTelTracing` | OpenTelemetry 分布式追踪 | 需配置 `OTEL_ENDPOINT` |

## DB 存储格式

Feature Gate 值存储在 `system_configs` 表中:

| key | value | category |
|-----|-------|----------|
| `fg.graphql_introspection` | `true` / `false` | `featuregate` |
| `fg.semantic_cache` | `true` / `false` | `featuregate` |

PascalCase 字段名自动转为 `fg.snake_case` 格式。

## 默认策略

- **安全类**: 默认 OFF (opt-in)，需明确开启才暴露调试/诊断接口
- **功能类**: 默认 ON (核心能力)，按需关闭以禁用非必要功能
- **可观测类**: 默认 OFF，需外部基础设施 (Prometheus/OTel) 才有意义
