# Contributing to LLM Router Platform

Thanks for considering contributing! Here's how to get started.

## Prerequisites

- **Go** 1.24+
- **Node.js** 18+ (npm or pnpm)
- **Docker & Docker Compose**
- **PostgreSQL 16**, **Redis 7** (or use Docker)

## Getting Started

```bash
# Clone
git clone https://github.com/your-org/llm-router-platform.git
cd llm-router-platform

# Start infrastructure
docker-compose up -d postgres redis

# Backend
cd server
cp .env.example .env       # Edit as needed
go mod download
go run cmd/server/main.go

# Frontend (separate terminal)
cd web
npm install
npm run dev
```

## Development Workflow

| Command | Description |
|---------|-------------|
| `make dev` | Run backend + frontend in dev mode |
| `cd server && go test ./...` | Run backend tests |
| `cd web && npm test` | Run frontend tests (vitest) |
| `cd server && golangci-lint run` | Lint Go code |
| `cd web && npm run lint` | Lint TypeScript/React |

## Architecture

```
server/
  cmd/server/       # Entry point
  internal/
    api/handlers/   # HTTP handlers (one file per domain)
    api/middleware/  # Auth, CORS, rate limiting, logging
    api/routes/     # Route registration
    config/         # Viper-based configuration
    models/         # GORM models
    repository/     # Data access (interface-based)
    service/        # Business logic
  pkg/              # Shared utilities

web/
  src/
    components/     # Reusable UI components
    pages/          # Route-level page components
    hooks/          # Custom React hooks
    stores/         # Zustand state management
    lib/            # API client, utilities
```

## Code Conventions

### Backend (Go)

- **Layered architecture**: `Handler → Service → Repository → Model`
- **Error handling**: Wrap context with `fmt.Errorf("...: %w", err)`
- **Logging**: Use `zap.Logger` (structured, never `log.Printf`)
- **API keys**: Encrypted at rest via `crypto.Encrypt()`
- **DB IDs**: UUIDs everywhere (`github.com/google/uuid`)
- **File splitting**: Keep files under ~300 lines; split by concern

### Frontend (React/TypeScript)

- **Styling**: Apple Design (neutral colors `#F5F5F7`, border-radius 12px+)
- **State**: Zustand for global state
- **Data fetching**: Apollo Client (`useQuery`/`useMutation`) via `@/lib/graphql/`
- **Testing**: Vitest + React Testing Library

## Pull Request Guidelines

1. **Branch from `main`**
2. **One concern per PR** — don't mix unrelated changes
3. **Tests required** for new handlers/services
4. **`go build ./...` and `go test ./...`** must pass
5. **`npm run lint` and `npm test`** must pass for frontend changes
6. **Commit messages**: Use conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`)

## Environment Variables

See `server/.env.example` for all available configuration. Key ones:

| Variable | Default | Description |
|----------|---------|-------------|
| `REGISTRATION_MODE` | `open` | `open` / `invite` / `closed` |
| `INVITE_CODE` | _(empty)_ | Required when mode is `invite` |
| `ENCRYPTION_KEY` | — | 32-byte AES-256 key for API key encryption |
| `JWT_SECRET` | — | Secret for JWT token signing |

## License

See [LICENSE](LICENSE) for details.
