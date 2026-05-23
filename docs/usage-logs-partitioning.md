# usage_logs Partitioning Runbook

This document is the operator-driven companion to migration `000020_usage_logs_daily_rollup` and addresses **DB-H1** in the audit roadmap. It covers two questions:

1. **When** should `usage_logs` be partitioned?
2. **How** to perform the partition swap with minimum downtime.

## When to partition

The materialized view `usage_logs_daily_rollup` shipped in 000020 absorbs almost all of the dashboard query load — its rows are already pre-aggregated and the query planner can hit a small btree index instead of scanning millions of raw rows. That is sufficient for most installations.

Consider partitioning the underlying `usage_logs` table when **any** of the following hold:

- `usage_logs` exceeds **50 million** rows.
- Single-row inserts are taking > 5 ms p50 (the table's btree indexes have grown beyond memory).
- The daily refresh of `usage_logs_daily_rollup` takes > 5 minutes.
- Compliance requires bounded data retention (you want to `DROP PARTITION` to purge old months).

## Partition strategy

Range-partition by `created_at` at monthly granularity:

```sql
CREATE TABLE usage_logs_partitioned (LIKE usage_logs INCLUDING ALL)
    PARTITION BY RANGE (created_at);

-- One partition per month
CREATE TABLE usage_logs_2026_05 PARTITION OF usage_logs_partitioned
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE usage_logs_2026_06 PARTITION OF usage_logs_partitioned
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
-- … one per month going forward
```

Use `pg_partman` (or the simpler `pg_cron`-driven recipe below) to auto-provision next month's partition a few days ahead.

## Online swap procedure

1. **Pre-create** `usage_logs_partitioned` and all required monthly child partitions covering the existing data range, plus the next 3 months.

2. **Copy existing data**:

   ```sql
   INSERT INTO usage_logs_partitioned
       SELECT * FROM usage_logs;
   ```

   For multi-billion-row tables, do this in batched windows (e.g. month-by-month) inside a transaction. Run during off-peak hours; expect this to take hours.

3. **Verify counts match**:

   ```sql
   SELECT count(*) FROM usage_logs;
   SELECT count(*) FROM usage_logs_partitioned;
   ```

4. **Stop the LLM proxy briefly** (drain in-flight requests via `lifecycleCtx`). Optional but recommended; the next 3 SQL statements should be one transaction.

5. **Swap the tables**:

   ```sql
   BEGIN;
   ALTER TABLE usage_logs RENAME TO usage_logs_legacy;
   ALTER TABLE usage_logs_partitioned RENAME TO usage_logs;
   COMMIT;
   ```

6. **Restart the proxy**. GORM model and all `repository/usage_repository.go` queries point at the table name `usage_logs` and don't care that it's now partitioned.

7. **Refresh the rollup view** so it picks up any rows that arrived during step 5:

   ```sql
   REFRESH MATERIALIZED VIEW CONCURRENTLY usage_logs_daily_rollup;
   ```

8. **Drop `usage_logs_legacy`** after a few days of monitoring confirms parity.

## Auto-provisioning future partitions

If you don't have `pg_partman`, schedule a monthly `pg_cron` job:

```sql
SELECT cron.schedule('usage_logs_make_next_month', '0 0 25 * *', $$
    DO $body$
    DECLARE
        next_start date := date_trunc('month', NOW() + INTERVAL '1 month')::date;
        next_end   date := date_trunc('month', NOW() + INTERVAL '2 months')::date;
        part_name  text := 'usage_logs_' || to_char(next_start, 'YYYY_MM');
    BEGIN
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF usage_logs FOR VALUES FROM (%L) TO (%L)',
            part_name, next_start, next_end
        );
    END
    $body$;
$$);
```

## Refreshing the rollup view

A separate background job (or `pg_cron`) refreshes the rollup every hour:

```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY usage_logs_daily_rollup;
```

`CONCURRENTLY` requires the unique index that 000020 already created. The first refresh after a fresh install can be non-concurrent (faster, but blocks readers); subsequent ones must be concurrent so dashboards don't pause.

## Rolling back

The migration 000020 down only drops the materialized view. Partitioning is a one-way operational change; if you need to revert it, repeat the swap procedure in the opposite direction (create an unpartitioned `usage_logs_flat`, copy data, swap names).
