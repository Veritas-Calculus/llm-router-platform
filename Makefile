.PHONY: all build test clean run dev docker-build docker-up docker-down lint

# Variables
BINARY_NAME=llm-router-server
GO_FILES=$(shell find . -name '*.go' -type f)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
DOCKER_REGISTRY ?= ghcr.io/veritas-calculus
LDFLAGS=-ldflags="-w -s -X llm-router-platform/internal/api/routes.Version=$(VERSION) -X llm-router-platform/internal/api/routes.GitCommit=$(GIT_COMMIT) -X llm-router-platform/internal/api/routes.BuildTime=$(BUILD_TIME)"

all: build

# Build
build:
	@echo "Building server... ($(VERSION) $(GIT_COMMIT))"
	cd server && go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

build-linux:
	@echo "Building for Linux... ($(VERSION) $(GIT_COMMIT))"
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux ./cmd/server

# Test
test:
	@echo "Running tests..."
	cd server && go test ./... -v

test-coverage:
	@echo "Running tests with coverage..."
	cd server && go test ./... -coverprofile=coverage.out
	cd server && go tool cover -html=coverage.out -o coverage.html

# Clean
clean:
	@echo "Cleaning..."
	rm -rf server/bin
	rm -rf web/dist
	rm -rf web/node_modules

# Run
run:
	@echo "Running server..."
	cd server && go run ./cmd/server

dev:
	@echo "Running in development mode..."
	cd server && go run ./cmd/server &
	cd web && npm run dev

# Frontend
web-install:
	@echo "Installing web dependencies..."
	cd web && npm install

web-build:
	@echo "Building web..."
	cd web && npm run build

web-dev:
	@echo "Running web dev server..."
	cd web && npm run dev

# Docker
docker-build:
	@echo "Building Docker images..."
	docker-compose build

docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f

docker-multiarch:
	@echo "Building multi-platform Docker images (amd64 + arm64)..."
	docker buildx build --platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(DOCKER_REGISTRY)/llm-router-server:$(VERSION) \
		-f server/Dockerfile server/ --push
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(DOCKER_REGISTRY)/llm-router-web:$(VERSION) \
		-f web/Dockerfile web/ --push

# Lint
lint:
	@echo "Running linter..."
	cd server && golangci-lint run ./...

# Database
migrate-up:
	@echo "Running migrations..."
	cd server && go run ./cmd/migrate up

migrate-down:
	@echo "Rolling back migrations..."
	cd server && go run ./cmd/migrate down

migrate-version:
	cd server && go run ./cmd/migrate version

migrate-status:
	cd server && go run ./cmd/migrate status

# Help
help:
	@echo "Available targets:"
	@echo "  build             - Build the server binary (with version info)"
	@echo "  build-linux       - Build for Linux"
	@echo "  test              - Run tests"
	@echo "  test-coverage     - Run tests with coverage"
	@echo "  clean             - Clean build artifacts"
	@echo "  run               - Run the server"
	@echo "  dev               - Run in development mode"
	@echo "  web-install       - Install web dependencies"
	@echo "  web-build         - Build web frontend"
	@echo "  web-dev           - Run web dev server"
	@echo "  docker-build      - Build Docker images"
	@echo "  docker-up         - Start Docker containers"
	@echo "  docker-down       - Stop Docker containers"
	@echo "  migrate-up        - Run SQL migrations"
	@echo "  migrate-down      - Rollback last migration"
	@echo "  migrate-version   - Show current migration version"
	@echo "  migrate-status    - Check DB connection and migration status"
	@echo "  lint              - Run linters"

	@echo "  docker-logs    - Show Docker logs"
	@echo "  lint           - Run linter"
	@echo "  help           - Show this help"
