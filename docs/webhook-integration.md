# Webhook 集成指南

通过 Webhook，LLM Router 可以在特定事件发生时向外部系统推送 HTTP 回调通知。

## 概述

| 特性 | 说明 |
|------|------|
| 配置粒度 | 按 **Project** |
| 签名验证 | HMAC-SHA256 |
| 重试策略 | 失败自动重试 (指数退避) |
| 投递记录 | 完整记录每次投递的状态、响应码、响应体 |
| Feature Gate | `WebhookNotify` (默认 ON) |

## 创建 Webhook Endpoint

### 通过 GraphQL

```graphql
mutation {
  createWebhookEndpoint(input: {
    projectId: "proj-uuid"
    url: "https://your-service.com/webhook"
    events: ["task.completed", "task.failed", "budget.alert"]
    description: "Production webhook"
  }) {
    id url secret events isActive
  }
}
```

> 创建时返回 `secret`，仅显示一次。请安全保存，用于验证签名。

## 事件类型

| 事件 | 触发条件 |
|------|---------|
| `task.completed` | 异步任务执行完成 |
| `task.failed` | 异步任务执行失败 |
| `budget.alert` | 用量达到预算告警阈值 |
| `budget.exceeded` | 用量超过预算上限 |
| `provider.down` | Provider 健康检查连续失败，被熔断 |
| `provider.recovered` | Provider 从熔断中恢复 |

## 请求格式

```http
POST /webhook HTTP/1.1
Content-Type: application/json
X-Webhook-Signature: sha256=<HMAC_hex>
X-Webhook-Event: task.completed
X-Webhook-Delivery: <delivery_uuid>

{
  "event": "task.completed",
  "timestamp": "2026-03-24T12:00:00Z",
  "data": {
    "taskId": "task-uuid",
    "status": "completed",
    "result": "..."
  }
}
```

## 签名验证

每次投递都包含 `X-Webhook-Signature` 头，值为 `sha256=<hex>`:

```python
import hmac
import hashlib

def verify_signature(payload: bytes, signature: str, secret: str) -> bool:
    expected = hmac.new(
        secret.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", signature)
```

```go
func verifySignature(payload []byte, signature, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

## 重试策略

| 尝试 | 延迟 |
|------|------|
| 第 1 次 | 立即 |
| 第 2 次 | 30 秒 |
| 第 3 次 | 2 分钟 |
| 第 4 次 | 10 分钟 |

共 4 次尝试。每次投递的状态和响应均记录在 `WebhookDelivery` 中，可通过管理后台查看。

## 投递状态

| 状态 | 说明 |
|------|------|
| `pending` | 等待投递 |
| `success` | 目标返回 2xx |
| `failed` | 所有重试均失败 |

## SSRF 防护

Webhook URL 会经过 SSRF 验证，禁止指向私有 IP 地址 (10.x, 172.16-31.x, 192.168.x, 127.x, ::1 等)。
