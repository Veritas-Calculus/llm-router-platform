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

# ---------------------------------------------------------------------------
# Schema-drift skip list
# ---------------------------------------------------------------------------
# Each entry below is an object name that legitimately exists on ONLY one side
# of the SQL-vs-GORM comparison and cannot be aligned without rewriting either
# the migrations or the GORM model in a worse way. We strip these from BOTH
# pg_dump outputs before diffing.
#
# Adding a name here is a deliberate choice: future, unintentional drift will
# still surface because the entry must be added by hand. Do NOT replace this
# with a regex blanket — that would silently hide real schema regressions.
#
# Each entry includes a short reason. When you add a new one, write the
# reason next to it.

EXEMPT_OBJECT_NAMES=(
    # --- Indexes on legacy columns no longer modelled in GORM ---
    # `budgets`, `subscriptions`, `orders`, `async_tasks`, `conversation_memories`
    # all still have a `user_id` column (initial migrations 0001-0003) but the
    # GORM struct now keys off `org_id` / `project_id`. The user_id columns
    # are kept for historical data; the indexes remain useful for any direct
    # SQL queries against legacy rows. GORM never sees these columns so it
    # cannot generate the index.
    "idx_async_tasks_user_id"
    "idx_conversation_memories_user_id"
    "idx_orders_user_id"
    "idx_budgets_user_id"
    "idx_subscriptions_user_id"

    # --- Indexes on BaseModel-embedded CreatedAt that we keep selectively ---
    # `usage_logs.created_at` is heavily queried for hot-path billing rollups.
    # GORM's BaseModel embeds CreatedAt without an `index` tag (deliberately,
    # so most tables don't get a redundant created_at index). Adding the tag
    # globally would index CreatedAt on every table — too costly. Keep the
    # SQL-only index for usage_logs alone.
    "idx_usage_logs_created_at"

    # --- Composite indexes with DESC ordering ---
    # GORM cannot express column ordering inside composite index tags
    # (`gorm:"index:foo,priority:1"` controls order WITHIN a composite, not
    # ASC/DESC of individual columns). The DESC ordering matters for our
    # "most recent N usage logs per project" hot path (migration 000016).
    "idx_usage_logs_project_channel_created_at"
    "idx_usage_logs_project_created_at"

    # --- Partial indexes (WHERE clause) ---
    # GORM has no syntax for `CREATE INDEX ... WHERE ...`. These are
    # query-specific partials that GORM AutoMigrate cannot replicate.
    "idx_models_provider_kind"           # WHERE deleted_at IS NULL — soft-delete-aware composite
    "idx_webhook_deliveries_pending_due" # WHERE status='pending' — dispatcher hot scan
    "idx_transactions_idempotency_key"   # UNIQUE WHERE idempotency_key IS NOT NULL — partial unique

    # --- Materialized view + its indexes ---
    # `usage_logs_daily_rollup` is a materialized view defined in migration
    # 000020 for fast dashboard rollups. GORM has no concept of materialized
    # views — AutoMigrate will never produce one. The MV's own indexes
    # (including its PK index) follow the same rationale.
    "usage_logs_daily_rollup"
    "idx_usage_logs_daily_rollup_pk"
    "idx_usage_logs_daily_rollup_project_channel_day"
    "idx_usage_logs_daily_rollup_project_day"
)

# Build a single `grep -E -v` pattern that matches any line referencing one
# of the exempt object names. We anchor on word boundaries so a substring
# match doesn't accidentally hide an unrelated object.
EXEMPT_PATTERN=""
for name in "${EXEMPT_OBJECT_NAMES[@]}"; do
    if [[ -n "$EXEMPT_PATTERN" ]]; then EXEMPT_PATTERN+="|"; fi
    EXEMPT_PATTERN+="\\b${name}\\b"
done

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
    rm -f /tmp/schema_sql.dump /tmp/schema_gorm.dump \
          /tmp/schema_sql.dump.raw /tmp/schema_gorm.dump.raw \
          /tmp/schema_diff.txt
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

# Dump both schemas. We exclude:
#   - schema_migrations: the migration tracking table itself (irrelevant to drift)
#   - usage_logs_daily_rollup: a materialized view (see EXEMPT_OBJECT_NAMES);
#     pg_dump's --exclude-table also drops materialized views, which suppresses
#     the entire CREATE MATERIALIZED VIEW block from the SQL dump.
docker exec "$SQL_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" --schema-only \
    --exclude-table='schema_migrations' \
    --exclude-table='usage_logs_daily_rollup' \
    --no-owner --no-privileges --no-comments \
    | grep -v '^--' | grep -v '^$' | sort > /tmp/schema_sql.dump.raw

docker exec "$GORM_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" --schema-only \
    --exclude-table='schema_migrations' \
    --exclude-table='usage_logs_daily_rollup' \
    --no-owner --no-privileges --no-comments \
    | grep -v '^--' | grep -v '^$' | sort > /tmp/schema_gorm.dump.raw

# Apply the explicit exempt-object skip list (see EXEMPT_OBJECT_NAMES near the
# top of this file). Any line referencing an exempt name is stripped from
# BOTH dumps so the diff focuses on real drift.
if [[ -n "$EXEMPT_PATTERN" ]]; then
    grep -E -v "$EXEMPT_PATTERN" /tmp/schema_sql.dump.raw > /tmp/schema_sql.dump
    grep -E -v "$EXEMPT_PATTERN" /tmp/schema_gorm.dump.raw > /tmp/schema_gorm.dump
else
    cp /tmp/schema_sql.dump.raw  /tmp/schema_sql.dump
    cp /tmp/schema_gorm.dump.raw /tmp/schema_gorm.dump
fi

if diff -u /tmp/schema_sql.dump /tmp/schema_gorm.dump > /tmp/schema_diff.txt 2>&1; then
    echo ""
    echo "✅ Schemas match! SQL migrations and GORM AutoMigrate produce identical schemas."
    echo "   (Exempt objects skipped: ${#EXEMPT_OBJECT_NAMES[@]}; see EXEMPT_OBJECT_NAMES at the top of this script.)"
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
    echo "        If the diff is a legitimately-unbridgeable object (materialized view,"
    echo "        partial index, composite-with-DESC), add it to EXEMPT_OBJECT_NAMES"
    echo "        near the top of this script with a one-line explanation."
    exit 1
fi
