# Gemini CLI - Project Context: LLM Router Platform

This file provides architectural context and development guidelines for the LLM Router Platform, a unified API gateway for multiple Large Language Models (LLMs).

## Project Overview

- **Purpose:** A centralized routing and management platform for LLM services (OpenAI, Claude, Gemini, etc.), providing an OpenAI-compatible interface with added features like proxy pooling, billing, and health monitoring.
- **Backend:** Go 1.24 with Gin (Web Framework), **gqlgen (GraphQL)**, GORM (ORM), Zap (Logging), and Viper (Config).
- **Frontend:** React 19 (TypeScript), Vite, TailwindCSS v4, **Apollo Client (GraphQL)**, Zustand (State Management), and Recharts (Visualization).
- **Infrastructure:** PostgreSQL 16 (Relational Data), Redis 7 (Caching/Rate Limiting), Docker Compose (Orchestration).
- **Design Philosophy:** "Apple-style" clean UI with high-performance Go backend.

## Technical Architecture

- **Monorepo Structure:**
  - `server/`: Go backend source code.
    - `cmd/`: Application entry points (`server`, `migrate`).
    - `internal/api/handlers/`: LLM proxy handlers only (chat, streaming, embeddings, images, audio, tts, model, payment webhook).
    - `internal/graphql/`: **GraphQL management API** (schema, resolvers, directives, dataloaders).
    - `internal/service/`: Business logic layer (provider, router, billing, health, etc.).
    - `pkg/`: Public/reusable utility packages.
  - `web/`: React frontend source code.
    - `src/components/`: UI components (including Apple-style design elements).
    - `src/pages/`: Main application views.
    - `src/lib/graphql/`: Apollo Client setup and GraphQL operations.
    - `src/stores/`: Zustand state definitions.
  - `docker-compose.yml`: Orchestration for local development and production.

## Building and Running

### Prerequisites
- Go 1.24+
- Node.js 18+ (pnpm/npm)
- Docker & Docker Compose

### Development Workflow
From the project root:
- **Run all services (dev):** `make dev` (Starts Go server and Vite dev server)
- **Start infrastructure:** `docker-compose up -d postgres redis`
- **Full stack (Docker):** `docker-compose up -d`

### Individual Components
- **Server:**
  ```bash
  cd server
  go mod download
  cp .env.example .env
  go run cmd/server/main.go
  ```
- **Web:**
  ```bash
  cd web
  npm install
  npm run dev
  ```

### Testing and Linting
- **Test Backend:** `cd server && go test ./...`
- **Test Frontend:** `cd web && npm test`
- **Lint Backend:** `cd server && golangci-lint run`
- **Lint Frontend:** `cd web && npm run lint`

## Development Conventions

### Backend (Go)
- **Layered Architecture:** LLM proxy follows `Handler -> Service -> Repository -> Model`. Management API follows `GraphQL Resolver -> Service -> Repository -> Model`.
- **Error Handling:** Use wrapped errors and structured logging via `zap`.
- **API Layout:** LLM API is under `/v1/` (REST, OpenAI-compatible). Management API is under `/graphql` (GraphQL, schema-first via gqlgen). REST management routes under `/api/v1/` have been deprecated.
- **Security:** API keys for providers are stored encrypted in PostgreSQL using the `ENCRYPTION_KEY`.

### GraphQL (gqlgen)
- **Schema-First:** All types and operations defined in `.graphqls` files under `server/internal/graphql/schema/`.
- **Directives:** `@auth(requires: ROLE)` for field-level authorization, `@rateLimit(max: N, window: "duration")` for field-level rate limiting.
- **Resolvers:** Business logic in `server/internal/graphql/resolvers/`, calling service layer directly.
- **Dataloaders:** N+1 query optimization via `server/internal/graphql/dataloaders/`.
- **Security:** Query depth limit (7), complexity limit (200), introspection disabled in production, error sanitization in release mode.

### Frontend (React)
- **Styling:** Strict adherence to "Apple Design Style" (neutral colors #F5F5F7, large border-radius 12px+, soft shadows).
- **State:** Use Zustand for global state (Auth, UI preferences).
- **Data Fetching:** Standardize on **Apollo Client** (`useQuery`/`useMutation`) for all management operations. GraphQL operations are organized by domain in `web/src/lib/graphql/operations/`.

### Database
- **Migrations:** Managed via GORM AutoMigrate in `server/internal/database/database.go` for non-release modes. Production/Release mode requires explicit SQL migrations.
- **Primary Keys:** UUIDs are used for all record identifiers.

## Security & Resilience

### Resilience Patterns
- **Provider-Level Circuit Breaking:** Automatically melts (skips) unhealthy providers after 5 consecutive 5xx or timeout errors.
- **API Key Rotation:** Retries requests with alternative API keys upon 429 (rate limit) or quota errors.
- **Context-Aware Streaming:** Background streaming goroutines strictly respect context cancellation to prevent resource leaks.

### Billing Robustness
- **Pre-recorded Usage:** All LLM requests (including streams) are pre-recorded in the database to ensure auditing even if the connection is interrupted.
- **Partial Billing:** Streamed chunks are tracked, and usage logs are updated upon completion or failure to capture partial consumption.

### Hardening
- **Encryption:** Provider API keys are mandatory-encrypted at rest using AES-GCM.
- **Security Headers:** Strict CSP, HSTS, and Cache-Control headers are enforced via middleware.
- **Production Guardrails:** `AutoMigrate` is disabled in `release` mode to prevent accidental schema corruption.

## Key Files
- `server/cmd/server/main.go`: Backend entry point and service initialization.
- `server/internal/api/routes/routes.go`: Route registration (LLM, GraphQL, operational endpoints).
- `server/internal/graphql/schema/*.graphqls`: GraphQL schema definitions.
- `server/internal/graphql/resolvers/`: GraphQL resolver implementations.
- `server/internal/graphql/directives/auth.go`: `@auth` directive implementation.
- `server/internal/models/models.go`: Core data structures and GORM models.
- `web/src/App.tsx`: Frontend routing and layout structure.
- `web/src/lib/graphql/client.ts`: Apollo Client initialization.
- `web/src/lib/graphql/operations/`: GraphQL query/mutation definitions.
- `docker-compose.yml`: Local infrastructure setup.
- `Makefile`: Common development tasks.

## User-Specific Context
- **API Base:** The server runs on port `8080` by default.
- **Admin Default:** `admin@example.com` / `admin@example.com` (configurable via env).
- **LLM Compatibility:** Supports OpenAI, Claude, and Gemini providers out of the box.
- **GraphQL Playground:** Available at `/graphql` in non-release mode.
