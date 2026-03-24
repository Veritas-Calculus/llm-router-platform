# DLP (数据防泄漏) 配置指南

DLP 引擎对 LLM 请求和响应进行双向内容扫描，自动检测并处理敏感信息。

## 概述

| 特性 | 说明 |
|------|------|
| **扫描方向** | 双向 (入站请求 + 出站响应) |
| **配置粒度** | 按 Project 独立配置 |
| **策略模式** | `BLOCK` (拦截) / `REDACT` (脱敏替换) / `AUDIT` (仅记录) |
| **内置检测** | 邮箱、手机号、信用卡号、SSN、API Key |
| **自定义规则** | 支持自定义正则表达式 |
| **Feature Gate** | 无独立 Gate，按 Project 粒度启用 |

## 配置

### 通过管理后台

**Organization → Project → Settings → DLP** 面板可视化配置。

### 通过 GraphQL

```graphql
mutation {
  updateProject(id: "project-uuid", input: {
    dlpConfig: {
      isEnabled: true
      strategy: "REDACT"
      maskEmails: true
      maskPhones: true
      maskCreditCards: true
      maskSsn: true
      maskApiKeys: true
      customRegex: ["\\b(SECRET|TOKEN)_[A-Z0-9]{16,}\\b"]
    }
  }) {
    id name
  }
}
```

## 策略模式

| 模式 | 行为 |
|------|------|
| `BLOCK` | 检测到敏感信息时立即拒绝请求 (403) |
| `REDACT` | 将敏感信息替换为掩码 (e.g., `user@example.com` → `[EMAIL_REDACTED]`) 然后继续处理 |
| `AUDIT` | 记录检测结果到审计日志，但不干预请求 |

## 内置检测器

| 检测器 | 字段 | 匹配示例 |
|--------|------|----------|
| Email | `maskEmails` | `user@domain.com` → `[EMAIL_REDACTED]` |
| Phone | `maskPhones` | `+1-555-123-4567` → `[PHONE_REDACTED]` |
| Credit Card | `maskCreditCards` | `4111-1111-1111-1111` → `[CC_REDACTED]` |
| SSN | `maskSsn` | `123-45-6789` → `[SSN_REDACTED]` |
| API Key | `maskApiKeys` | `sk-xxxxx...` → `[KEY_REDACTED]` |

## 自定义正则

`customRegex` 字段接受 JSON 字符串数组，每个元素是一个 Go 正则表达式:

```json
[
  "\\bAWS[A-Z0-9]{16,}\\b",
  "\\bpassword\\s*[:=]\\s*\\S+",
  "\\b\\d{3}-\\d{2}-\\d{4}\\b"
]
```

匹配到的内容会被替换为 `[CUSTOM_REDACTED]`。

## 降级行为

- DLP 扫描引擎内部错误时默认 **fail-open** (放行请求，记录错误日志)
- 可通过策略模式 `BLOCK` 实现 fail-closed 行为
