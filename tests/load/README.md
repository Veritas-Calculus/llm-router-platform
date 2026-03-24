# Load Testing

## Prerequisites

Install [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/):

```bash
brew install k6          # macOS
```

## Quick Start

```bash
# Smoke test (1 VU, 30s)
k6 run --env BASE_URL=http://localhost:8080 \
       --env API_KEY=sk-your-key \
       tests/load/k6-load-test.js \
       --scenario smoke

# Full load test (smoke → load → stress)
k6 run --env BASE_URL=http://localhost:8080 \
       --env API_KEY=sk-your-key \
       tests/load/k6-load-test.js
```

## Scenarios

| Scenario | VUs | Duration | Purpose |
|----------|-----|----------|---------|
| **Smoke** | 1 | 30s | Verify system works |
| **Load** | 0→20→50→0 | 8min | Normal traffic |
| **Stress** | 0→100→200→0 | 7min | Find breaking point |

## Thresholds (Baselines)

| Metric | Target |
|--------|--------|
| `/health` P95 | < 100ms |
| `/graphql` (dashboard query) P95 | < 500ms |
| `/v1/chat/completions` P95 | < 5000ms |
| Error rate | < 5% |

## Results

Results are exported to `tests/load/results/summary.json` after each run.

## Grafana Integration

Import the results into Grafana using the k6 Cloud output or the k6-to-Prometheus remote-write adapter:

```bash
k6 run --out experimental-prometheus-rw \
       tests/load/k6-load-test.js
```
