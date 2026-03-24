# Graceful Degradation Guide

This document describes how the LLM Router Platform behaves when optional dependencies are unavailable.

## Required Dependencies

| Dependency | Impact of Failure |
|---|---|
| **PostgreSQL** | Server refuses to start. All data persistence requires PG. |

## Optional Dependencies

### Redis (Rate Limiting & Caching)

- **Detection**: If `REDIS_HOST` is unreachable at startup, `redisClient` is set to `nil`.
- **Behavior when unavailable**:
  - Rate limiting is **disabled** — all requests pass through without throttling.
  - Dashboard metrics caching falls back to direct DB queries (higher load).
  - Login rate limiter (`LoginLimiter`) is a no-op.
  - Semantic cache lookups are skipped.
- **Recovery**: Automatic on reconnect; no restart required for the Redis client library.
- **Monitoring**: `/healthz` reports Redis status as `error`.

### Langfuse (Observability)

- **Detection**: Controlled by `LANGFUSE_ENABLED=true/false`.
- **Behavior when disabled/unavailable**:
  - Request traces are **silently dropped**; no error propagation to the caller.
  - The `CompositeService` skips Langfuse calls entirely if the client fails to initialize.
- **Recovery**: Set `LANGFUSE_ENABLED=true` with valid keys and restart.

### Sentry (Error Tracking)

- **Detection**: Controlled by `SENTRY_ENABLED=true/false`.
- **Behavior when disabled/unavailable**:
  - Errors are logged via `zap` but not reported to Sentry.
  - No impact on request processing.
- **Recovery**: Set `SENTRY_ENABLED=true` with a valid DSN and restart.

### Stripe (Payments)

- **Detection**: Stripe keys configured via DB or env vars.
- **Behavior when unavailable**:
  - Subscription and payment operations return errors to the caller.
  - Billing tracking (usage logs) continues normally.
  - Free-tier users are unaffected.
- **Recovery**: Correct Stripe API keys and restart.

### SMTP (Email Notifications)

- **Detection**: Controlled by `SMTP_HOST`, `SMTP_PORT`, etc.
- **Behavior when unavailable**:
  - Password reset, email verification, and balance alerts fail silently (logged as errors).
  - User registration and login are unaffected.
- **Recovery**: Correct SMTP configuration; no restart needed for most email libraries.

### Cloudflare Turnstile (CAPTCHA)

- **Detection**: Controlled by `TURNSTILE_ENABLED=true/false`.
- **Behavior when disabled/unavailable**:
  - Registration and login forms skip CAPTCHA verification entirely.
  - Bot protection relies solely on rate limiting.
- **Recovery**: Set `TURNSTILE_ENABLED=true` with valid site key / secret key and restart.

### WeChat Pay

- **Detection**: Controlled by `WECHAT_PAY_ENABLED=true/false`.
- **Behavior when disabled/unavailable**:
  - WeChat Pay payment option is hidden from the billing UI.
  - Other payment methods (Stripe, Alipay) are unaffected.
  - Usage tracking and free-tier billing continue normally.
- **Recovery**: Set `WECHAT_PAY_ENABLED=true` with valid merchant credentials and restart.

### Alipay

- **Detection**: Controlled by `ALIPAY_ENABLED=true/false`.
- **Behavior when disabled/unavailable**:
  - Alipay payment option is hidden from the billing UI.
  - Other payment methods (Stripe, WeChat Pay) are unaffected.
  - Usage tracking and free-tier billing continue normally.
- **Recovery**: Set `ALIPAY_ENABLED=true` with valid app credentials and restart.

### MCP Servers (Tool Integration)

- **Detection**: Controlled by `MCPIntegration` feature gate (default: ON). Individual servers detected via stdio/SSE connectivity.
- **Behavior when unavailable**:
  - If the feature gate is OFF, MCP tool injection is skipped entirely; LLM requests proceed without tools.
  - If a specific MCP server is unreachable, it is marked as `Error` status. Tools from that server are not injected.
  - LLM requests continue normally without the unavailable server's tools.
- **Recovery**: Fix the MCP server process or URL; use the management UI to refresh the server connection.

## Health Check Endpoints

| Endpoint | Purpose | Checks |
|---|---|---|
| `GET /health` | Liveness probe (K8s) | Always returns `200 OK` |
| `GET /healthz` | Deep health check | PostgreSQL, Redis, Migration version |
| `GET /readyz` | Readiness probe (K8s) | PostgreSQL connectivity |

## Recommendations

1. **Always monitor `/healthz`** — it surfaces degraded state before failures cascade.
2. **Set up alerts on Redis disconnection** — rate limiting is your first line of defense against abuse.
3. **Test with optional deps disabled** — run `docker compose up postgres server` (without Redis) to validate degradation behavior.
