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

服务端只在创建响应中返回明文 secret。数据库中保存的是带 `enc:v1:` 版本前缀的 AES-256-GCM 密文；历史明文或无前缀密文会在 endpoint 下次更新时自动归一化为当前密文格式。

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
User-Agent: LLM-Router-Platform/Webhook
Webhook-Signature: t=<unix_timestamp>,v1=<HMAC_hex>
Webhook-Timestamp: <unix_timestamp>
X-Hub-Signature-256: sha256=<legacy_HMAC_hex>
X-VC-Event: task.completed
X-VC-Delivery: <delivery_uuid>

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

每次投递都包含推荐使用的 `Webhook-Signature` 头，格式为：

```text
t=<unix_timestamp>,v1=<hex_hmac_sha256(timestamp + "." + raw_body)>
```

接收方应：

1. 解析 `t` 与 `v1`。
2. 拒绝超出本地容忍窗口的时间戳，例如 5 分钟。
3. 用 endpoint secret 对 `t + "." + raw_body` 计算 HMAC-SHA256。
4. 使用 constant-time compare 比较 `v1`。

`X-Hub-Signature-256: sha256=<hex>` 仍会发送给旧消费者兼容，但新集成应使用带时间戳的 `Webhook-Signature`。

```python
import hmac
import hashlib
import time

def verify_signature(payload: bytes, signature: str, secret: str, tolerance_seconds: int = 300) -> bool:
    parts = dict(part.split("=", 1) for part in signature.split(",") if "=" in part)
    timestamp = int(parts.get("t", "0"))
    received = parts.get("v1", "")
    if abs(time.time() - timestamp) > tolerance_seconds:
        return False
    signed_payload = f"{timestamp}.".encode() + payload
    expected = hmac.new(secret.encode(), signed_payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, received)
```

```go
func verifySignature(payload []byte, signature, secret string) bool {
    parts := map[string]string{}
    for _, part := range strings.Split(signature, ",") {
        kv := strings.SplitN(part, "=", 2)
        if len(kv) == 2 {
            parts[kv[0]] = kv[1]
        }
    }
    ts, err := strconv.ParseInt(parts["t"], 10, 64)
    if err != nil || time.Since(time.Unix(ts, 0)) > 5*time.Minute {
        return false
    }
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(parts["t"] + "."))
    mac.Write(payload)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(parts["v1"]))
}
```

## 重试策略

| 阶段 | 行为 |
|------|------|
| 首次投递 | 新 delivery 入队后由后台 dispatcher 立即捞取 |
| 失败后重试 | 指数退避，约 1 / 2 / 4 / 8 分钟，带 ±25% jitter |
| `Retry-After` | 若目标返回合法 `Retry-After`，优先按该值调度，最大 1 小时 |
| 最大次数 | 5 次投递尝试后标记为 `failed` |

后台 dispatcher 每 5 秒扫描一次到期的 pending delivery；真实重试节奏由 `next_attempt_at` 控制，不会每 5 秒暴力重试同一失败 endpoint。每次投递的状态和响应均记录在 `WebhookDelivery` 中，可通过管理后台查看。

## 投递状态

| 状态 | 说明 |
|------|------|
| `pending` | 等待投递 |
| `success` | 目标返回 2xx |
| `failed` | 所有重试均失败 |

## SSRF 防护

Webhook URL 会经过 SSRF 验证，禁止指向私有 IP 地址 (10.x, 172.16-31.x, 192.168.x, 127.x, ::1 等)。
