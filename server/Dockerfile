# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with version info
ARG VERSION=dev
ARG GIT_COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X llm-router-platform/internal/api/routes.Version=${VERSION} -X llm-router-platform/internal/api/routes.GitCommit=${GIT_COMMIT} -X llm-router-platform/internal/api/routes.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
    -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.23

WORKDIR /app

# Install runtime dependencies and apply security patches
RUN apk add --no-cache ca-certificates tzdata && apk upgrade --no-cache

# Create non-privileged user
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -h /app -s /sbin/nologin

# Copy binary from builder
COPY --from=builder /app/server /app/server

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
