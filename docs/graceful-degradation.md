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
