#!/usr/bin/env bash
# check-schema.sh — Verify GORM AutoMigrate and SQL migrations produce the same schema.
#
# Requires: Docker, psql, pg_dump, Go 1.24+
#
# How it works:
#   1. Starts TWO temporary Postgres containers (schema_sql and schema_gorm)
#   2. Runs SQL migrations (golang-migrate) against schema_sql
#   3. Runs GORM AutoMigrate against schema_gorm
#   4. Compares pg_dump --schema-only output of both
#
# Exit code 0 = schemas match, 1 = schemas differ
set -euo pipefail

PG_IMAGE="pgvector/pgvector:pg16"
PG_USER="testuser"
PG_PASS="testpass"
PG_DB="testdb"
NET_NAME="schema-check-net-$$"
SQL_CONTAINER="schema-sql-$$"
GORM_CONTAINER="schema-gorm-$$"

MIGRATION_DIR="$(cd "$(dirname "$0")/../server/migrations" && pwd)"
SERVER_DIR="$(cd "$(dirname "$0")/../server" && pwd)"
# The AutoMigrate program must live inside the server module tree to import
# `llm-router-platform/internal/...` packages — Go forbids importing internal
# packages from outside the module's directory subtree.
AUTOMIGRATE_DIR="${SERVER_DIR}/cmd/schema_check_automigrate_$$"
AUTOMIGRATE_GO="${AUTOMIGRATE_DIR}/main.go"

cleanup() {
    echo "🧹 Cleaning up..."
    docker rm -f "$SQL_CONTAINER" "$GORM_CONTAINER" 2>/dev/null || true
    docker network rm "$NET_NAME" 2>/dev/null || true
    rm -f /tmp/schema_sql.dump /tmp/schema_gorm.dump
    rm -rf "$AUTOMIGRATE_DIR"
}
trap cleanup EXIT

echo "📦 Starting temporary Postgres containers..."
docker network create "$NET_NAME" >/dev/null 2>&1 || true

# Start two PG instances. The GORM container also publishes its port to
# 127.0.0.1 because `go run` executes on the host and host→container-IP
# routing is not available on Docker Desktop (Mac/Windows). On Linux CI
# the container IP would work; the host-port publish path works on both.
docker run -d --rm \
    --name "$SQL_CONTAINER" \
    --network "$NET_NAME" \
    -e POSTGRES_USER="$PG_USER" \
    -e POSTGRES_PASSWORD="$PG_PASS" \
    -e POSTGRES_DB="$PG_DB" \
    "$PG_IMAGE" >/dev/null

docker run -d --rm \
    --name "$GORM_CONTAINER" \
    --network "$NET_NAME" \
    -p 127.0.0.1::5432 \
    -e POSTGRES_USER="$PG_USER" \
    -e POSTGRES_PASSWORD="$PG_PASS" \
    -e POSTGRES_DB="$PG_DB" \
    "$PG_IMAGE" >/dev/null

# Wait for both to be ready
for name in "$SQL_CONTAINER" "$GORM_CONTAINER"; do
    echo "  ⏳ Waiting for $name..."
    for i in $(seq 1 30); do
        if docker exec "$name" pg_isready -U "$PG_USER" -d "$PG_DB" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
done

echo ""
echo "🔄 Running SQL migrations on $SQL_CONTAINER..."
# Get the IP of the SQL container
SQL_HOST=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$SQL_CONTAINER")

# Use golang-migrate via docker
docker run --rm \
    --network "$NET_NAME" \
    -v "${MIGRATION_DIR}:/migrations" \
    migrate/migrate:latest \
    -path=/migrations \
    -database "postgres://${PG_USER}:${PG_PASS}@${SQL_CONTAINER}:5432/${PG_DB}?sslmode=disable" \
    up

echo ""
echo "🔄 Running GORM AutoMigrate on $GORM_CONTAINER..."
# GORM runs on the host — connect via the published port on 127.0.0.1.
GORM_HOST="127.0.0.1"
GORM_PORT=$(docker inspect -f '{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}' "$GORM_CONTAINER")

# Run a temporary Go program that performs AutoMigrate. It must live inside
# $SERVER_DIR so Go allows the internal package imports.
mkdir -p "$AUTOMIGRATE_DIR"
cat > "$AUTOMIGRATE_GO" << 'GOEOF'
package main

import (
    "fmt"
    "os"

    "llm-router-platform/internal/database"
    "llm-router-platform/internal/config"
)

func main() {
    port := os.Getenv("DB_PORT")
    if port == "" {
        port = "5432"
    }
    cfg := &config.DatabaseConfig{
        Host:     os.Getenv("DB_HOST"),
        Port:     port,
        User:     os.Getenv("DB_USER"),
        Password: os.Getenv("DB_PASSWORD"),
        Name:     os.Getenv("DB_NAME"),
        SSLMode:  "disable",
    }
    db, err := database.New(cfg, "debug", nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
        os.Exit(1)
    }
    if err := db.Migrate(); err != nil {
        fmt.Fprintf(os.Stderr, "automigrate failed: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("AutoMigrate completed successfully")
}
GOEOF

cd "$SERVER_DIR"
DB_HOST="$GORM_HOST" DB_PORT="$GORM_PORT" \
    DB_USER="$PG_USER" DB_PASSWORD="$PG_PASS" DB_NAME="$PG_DB" \
    go run "$AUTOMIGRATE_GO"

echo ""
echo "📊 Comparing schemas..."

# Dump both schemas (excluding migration tracking tables)
docker exec "$SQL_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" --schema-only \
    --exclude-table='schema_migrations' \
    --no-owner --no-privileges --no-comments \
    | grep -v '^--' | grep -v '^$' | sort > /tmp/schema_sql.dump

docker exec "$GORM_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" --schema-only \
    --exclude-table='schema_migrations' \
    --no-owner --no-privileges --no-comments \
    | grep -v '^--' | grep -v '^$' | sort > /tmp/schema_gorm.dump

if diff -u /tmp/schema_sql.dump /tmp/schema_gorm.dump > /tmp/schema_diff.txt 2>&1; then
    echo ""
    echo "✅ Schemas match! SQL migrations and GORM AutoMigrate produce identical schemas."
    exit 0
else
    echo ""
    echo "❌ Schema mismatch detected!"
    echo ""
    echo "--- SQL Migrations vs GORM AutoMigrate ---"
    cat /tmp/schema_diff.txt
    echo ""
    echo "Lines starting with '-' exist only in SQL migrations."
    echo "Lines starting with '+' exist only in GORM AutoMigrate."
    echo ""
    echo "ACTION: Add SQL migration to match the GORM model changes, or update GORM models."
    exit 1
fi
