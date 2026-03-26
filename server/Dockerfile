# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with version info (cross-compile for target platform)
ARG VERSION=dev
ARG GIT_COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X llm-router-platform/internal/api/routes.Version=${VERSION} -X llm-router-platform/internal/api/routes.GitCommit=${GIT_COMMIT} -X llm-router-platform/internal/api/routes.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
    -o /app/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s" -o /app/migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.23

LABEL org.opencontainers.image.source="https://github.com/Veritas-Calculus/llm-router-platform"
LABEL org.opencontainers.image.description="LLM Router Platform - API Server"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app

# Install runtime dependencies and apply security patches (incl. zlib CVE-2026-22184, CVE-2026-27171)
RUN apk add --no-cache ca-certificates tzdata && apk upgrade --no-cache --force-missing-repositories && \
    apk add --no-cache --upgrade zlib || true

# Create non-privileged user
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -h /app -s /sbin/nologin

# Copy binary from builder
COPY --from=builder /app/server /app/server
COPY --from=builder /app/migrate /app/migrate
COPY --from=builder /app/migrations /app/migrations

# Copy config if exists
COPY --from=builder /app/.env.example /app/.env.example

# Ensure appuser owns the workdir
RUN chown -R appuser:appgroup /app

# Switch to non-privileged user
USER appuser

# Expose port
EXPOSE 8080

# Run
CMD ["/app/server"]
