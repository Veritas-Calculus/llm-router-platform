# Changelog

All notable changes to the LLM Router Platform will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- **WeChat Pay & Alipay** payment integration with webhook callbacks
- **Feature Gate system** — runtime feature toggles managed via admin UI, stored in DB
- **Prompt Safety engine** — rule-based prompt injection detection
- **Cloudflare Turnstile** CAPTCHA integration for registration and login
- **Admin Dashboard** — platform-level KPIs, user growth, revenue tracking, infrastructure health
- **Audit log export** — CSV export of audit trails for compliance
- **Routing Rules** — configurable model routing with priority/weight/conditions
- **Prompt Template management** — versioned prompt templates with active version selection
- **Announcement system** — platform announcements with scheduling
- **Coupon & Redeem code system** — promotional code generation and redemption
- **Document management** — admin-managed content pages with Markdown editor
- **Anthropic-compatible route** — `POST /v1/v1/messages` for Anthropic SDK compatibility
- **OpenAPI spec** — auto-generated spec at `/openapi.json`
- **Grafana dashboard** — pre-built dashboard JSON for Prometheus metrics
- **k6 load tests** — smoke, load, and stress test scenarios

### Changed
- **Management API** migrated from REST to **GraphQL** (gqlgen, schema-first)
- **Frontend data fetching** migrated from Axios to **Apollo Client**
- **Architecture** refactored: resolvers now route through service layer (no direct DB/Redis access)
- **Billing** — pre-recorded usage for audit resilience; partial billing for streams
- **Provider health** — circuit breaker with 5-failure threshold; automatic recovery
- **API Key auth** — supports both `Authorization: Bearer` and `X-API-Key` headers
- **Project structure** — service layer expanded from 15 to 26 modules

### Security
- **SSRF protection** for provider model sync and webhook URLs
- **PII log sanitization** across all service modules
- **Security headers** (CSP, HSTS, Cache-Control) enforced via middleware
- **Query depth limit** (7) and **complexity limit** (200) for GraphQL
- **GraphQL introspection** disabled by default in production
- **Error sanitization** in release mode
- **Common password blocklist** for registration
- **AES-256-GCM** encryption for provider API keys at rest
- **Backpressure protection** against DB connection pool exhaustion
- **Body size limit** (10MB) to prevent OOM

### Fixed
- Organization members page dropdown query mismatch
- 2FA QR code rendering (data URL generation)
- Health check field name mismatches between frontend and backend
- Docker caching issues requiring clean rebuilds

### Deprecated
- REST management API routes under `/api/v1/` (replaced by `/graphql`)
