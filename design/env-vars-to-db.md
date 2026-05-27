# PR #68 新增环境变量 — 哪些可以搬到管理后台

> 评估目标：把可在运行时调整的配置从 env 搬到 DB 里的 `SystemSettings`，让运维不用重启容器、不用接触 `.env` 就能改。安全敏感的 secret 仍然走 env。

## 现状（10 个新增变量 / build args）

| # | 变量 | 默认值 | 当前位置 | 谁读 |
|---|---|---|---|---|
| 1 | `COOKIE_SECURE_MODE` | `auto` | server env | `helpers_auth.go:refreshCookieSecure` |
| 2 | `REGISTRATION_MODE` | `open` | server env | `auth.resolvers.go:Register` |
| 3 | `PROVIDER_SYNC_AUTO_ACTIVATE` | `false` | server env | `provider_ops.go:SyncProviderModels` |
| 4 | `PROVIDER_SYNC_BLOCKLIST_REGEX` | (built-in) | server env | `model_classification.go:newCatalogSyncRules` |
| 5 | `CAPTCHA_PROVIDER` | `dev` | server env | `captcha/captcha.go:New` |
| 6 | `CAPTCHA_SITE_KEY` | (empty) | server env | `captcha/captcha.go:New` (转给 hcaptcha/turnstile) |
| 7 | `CAPTCHA_SECRET_KEY` | (empty) | server env | `captcha/captcha.go:New` (转给 hcaptcha/turnstile) |
| 8 | `DEV_CAPTCHA_BYPASS_TOKEN` | `dev-ok` | server env | `captcha/captcha.go:New` |
| 9 | `VITE_CAPTCHA_PROVIDER` | `dev` | **build-arg** | 前端 widget 选择 |
| 10 | `VITE_CAPTCHA_SITE_KEY` | (empty) | **build-arg** | 前端 widget render |
| (+) | `VITE_DEV_CAPTCHA_BYPASS_TOKEN` | `dev-ok` | **build-arg** | 前端 dev stub |

`COOKIE_SECURE_MODE`、`CAPTCHA_PROVIDER`、`CAPTCHA_SITE_KEY` 等几个其实在 Round 7 已经从 build-arg 变成 runtime — 它们通过 `web/public/runtime-config.template.js` + `envsubst` 在容器启动时注入到 `/runtime-config.js`，前端读 `window.__RUNTIME_CONFIG__`。所以 VITE_* 不是 build-time hard-bake，是 runtime envsubst。

## 三类分桶

### 🟢 第一类 — 完全可搬（5 个）

**这些都是「运维想随时调整、调整后不该重启服务」的策略型配置**。直接进 `SystemSettings.security` 或新增 `SystemSettings.captcha` JSON category。

| 变量 | 搬到哪 | 为什么 |
|---|---|---|
| `REGISTRATION_MODE` | `SystemSettings.security.registrationMode` | 运维想从 `open` 切到 `invite` 或 `closed` 来快速止血滥用，不该重启服务。schema 顶层已有 `registrationMode: String!` 字段。 |
| `PROVIDER_SYNC_AUTO_ACTIVATE` | `SystemSettings.defaults.providerSyncAutoActivate` | 新模型自动启用是个运营决策，admin 在 UI 里勾选 checkbox 比 SSH 改 env 友好。 |
| `PROVIDER_SYNC_BLOCKLIST_REGEX` | `SystemSettings.defaults.providerSyncBlocklistRegex` | 同上，且正则是运营内容审核策略，应该可视化编辑+回滚。 |
| `CAPTCHA_PROVIDER` | `SystemSettings.security.captcha.provider` | 在线切换 captcha 提供商（如发现 turnstile 故障切到 hcaptcha）不应该需要重启。 |
| `COOKIE_SECURE_MODE` | `SystemSettings.security.cookieSecureMode` | 跨环境部署时 TLS terminator 配置可能临时改变，admin 紧急调整不应该重启。 |

**实现成本**：~150 LoC（一个 migration 改 default、若干 resolver 改读 DB、Admin Settings 加 UI 字段、缓存 5 分钟避免每次请求查 DB）。

### 🟡 第二类 — 必须留在 env（3 个 — 都是 secret）

| 变量 | 为什么不该搬 |
|---|---|
| `CAPTCHA_SECRET_KEY` | **secret**：hcaptcha/turnstile 的 server-side secret。即便加密存 DB 也比环境变量更易泄漏（admin UI 渲染会经过更多层），且 docker/k8s 已经有成熟的 secret 管理。 |
| `CAPTCHA_SITE_KEY` | **半 secret**：public key，但和 secret key 强绑定，分开管理会导致 mismatch。维持成对管理在 env 里更省心。 |
| `DEV_CAPTCHA_BYPASS_TOKEN` | dev-only 旁路 token，生产用不到；本来就该是 env 级别（部署管线里就能控制是否注入）。 |

### 🔵 第三类 — 可搬但建议留下（2 个 build args / runtime config）

| 变量 | 建议 |
|---|---|
| `VITE_CAPTCHA_PROVIDER` | 前端 widget 类型必须在容器启动时已知，否则 React 组件不知道渲染哪个 widget。Round 7 已经搬到 `/runtime-config.js`，**实际上就是 runtime config 注入**。让它从 `SystemSettings` 反向同步出来更优雅，但工程量大；当前的 envsubst 已经足够。 |
| `VITE_CAPTCHA_SITE_KEY` | 同上。前端 widget 初始化时需要这个值，envsubst 已经处理。 |

## 推荐方案

**A. 搬第一类 5 个变量到 `SystemSettings`（推荐）**

工作量：~150 LoC、1 个新 migration。

收益：
- env 减少 5 条
- 运维可以**热切换** registration 模式、captcha provider、blocklist 正则
- 配置变更进 audit log（已有 `audit_logs` 表）
- 多个实例部署时不用每个都改 env，DB 自动同步

代价：
- 服务启动时如果 DB 不可用，走 fallback 到代码里的硬编码默认值（已有这个 pattern，看 `SiteConfig` 公开查询）
- captcha provider 切换需要 reload captcha service 实例（一个 sync.Mutex + atomic.Pointer 就够）

**B. 只搬「最常调整」的 2 个：`REGISTRATION_MODE` + `PROVIDER_SYNC_AUTO_ACTIVATE`**

工作量：~60 LoC。最高频改动的两个搬走，其他 3 个留在 env 等真的有需求再搬。

**C. 不搬，只补全 docs/environment-variables.md**

工作量：15 分钟。承认运维就该用 env 文件，不增加运行时复杂度。
