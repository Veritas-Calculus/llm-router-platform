# Grafana Dashboard

## 导入 Dashboard

### 方式 1: 通过 Grafana UI 导入

1. 进入 Grafana → **Dashboards → Import**
2. 上传 `deploy/grafana/dashboards/llm-router-overview.json`
3. 选择 Prometheus 数据源
4. 点击 **Import**

### 方式 2: 通过 ConfigMap 自动加载

如果使用 Grafana Helm Chart，可通过 sidecar 自动导入:

```yaml
# values.yaml for grafana helm chart
sidecar:
  dashboards:
    enabled: true
    label: grafana_dashboard
```

然后创建 ConfigMap:

```bash
kubectl create configmap grafana-llm-router \
  --from-file=deploy/grafana/dashboards/llm-router-overview.json \
  -n monitoring
kubectl label configmap grafana-llm-router grafana_dashboard=1 -n monitoring
```

## Dashboard 面板

`llm-router-overview.json` 包含以下面板:

| 面板 | 指标 | 说明 |
|------|------|------|
| Request Rate | `http_requests_total` | 每秒请求数 (QPS) |
| Latency P50/P95/P99 | `http_request_duration_seconds` | 请求延迟分位数 |
| Error Rate | `http_requests_total{status=~"5.."}` | 5xx 错误率 |
| Active Connections | `gin_active_connections` | 当前活跃连接数 |
| Auth Failures | `auth_failures_total` | 按类型的认证失败计数 |

## 数据源配置

确保 Prometheus 能够抓取 LLM Router 的 metrics 端点:

```yaml
# prometheus scrape config
scrape_configs:
  - job_name: llm-router
    static_configs:
      - targets: ['llm-router:8080']
    metrics_path: /internal/metrics    # 需开启 MetricsUnauthenticated Feature Gate
    scrape_interval: 10s
```

或通过 Helm Chart 的 ServiceMonitor (参见 [Helm README](../helm/llm-router/README.md))。
