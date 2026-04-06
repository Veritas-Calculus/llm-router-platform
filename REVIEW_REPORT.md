# LLM Router Platform 多维度审查报告

**审查日期**: 2026-04-06  
**审查范围**: 架构设计、文档管理、部署维护、监控运维、安全  
**审查方式**: 五个专业 Agent 并行审查（Software Architect / Technical Writer / DevOps / SRE / Security Engineer）

---

## 目录

- [综合概览](#综合概览)
- [一、架构设计审查](#一架构设计审查)
- [二、文档管理审查](#二文档管理审查)
- [三、部署与维护审查](#三部署与维护审查)
- [四、监控运维审查](#四监控运维审查)
- [五、安全审查](#五安全审查)
- [六、跨维度优先修复路线图](#六跨维度优先修复路线图)

---

## 综合概览


| 维度   | 评分    | 严重问题数 | 中等问题数 | 低等问题数 |
| ---- | ----- | ----- | ----- | ----- |
| 架构设计 | B+    | 5     | 15    | 7     |
| 文档管理 | 3.0/5 | 7     | 12    | 9     |
| 部署维护 | B     | 8     | 12    | 7     |
| 监控运维 | B     | 5     | 10    | 8     |
| 安全   | B+    | 2     | 4     | 5     |


**总发现**: 共计 **131 个问题**（严重/高 27 个，中 53 个，低 36 个，信息 15 个）

### 整体评价

这是一个功能丰富、工程质量较高的 LLM 路由平台。分层架构合理，安全基础设施扎实（AES-256-GCM 加密、CORS 防护、CSRF 防护、多层安全扫描），Feature Gate 系统设计精良，Docker 容器安全加固到位。主要风险集中在：

1. **数据层正确性**：金额字段使用 float64、缺少分区策略、双重迁移冲突
2. **支付安全**：微信支付缺少 HTTP 签名验证
3. **部署一致性**：两套 Helm Chart 共存且功能互补但不兼容
4. **监控完整度**：Alertmanager 未配置、DB 连接池指标采集未完成
5. **文档准确性**：Swagger 规范为空、多处文档与代码不一致

---

## 一、架构设计审查

### 高优先级问题


| #    | 问题描述                         | 位置                                                       | 原因说明                                                                                                                                                                                           |
| ---- | ---------------------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A-3  | **双重迁移策略冲突**                 | `server/cmd/server/main.go:206-217`                      | 系统同时运行 `golang-migrate` SQL 迁移和 GORM `AutoMigrate`。两个系统对 schema 有不同的命名约定（如约束名差异导致 migration 000007 的存在），AutoMigrate 的"只增不减"策略可能创建出 SQL 迁移未预期的列或索引。**建议**：选择一种迁移策略并坚持使用，推荐仅使用 `golang-migrate`。 |
| A-9  | **usage_logs 表缺少分区策略**       | `server/migrations/000001_initial_schema.up.sql:122-149` | 每次 API 请求产生一行记录，无分区策略。随数据增长，时间范围聚合查询会变慢。也缺少数据保留机制。**建议**：按月范围分区 `PARTITION BY RANGE (created_at)`。                                                                                             |
| A-10 | **金额字段使用 float64**           | `server/internal/models/provider.go:48-52`               | `input_price_per1_k`、`output_price_per1_k`、`cost` 等金额字段使用 `DOUBLE PRECISION`/`float64`，浮点数在金融计算中存在精度丢失（如 `0.1 + 0.2 ≠ 0.3`）。**建议**：迁移到 `NUMERIC(20,8)` 或整数最小单位，Go 侧使用 `shopspring/decimal`。    |
| A-18 | **ChatHandler 直接持有 gorm.DB** | `server/internal/api/handlers/chat_handler.go:43`        | Handler 层直接持有 `*gorm.DB` 引用并执行数据库查询，违反分层架构原则。**建议**：将 DB 操作移入对应的 Service/Repository。                                                                                                           |
| A-26 | **DSN 构建未转义特殊字符**            | `server/internal/config/config.go:560-563`               | `GetDSN()` 使用简单字符串拼接，密码中的特殊字符可能导致 DSN 解析失败。**建议**：使用 `net/url` 包正确转义或使用 `pgx` 的 Config 结构体。                                                                                                    |


### 中优先级问题


| #    | 问题描述                                      | 位置                          | 原因说明                                                                      |
| ---- | ----------------------------------------- | --------------------------- | ------------------------------------------------------------------------- |
| A-1  | Services 结构体膨胀为 God Object（30+ 字段）        | `routes.go:59-95`           | 新增服务需修改多处，牵一发而动全身。建议引入 DI 容器或按领域拆分子组。                                     |
| A-4  | Repository 层混合使用接口和具体类型                   | `main.go:446-470`           | 大多数字段用具体类型，仅 RoutingRule/Webhook 用接口。建议统一使用接口类型。                          |
| A-6  | LLM 端点三重注册导致路由表冗余                         | `routes.go:326-350`         | `/api/v1`、`/v1`、根路径各注册一次完整中间件链。建议用路径重写替代。                                 |
| A-7  | Anthropic 兼容路由嵌套错误                        | `routes.go:388-391`         | `parent.Group("/v1/messages")` 当 parent 为 `/v1` 时实际路径为 `/v1/v1/messages`。 |
| A-11 | 缺少复合索引优化高频查询                              | `migrations/000001:142-149` | 单列索引无法高效覆盖 `(project_id, created_at)` 组合查询。                               |
| A-13 | Redis 缓存键构建读写不一致                          | `billing.go:214-265`        | 写入用 org-level 键，读取尝试 project/channel 维度，导致缓存无法命中。                         |
| A-14 | 内存缓存缺乏淘汰机制                                | `router.go:79`              | `failedKeys` map 从不删除过期条目，高流量下内存泄漏。                                       |
| A-15 | 路由器 roundRobinIndex 是进程内状态                | `router.go:77`              | 多实例部署时各实例独立计数，Round Robin 失去均衡效果。                                         |
| A-16 | Webhook 分发无分布式锁                           | `main.go:340-351`           | 多实例部署时同一 Webhook 可能被重复投递。                                                 |
| A-17 | 模型发现缓存刷新存在 thundering herd 问题             | `router.go:167-199`         | 缓存失效时大量请求同时触发 DB 查询。建议使用 `singleflight`。                                  |
| A-19 | database.go 包含种子数据和清理逻辑                   | `database.go:134-443`       | 违反单一职责，应拆分到独立的 seeder/cleanup 包。                                          |
| A-20 | billing.RecordUsageAndDeduct 直接操作 User 模型 | `billing.go:143-178`        | 计费逻辑直接 GORM 操作用户余额，绕过 Repository 层。                                       |
| A-21 | 两套错误系统未统一使用                               | `errors.go` + `apierror.go` | RouterError、APIError、gqlgen 错误三种格式并存。                                     |
| A-23 | go-redis/redis/v8 已过时                     | `go.mod:12`                 | v8 已停止维护，应迁移到 `redis/go-redis/v9`。                                        |
| A-27 | JWT 认证每次请求查询数据库                           | `auth_middleware.go:106`    | `validateUserState` 每次调用 `GetByID`，高流量下压力大。建议添加短期缓存。                      |


---

## 二、文档管理审查

### 高优先级问题


| #     | 问题描述                                     | 位置                         | 原因说明                                                             |
| ----- | ---------------------------------------- | -------------------------- | ---------------------------------------------------------------- |
| D-2.1 | **Swagger/OpenAPI 规范文件完全为空**             | `server/docs/swagger.json` | `"paths": {}` 无任何端点定义，Swagger UI 无法展示文档。原因是 handler 缺少 swag 注解。  |
| D-7.1 | **CHANGELOG 没有任何已发布版本**                  | `CHANGELOG.md`             | 只有 `[Unreleased]` 段落，无版本号。项目有 release 工作流但 CHANGELOG 未同步。        |
| D-8.1 | **CONTRIBUTING.md 中 Git Clone URL 为占位符** | `CONTRIBUTING.md:16`       | `your-org` 未替换为实际组织名 `Veritas-Calculus`。                         |
| D-9.2 | **GEMINI.md 中 @auth 指令参数名错误**            | `GEMINI.md:73`             | 写 `@auth(requires: ROLE)`，实际为 `@auth(role: ROLE)`。AI 工具可能生成错误代码。 |
| D-9.3 | **GEMINI.md 暴露默认管理员明文密码**                | `GEMINI.md:119`            | `Admin Credentials: admin@example.com / DevAdmin123!` 可能被搜索引擎索引。 |
| D-4.1 | **多个环境变量未出现在文档中**                        | `environment-variables.md` | 9 个变量遗漏（PROXY_POOL_*, LOG_*, FRONTEND_URL, LOKI_URL 等）。          |
| D-5.1 | **两套 Helm Chart 未文档化**                   | `deploy/helm/` vs `helm/`  | 用户无法判断应使用哪一套。                                                    |


### 中优先级问题


| #      | 问题描述                                  | 位置                             | 原因说明                                                      |
| ------ | ------------------------------------- | ------------------------------ | --------------------------------------------------------- |
| D-2.2  | Anthropic 路由路径文档令人困惑                  | `api-reference.md:172`         | 仅列出 `/v1/v1/messages`，缺少更常用的路径说明。                         |
| D-2.3  | Models 端点路由文档与代码不一致                   | `api-reference.md:163`         | 文档写 `/v1/models/{model_id}`，实际为两段式 URL `/:org/*name`。     |
| D-2.4  | API Reference 缺少部分端点                  | `api-reference.md`             | captcha/config、SSO、审计导出、OAuth2 端点未收录。                     |
| D-4.2  | DB_SSL_MODE 默认值文档与 .env.example 不一致   | `environment-variables.md:47`  | 文档标注 `require`，.env.example 为 `prefer`。                   |
| D-4.3  | RATE_LIMIT_ENABLED 文档默认值与代码不一致        | `environment-variables.md:101` | 文档标注 `true`，但 config.go 无 SetDefault，Go bool 零值为 `false`。 |
| D-6.1  | GraphQL Schema 缺少字段级注释                | `schema/*.graphqls`            | 降低 Playground 和客户端自动补全的可用性。                               |
| D-6.2  | handler 函数缺少 Swagger 注解               | `handlers/*.go`                | swagger.json 为空的根本原因。                                     |
| D-9.1  | 文档语言不一致（中英文混合）                        | 多文件                            | README/docs 用中文，CONTRIBUTING/CHANGELOG 等用英文。              |
| D-9.4  | Graceful Degradation 中 SMTP 变量名与代码不一致 | `graceful-degradation.md:52`   | 文档写 `SMTP_HOST`，代码为 `EMAIL_SMTP_HOST`。                    |
| D-10.1 | Python 示例存在 .api_key_cache 缓存文件       | `examples/python/`             | 可能包含实际 API Key，应加入 .gitignore。                            |
| D-10.3 | Webhook 签名验证 Python 示例需补充说明           | `webhook-integration.md:74`    | 未说明 signature 参数应包含 `sha256=` 前缀。                         |
| D-5.2  | Helm Chart README 未提及子 Chart 依赖       | `deploy/helm/README.md`        | 文档与实际打包的子 Chart 矛盾。                                       |


---

## 三、部署与维护审查

### 高优先级问题


| #      | 问题描述                                      | 位置                                      | 原因说明                                                       |
| ------ | ----------------------------------------- | --------------------------------------- | ---------------------------------------------------------- |
| P-1.1  | **缺少 .dockerignore 文件**                   | `server/` 和 `web/`                      | `COPY . .` 会复制 .git、.env、node_modules 等敏感/冗余文件到镜像中。        |
| P-2.1  | **Redis 健康检查认证失败**                        | `docker-compose.yml:42`                 | `$REDIS_PASSWORD` 在容器内为空，健康检查因认证失败一直重试。需在 Redis 服务中添加环境变量。 |
| P-2.2  | **sentry_net 外部网络导致无 Sentry 时启动失败**       | `docker-compose.yml:584`                | 引用不存在的外部网络会导致 `docker compose up` 直接失败，即使不需要 Sentry。       |
| P-3.1  | **CI 测试用 postgres:16-alpine 而非 pgvector** | `.github/workflows/ci.yml:20`           | 生产用 `pgvector/pgvector:pg16`，CI 缺少 pgvector 扩展，相关测试可能失败。   |
| P-4.1  | **deploy/helm/ 缺少 Web 前端 Deployment**     | `deploy/helm/templates/`                | 只部署 Server，无前端 UI。旧 helm/ chart 有但未被维护。                    |
| P-5.1  | **迁移未集成到 Helm 部署流程**                      | `deploy/helm/templates/deployment.yaml` | K8s 部署新版本时需手动运行迁移。建议添加 init container 或 Helm hook。         |
| P-6.2  | **备份仅存储在本地 Docker Volume**                | `docker-compose.yml:512`                | 不满足 3-2-1 备份原则，宿主机损坏则备份全部丢失。                               |
| P-6.3  | **deploy/helm/ 没有任何备份机制**                 | `deploy/helm/`                          | 备份 CronJob 仅存在于旧的 helm/ chart 中。                           |
| P-7.1  | **values.yaml 镜像标签默认 latest**             | `deploy/helm/values.yaml:6`             | K8s 中使用 latest 导致不可重现部署和无法回滚。                              |
| P-8.1  | **Helm values.yaml 默认值偏向开发环境**            | `deploy/helm/values.yaml:95,115`        | `dbSslMode: disable`、`registrationMode: open` 不适合生产。       |
| P-10.1 | **两套独立 Helm Chart 共存**                    | `deploy/helm/` vs `helm/`               | 设计思路不同，功能互补但不兼容，维护成本高且易混淆。                                 |


### 中优先级问题


| #     | 问题描述                             | 位置                               | 原因说明                                           |
| ----- | -------------------------------- | -------------------------------- | ---------------------------------------------- |
| P-1.2 | Web Dockerfile 基础镜像未固定版本         | `web/Dockerfile:27`              | `nginx:alpine` 不同构建时间可能拉取不同版本。                 |
| P-2.3 | Loki/Promtail 未使用 Profile 隔离     | `docker-compose.yml:547`         | 每次 `docker compose up` 自动启动，且 Loki 暴露 3100 端口。 |
| P-2.4 | Promtail 挂载 Docker Socket 安全风险   | `docker-compose.yml:569`         | 只读标志不能阻止 Docker API 层面的写操作。                    |
| P-2.6 | Langfuse 默认密码/密钥过于简单             | `docker-compose.yml` 多处          | `ENCRYPTION_KEY` 全零填充尤其危险。                     |
| P-3.2 | 安全扫描结果被 `                        |                                  | true` 静默忽略                                     |
| P-3.4 | Release 工作流缺少 Docker 构建依赖        | `release.yml`                    | Helm chart 可能发布指向不存在镜像的引用。                     |
| P-4.2 | deploy/helm/ 缺少备份 CronJob        | `deploy/helm/templates/`         | 仅旧 chart 有备份功能。                                |
| P-4.3 | Helm Secret 默认值为空字符串             | `deploy/helm/values.yaml:175`    | 未设置时应用以空密钥启动，严重安全隐患。                           |
| P-4.5 | helm/ chart Ingress 暴露 /metrics  | `helm/templates/ingress.yaml:56` | 业务指标不应对公网开放。                                   |
| P-6.1 | Docker Compose 备份用 sleep 循环      | `docker-compose.yml:521`         | 无法精确控制备份时间，无重试和告警。                             |
| P-7.2 | Docker Compose 使用 :latest 标签     | `docker-compose.yml:61,143`      | Server/Web 镜像版本不确定。                            |
| P-7.3 | Prometheus/Grafana 使用 :latest 标签 | `docker-compose.yml:191,225`     | 可能因版本不同导致配置不兼容。                                |


---

## 四、监控运维审查

### 高优先级问题


| #     | 问题描述                            | 位置                           | 原因说明                                                                             |
| ----- | ------------------------------- | ---------------------------- | -------------------------------------------------------------------------------- |
| M-1.1 | **DB 连接池 Counter 指标未实际写入数据**    | `db_pool_metrics.go:69-81`   | `Collect()` 中 4 个 Counter 从未调用 `Add()`，始终为零。告警规则 `DBHighWaitCount` 依赖此数据但永远不会触发。 |
| M-3.1 | **Prometheus 未配置 Alertmanager** | `prometheus.yml`             | 缺少 `alerting:` 配置块，所有告警规则"写了不发"。                                                 |
| M-3.2 | **缺少 Alertmanager 路由/接收器配置**    | 无 `alertmanager.yml`         | 无法区分告警严重级别、无法路由到不同通知渠道。                                                          |
| M-6.1 | **Grafana 仪表盘数据源 UID 硬编码**      | `dashboards/llm-router.json` | 新环境 UID 不同导致全部面板无数据。应使用变量引用。                                                     |
| M-8.1 | **完全缺失 SLO 定义**                 | 全局                           | 无正式 SLO 文档或 recording rules，无法量化可靠性和 error budget。                               |


### 中优先级问题


| #     | 问题描述                           | 位置                          | 原因说明                               |
| ----- | ------------------------------ | --------------------------- | ---------------------------------- |
| M-1.3 | HTTP 层无 W3C Trace Context 传播   | `routes.go`                 | 未注册 otelgin 中间件，分布式追踪无法跨服务串联。      |
| M-2.1 | /healthz 将 Redis 不可用视为不健康(503) | `operational_handler.go:78` | Redis 是可选依赖，不可用时不应影响健康状态。          |
| M-3.3 | 告警阈值基于经验值而非 SLO                | `alert_rules.yml:17`        | 应从 SLO 推导 burn rate 告警。            |
| M-4.1 | 所有请求均以 Info 级别记录               | `logging_middleware.go:45`  | 高流量下 Info 日志量大，应按状态码分级。            |
| M-4.2 | Panic Recovery 缺少堆栈追踪          | `logging_middleware.go:131` | panic 发生时无法定位调用链。                  |
| M-5.2 | 缺少 Redis 连接池指标                 | 无对应文件                       | Redis 是核心依赖但无连接池/延迟指标。             |
| M-5.3 | 缺少上游 Provider 请求延迟指标           | 无对应文件                       | 无法区分 Router 处理时间 vs Provider 响应时间。 |
| M-6.2 | 缺少 DB 连接池面板                    | `dashboards/`               | 有指标和告警但无仪表盘面板。                     |
| M-6.3 | 缺少熔断器状态面板                      | `dashboards/`               | 无法直观查看 Provider 熔断状态。              |
| M-6.4 | 缺少 Loki 日志数据源配置                | `datasources/`              | 已部署 Loki 但 Grafana 未配置数据源。         |


---

## 五、安全审查

### 严重/高优先级问题


| #      | 问题描述                                         | 位置                      | 原因说明                                                                                                                  |
| ------ | -------------------------------------------- | ----------------------- | --------------------------------------------------------------------------------------------------------------------- |
| S-9.1  | **微信支付 Webhook 缺少 HTTP 签名验证**                | `wechat_pay.go:155-223` | 只解密了 AES-256-GCM 数据，未验证 `Wechatpay-Signature` HTTP 头。攻击者获取 APIv3 Key 可构造虚假通知。微信官方要求双重验证。**建议**：添加 SHA256withRSA 签名验证。 |
| S-10.1 | **ALLOW_LOCAL_PROVIDERS 默认 true 禁用 SSRF 保护** | `docker-compose.yml:95` | 私有 IP 检查和 DNS 重绑定保护全部被绕过。管理员账户被入侵时攻击者可探测内网（如云元数据 `169.254.169.254`）。**建议**：默认改为 `false`，仅在 dev 覆盖文件中设为 `true`。        |


### 中优先级问题


| #     | 问题描述                               | 位置                      | 原因说明                                                     |
| ----- | ---------------------------------- | ----------------------- | -------------------------------------------------------- |
| S-9.2 | 支付宝通知签名验证为条件性执行                    | `alipay_service.go:177` | 公钥未配置时通知不经验签即被处理，攻击者可伪造充值通知。                             |
| S-2.1 | 根 .env 含疑似真实 Langfuse API 密钥       | `.env:26-28`            | `sk-lf-` 前缀格式与真实密钥一致，应轮换。                                |
| S-3.1 | DLP 自定义正则表达式存在 ReDoS 风险            | `dlp_service.go:69-80`  | 恶意正则（如 `(a+)+$`）可导致 CPU 耗尽。在 LLM 请求路径上影响广泛。              |
| S-4.1 | PerKey/PerUser 速率限制器 Redis 为空时完全跳过 | `ratelimit.go:47-50`    | Redis 宕机时所有 API Key/用户级限制失效，与 AuthRateLimiter（有内存回退）不一致。 |


### 低优先级问题


| #      | 问题描述                                  | 位置                      | 原因说明                                               |
| ------ | ------------------------------------- | ----------------------- | -------------------------------------------------- |
| S-1.1  | Refresh Token 未校验 TokensInvalidatedAt | `helpers_auth.go:59-83` | 令牌吊销后 Refresh Token 仍可用于获取新 Access Token。          |
| S-2.2  | 辅助服务使用弱默认密码                           | `docker-compose.yml` 多处 | Langfuse ENCRYPTION_KEY 全零尤其危险。                    |
| S-5.1  | 开发 .env 中 DB SSL 为 disable            | `server/.env:14`        | 开发可接受，但 docker-compose.yml 默认也为 disable。           |
| S-13.1 | Nginx 子 location 安全头继承缺失              | `nginx.conf:70-74`      | `location = /index.html` 中定义 add_header 后，父级安全头丢失。 |


### 安全亮点

- Docker 容器非 root 运行 + 只读文件系统 + no-new-privileges
- AES-256-GCM 加密实现正确（随机 nonce、GCM 认证）
- CI/CD 多层安全扫描（CodeQL + gosec + Trivy + Semgrep + govulncheck）
- Actions 使用 commit SHA 固定，防止供应链攻击
- SSRF 防护架构全面（私有 IP + DNS 重绑定保护）
- Stripe Webhook 签名验证正确
- 审计日志体系完善（覆盖关键操作 + CSV 导出 + 自动清理）

---

## 六、跨维度优先修复路线图

### P0 — 立即修复（影响正确性或安全性）


| 问题     | 维度  | 描述                                 |
| ------ | --- | ---------------------------------- |
| S-9.1  | 安全  | 微信支付 Webhook 添加 HTTP 签名验证          |
| S-10.1 | 安全  | 将 ALLOW_LOCAL_PROVIDERS 默认改为 false |
| S-9.2  | 安全  | 支付宝签名验证改为强制性                       |
| A-10   | 架构  | 金额字段从 float64 迁移到 NUMERIC/decimal  |
| P-2.1  | 部署  | 修复 Redis 健康检查认证失败                  |
| P-2.2  | 部署  | 修复 sentry_net 外部网络硬依赖              |
| M-1.1  | 监控  | 完成 DB 连接池指标 Counter 的实际采集          |
| M-3.1  | 监控  | 配置 Alertmanager 接收地址和路由            |


### P1 — 本周完成（影响功能完整性）


| 问题     | 维度  | 描述                              |
| ------ | --- | ------------------------------- |
| P-10.1 | 部署  | 合并两套 Helm Chart，删除旧 helm/ 目录    |
| P-1.1  | 部署  | 添加 .dockerignore 文件             |
| P-3.1  | 部署  | CI Postgres 镜像改用 pgvector       |
| P-4.1  | 部署  | Helm chart 补充 Web 前端 Deployment |
| P-5.1  | 部署  | 迁移集成到 K8s 部署流程                  |
| A-3    | 架构  | 统一迁移策略，移除 AutoMigrate           |
| D-2.1  | 文档  | 生成 Swagger/OpenAPI 规范           |
| D-8.1  | 文档  | 修复 CONTRIBUTING.md clone URL    |
| D-9.3  | 文档  | 移除 GEMINI.md 中的明文密码             |
| S-2.1  | 安全  | 轮换 .env 中的 Langfuse 密钥          |
| M-6.1  | 监控  | 修复 Grafana 数据源 UID 硬编码          |


### P2 — 本月完成（改善可维护性和可扩展性）


| 问题    | 维度  | 描述                                 |
| ----- | --- | ---------------------------------- |
| A-9   | 架构  | usage_logs 添加分区策略                  |
| A-13  | 架构  | 统一 Redis 缓存键读写逻辑                   |
| A-17  | 架构  | 使用 singleflight 防止 thundering herd |
| A-18  | 架构  | ChatHandler 移除直接 DB 依赖             |
| P-7.1 | 部署  | 修复镜像标签 latest 问题                   |
| P-8.1 | 部署  | Helm 默认值改为生产安全级别                   |
| M-8.1 | 监控  | 定义正式 SLO 并实施 burn rate 告警          |
| M-6.4 | 监控  | 添加 Loki 数据源到 Grafana               |
| D-4.1 | 文档  | 补充遗漏的环境变量文档                        |
| D-9.1 | 文档  | 统一文档语言策略                           |
| S-3.1 | 安全  | DLP ReDoS 防护                       |
| S-4.1 | 安全  | 速率限制器 Redis nil 时添加内存回退            |


### P3 — 持续改进（技术债务和优化）


| 问题    | 维度  | 描述                     |
| ----- | --- | ---------------------- |
| A-1   | 架构  | Services 结构体拆分/引入 DI   |
| A-14  | 架构  | 内存缓存添加淘汰机制             |
| A-23  | 架构  | 升级 go-redis 到 v9       |
| P-6.2 | 部署  | 备份上传外部存储               |
| P-9.1 | 部署  | Helm 子 chart 依赖更新监控    |
| M-5.2 | 监控  | 添加 Redis 连接池指标         |
| M-5.3 | 监控  | 添加上游 Provider 延迟指标     |
| D-7.1 | 文档  | CHANGELOG 添加版本记录       |
| D-6.1 | 文档  | GraphQL Schema 添加字段级注释 |


---

*本报告由 5 个专业 Agent 并行审查生成，仅记录问题和建议，未直接修改任何代码或配置。*