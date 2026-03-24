# LLM Router — Helm Chart

> Kubernetes 部署 Chart，用于将 LLM Router Gateway Server 部署到 K8s 集群。

## 安装

```bash
# 从本地 chart 安装
helm install llm-router deploy/helm/llm-router \
  --namespace llm-router --create-namespace \
  --set secret.jwtSecret="your-jwt-secret-32chars" \
  --set secret.encryptionKey="your-32-byte-aes-key" \
  --set secret.adminPassword="DevAdmin123!" \
  --set secret.dbPassword="your-db-password" \
  --set secret.redisPassword="your-redis-password"
```

## 前置依赖

Chart 部署的是 **Gateway Server** 进程，需要外部提供:

| 依赖 | 说明 |
|------|------|
| PostgreSQL 16 | 通过 `config.dbHost` / `secret.dbPassword` 连接 |
| Redis 7 | 通过 `config.redisAddr` / `secret.redisPassword` 连接 (可选) |

> 推荐使用 Bitnami PostgreSQL 和 Redis Helm Charts 或云托管服务。

## 主要配置

### 镜像

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `image.repository` | `ghcr.io/veritas-calculus/llm-router-platform/server` | 镜像仓库 |
| `image.tag` | `latest` | 镜像标签 |
| `image.pullPolicy` | `IfNotPresent` | 拉取策略 |
| `replicaCount` | `2` | 副本数 |

### 应用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `config.ginMode` | `release` | Gin 运行模式 |
| `config.logLevel` | `info` | 日志级别 |
| `config.domain` | `localhost:8080` | 服务域名 |
| `config.corsOrigins` | `http://localhost:5173,...` | CORS 允许源 |
| `config.dbHost` | `postgres` | PostgreSQL 主机 |
| `config.dbPort` | `5432` | PostgreSQL 端口 |
| `config.dbUser` | `postgres` | 数据库用户 |
| `config.dbName` | `llm_router` | 数据库名 |
| `config.dbSslMode` | `disable` | SSL 模式 |
| `config.redisAddr` | `redis:6379` | Redis 地址 |
| `config.redisDb` | `0` | Redis DB 编号 |

### 敏感信息 (Secret)

> ⚠️ 生产环境应使用 External Secrets Operator 或 Sealed Secrets。

| 参数 | 说明 |
|------|------|
| `secret.jwtSecret` | JWT 签名密钥 |
| `secret.adminPassword` | 初始管理员密码 |
| `secret.encryptionKey` | AES-256 加密密钥 |
| `secret.dbPassword` | 数据库密码 |
| `secret.redisPassword` | Redis 密码 |

### 资源与伸缩

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `resources.requests.cpu` | `100m` | CPU 请求 |
| `resources.requests.memory` | `128Mi` | 内存请求 |
| `resources.limits.cpu` | `1000m` | CPU 上限 |
| `resources.limits.memory` | `512Mi` | 内存上限 |
| `autoscaling.enabled` | `true` | 启用 HPA |
| `autoscaling.minReplicas` | `2` | 最小副本 |
| `autoscaling.maxReplicas` | `10` | 最大副本 |
| `autoscaling.targetCPUUtilizationPercentage` | `80` | CPU 目标使用率 |

### Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-read-timeout: "120"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"   # SSE streaming
  hosts:
    - host: llm-gateway.example.com
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
    - secretName: llm-gateway-tls
      hosts:
        - llm-gateway.example.com
```

### Prometheus ServiceMonitor

```yaml
serviceMonitor:
  enabled: true
  path: /internal/metrics
  interval: 10s
  labels:
    release: prometheus    # 匹配你的 Prometheus Operator 标签选择器
```

> 需要开启 `MetricsUnauthenticated` Feature Gate 以便 Prometheus 无认证抓取。

## 安全默认

Chart 默认启用以下安全配置:

- `readOnlyRootFilesystem: true`
- `runAsNonRoot: true` (UID 10001)
- `allowPrivilegeEscalation: false`
- 丢弃所有 Linux Capabilities

## 验证

```bash
helm lint deploy/helm/llm-router
helm template llm-router deploy/helm/llm-router | kubectl apply --dry-run=client -f -
```
