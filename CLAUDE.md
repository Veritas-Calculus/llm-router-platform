# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository overview

Monorepo for a unified LLM gateway. Two top-level apps:

- `server/` — Go 1.24 backend. Dual API surface: OpenAI-compatible REST proxy at `/v1/*` (LLM traffic) and a gqlgen GraphQL endpoint at `/graphql` (all admin/management). Any legacy REST management routes under `/api/v1/` are deprecated — do not add new ones there.
- `web/` — React 19 + TypeScript + Vite + Apollo Client admin console. Talks to the backend exclusively via GraphQL.

Supporting infra lives in `deploy/` (Helm, Grafana, Caddy/Nginx TLS), `docker/`, `docker-compose.yml`, `tests/` (k6 load tests), and `docs/`.

## Common commands

All `make` targets run from the repo root; the underlying `go`/`npm` commands run from `server/` or `web/`.

| Task | Command |
|------|---------|
| One-time dev bootstrap (env file, deps, infra, migrate+seed) | `make setup` |
| Run backend + frontend together | `make dev` |
| Start just Postgres + Redis | `docker-compose up -d postgres redis` |
| Run backend only | `cd server && go run ./cmd/server` |
| Build backend binary (with version ldflags) | `make build` |
| Backend tests | `cd server && go test ./...` |
| Single Go test | `cd server && go test ./internal/service/router -run TestWeightedPick` |
| Backend lint (must pass — CI gate) | `cd server && golangci-lint run ./...` |
| Frontend dev server | `cd web && npm run dev` |
| Frontend build (typecheck + vite) | `cd web && npm run build` |
| Frontend tests (vitest) | `cd web && npm test` |
| Frontend lint (0 warnings enforced) | `cd web && npm run lint` |
| SQL migrations up/down/status | `make migrate-up` / `make migrate-down` / `make migrate-status` |
| Verify GORM ↔ SQL migration drift | `make check-schema` (requires Docker) |
| Helm chart lint | `make helm-lint` |

`cd web && npm run lint` runs with `--max-warnings 0` — warnings fail CI.

## Architecture

### Two distinct API surfaces — do not conflate them

1. **LLM proxy (REST, OpenAI-compatible)** under `/v1/*`. Lives in `server/internal/api/handlers/`. These handlers forward traffic to upstream providers and MUST stay OpenAI-wire-compatible. Layering: `Handler → Service → Repository → Model`.
2. **Management API (GraphQL, schema-first via gqlgen)** under `/graphql`. Schema files in `server/internal/graphql/schema/*.graphqls`; resolvers in `server/internal/graphql/resolvers/`; generated code in `server/internal/graphql/generated/` (do not edit). Layering: `Resolver → Service → Repository → Model`. The same service layer is reused by both surfaces.

When adding a management feature, add/modify a `.graphqls` schema file, regenerate with gqlgen, and wire the resolver to an existing (or new) service — do not add a REST handler for it.

### GraphQL specifics

- Auth is enforced at the field level via the `@auth(role: ROLE)` directive (see `server/internal/graphql/directives/`), not inside resolvers.
- Rate limiting is enforced at the field level via `@rateLimit(max: N, window: "duration")`.
- Dataloaders in `server/internal/graphql/dataloaders/` solve N+1 — use them for any new parent→children resolver.
- Production hardening: query depth limit 7, complexity limit 200, introspection disabled in release mode, errors sanitized in release mode. Do not loosen these without explicit reason.

### Service layer (`server/internal/service/`)

Business logic is split into ~25 small packages, each owning one bounded concern: `router` (routing strategies: round-robin / weighted / lowest-latency / cost), `provider` (registration + model sync), `cache` (semantic cache via Redis + embeddings), `dlp` (bidirectional content inspection: block / mask / redact), `billing` (token accounting, budgets, Stripe / WeChat Pay / Alipay), `health` (provider liveness probes and auto-eviction), `mcp` (Model Context Protocol stdio + SSE clients), `safety` (prompt injection detection), `webhook`, `audit`, plus `memory`, `notification`, `user`, `config` (runtime feature gates), and others. Prefer extending an existing service over creating a new one.

### Resilience and billing invariants — load-bearing, do not break

- **Circuit breaking**: providers are skipped after 5 consecutive 5xx/timeout failures. Logic lives in the router/health services.
- **API key rotation**: on 429 or quota errors, the request is retried against an alternate key from the pool before surfacing the failure.
- **Pre-recorded usage**: every LLM request (including streams) is written to the DB *before* the upstream call, so audit/billing survives mid-stream disconnects. Streaming chunks update the row incrementally. Any new proxy handler must preserve this write-ahead pattern.
- **Context-aware streaming**: SSE goroutines must respect `ctx.Done()` — goroutine leaks here are a known failure mode.

### Security invariants — do not loosen

- Provider API keys are AES-256-GCM encrypted at rest using `ENCRYPTION_KEY`. Always go through `crypto.Encrypt` / `crypto.Decrypt` — never persist raw keys.
- **SSRF**: every outbound `http.Client` whose URL is influenced by user, tenant, or admin input must be constructed via `sanitize.SafeHTTPClient(allowLocal, timeout)` (or `SafeHTTPClientWithProxy` for proxy-backed requests). A bare `&http.Client{}` in router/webhook/notification/health/mcp/oauth/sso code is a security regression. URL-level validation via `sanitize.ValidateWebhookURL` only happens at write time; the dial-time dialer is what blocks DNS rebinding.
- **OAuth / SSO identity linking**: never auto-link a federated identity (GitHub, Google, OIDC) to a pre-existing account that has a password hash. Linking must be user-initiated from an already-authenticated session.
- **OIDC**: id_token must be verified against the IdP's JWKS with RS256. `iss`, `aud`, `exp`, and `nonce` are all mandatory checks. PKCE is required; never drop `code_verifier`/`code_challenge` from the flow. State and nonce live in a single-use HttpOnly+Secure cookie.
- **OAuth / SSO callback URL** is built from `cfg.Frontend.PublicBackendURL` (fallback `cfg.Frontend.URL`). Never read `Host` or `X-Forwarded-Proto` from the request to compose `redirect_uri`.
- **Refresh token rotation**: `RotateRefreshToken` enforces `iat.Before(user.TokensInvalidatedAt)`. The parser must preserve `iat` on the returned claims, otherwise logout / password change / admin reset silently stop revoking sessions.
- **Admin role is a platform-wide super-user**, not per-tenant. `@auth(role: ADMIN)` resolvers query across all orgs without an `org_id` filter, and `RequireOrgRole` deliberately bypasses org membership for users whose `User.Role == "admin"`. Treat anyone with `role=admin` as operator-level staff and scope customer administration through `OrganizationMember.Role` (`OWNER` / `ADMIN` / `MEMBER` / `READONLY`) instead.
- **Payment webhook integrity**: read fulfillment amounts from the signed event object (`sess.AmountTotal` for Stripe, decrypted resource for WeChat Pay, verified form values for Alipay) — never from metadata.
- JWT uses access + refresh tokens; CSP, HSTS, and Cache-Control headers are set by middleware.
- `AutoMigrate` is **disabled in `release` mode** — production schema changes must go through explicit SQL migrations (`cmd/migrate`). GORM AutoMigrate is only for local/dev.

### Frontend

- Apollo Client is the single data layer. GraphQL operations are organized by domain under `web/src/lib/graphql/operations/` — reuse existing query/mutation files rather than inlining `gql` in components.
- Zustand holds auth + UI prefs only; server state belongs in Apollo's cache.
- Styling follows an Apple-style design system (neutral `#F5F5F7`, 12px+ radii, soft shadows). Icon libraries are `lucide` / `@heroicons/react` — do not use emoji in UI.
- Pages split into a user-facing dashboard and an admin control panel; routing entry point is `web/src/App.tsx`.

## Conventions

- Backend errors: wrap with `fmt.Errorf("...: %w", err)`; log via `zap` (never `log.Printf`).
- All DB IDs are UUIDs (`github.com/google/uuid`).
- Keep Go files roughly under ~300 lines; split by concern when they grow.
- Commit messages follow conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `security:`).
- One concern per PR.

## Key entry points

- `server/cmd/server/main.go` — backend bootstrap and service wiring.
- `server/cmd/migrate/` — SQL migration CLI (`up`, `down`, `status`, `version`, `seed`).
- `server/internal/api/routes/routes.go` — route registration for REST + GraphQL + ops endpoints.
- `server/internal/graphql/schema/*.graphqls` — source of truth for the management API.
- `server/internal/models/models.go` — GORM models / ER definitions.
- `web/src/App.tsx` — frontend routing.
- `web/src/lib/graphql/client.ts` — Apollo Client setup.

## Further reading inside the repo

`docs/architecture.md`, `docs/graphql-guide.md`, `docs/environment-variables.md` (80+ vars), `docs/feature-gates.md`, `docs/database-schema.md`, `docs/dlp-guide.md`, `docs/sso-integration.md`, `docs/graceful-degradation.md`, `MCP.md`, `deploy/helm/llm-router/README.md`.
