.PHONY: all build test clean run dev docker-build docker-up docker-down lint

# Variables
BINARY_NAME=llm-router-server
GO_FILES=$(shell find . -name '*.go' -type f)

all: build

# Build
build:
	@echo "Building server..."
	cd server && go build -o bin/$(BINARY_NAME) ./cmd/server

build-linux:
	@echo "Building for Linux..."
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/$(BINARY_NAME)-linux ./cmd/server

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

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the server binary"
	@echo "  build-linux    - Build for Linux"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Clean build artifacts"
	@echo "  run            - Run the server"
	@echo "  dev            - Run in development mode"
	@echo "  web-install    - Install web dependencies"
	@echo "  web-build      - Build web frontend"
	@echo "  web-dev        - Run web dev server"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start Docker containers"
	@echo "  docker-down    - Stop Docker containers"
	@echo "  docker-logs    - Show Docker logs"
	@echo "  lint           - Run linter"
	@echo "  help           - Show this help"
