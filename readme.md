# VC LLM Router Platform

一个统一的大语言模型 (LLM) 中转路由平台，支持多平台接入、代理池管理、计费统计和会话记忆功能。

## 功能特性

### 核心功能

- **统一 API 接口**: 提供与 OpenAI 兼容的 API 格式，一套代码对接多个 LLM 平台
- **多平台支持**: 集成 OpenAI、Claude、Gemini 等主流 LLM 服务商
- **代理池管理**: 支持自定义 Proxy Pool 配置，实现请求负载均衡和故障转移
- **智能路由**: 根据模型、成本、延迟等因素自动选择最优服务商

### 计费功能

- 按 Token 用量计费统计
- 支持多用户/多租户计费隔离
- 用量报表和消费明细查询
- 预算控制和额度告警

### 会话记忆

- 保存历史对话上下文
- 支持会话恢复和续写
- 跨设备会话同步

### 健康检查

- **API Key 可用性检测**: 定时验证各平台 API Key 是否有效、余额是否充足
- **代理池健康监测**: 检测 Proxy Pool 中各节点的连通性和响应延迟
- **自动故障转移**: 当检测到 API Key 失效或代理节点不可用时，自动切换备用资源
- **告警通知**: 支持邮件、Webhook、钉钉/飞书等多渠道告警
- **健康状态面板**: 可视化展示各资源的健康状态和历史可用率

### 管理后台 (Apple Design Style)

- **Dashboard 概览**: 实时数据可视化，简洁直观的数据展示
- **用量统计**: Token 消耗趋势图、调用次数统计、响应时间分析
- **费用报表**: 按日/周/月维度的费用统计，支持导出 CSV/PDF
- **用户管理**: API Key 管理、配额设置、权限控制
- **系统监控**: 服务健康状态、请求成功率、错误日志追踪
- **健康检查**: API Key 状态、代理池节点状态、实时告警信息

## 技术架构

```
                                    +------------------+
                                    |   LLM Providers  |
                                    |  (OpenAI/Claude) |
                                    +--------+---------+
                                             |
+------------------+    +------------------+ | +------------------+
|                  |    |                  | | |                  |
|  Web Dashboard   +--->+   API Gateway   +-+-+    Proxy Pool    |
|  (React + Vite)  |    |     (Go Gin)    |   |                  |
|                  |    |                  |   +------------------+
+------------------+    +--------+---------+
                                 |
                    +------------+------------+
                    |            |            |
              +-----+----+ +-----+----+ +-----+----+
              |          | |          | |          |
              | Billing  | |  Memory  | |  Router  |
              |  Module  | |  Module  | |  Module  |
              |          | |          | |          |
              +-----+----+ +-----+----+ +----------+
                    |            |
                    +------+-----+
                           |
                    +------+------+
                    |             |
                    | PostgreSQL  |
                    |    Redis    |
                    |             |
                    +-------------+
```

## 技术栈

### 后端 (Go)

| 技术 | 用途 |
|------|------|
| Go 1.21+ | 主开发语言 |
| Gin | Web 框架 |
| GORM | ORM 数据库操作 |
| go-redis | Redis 客户端 |
| zap | 日志库 |
| viper | 配置管理 |
| swaggo | API 文档生成 |

### 前端 (React)

| 技术 | 用途 |
|------|------|
| React 18 | UI 框架 |
| TypeScript | 类型安全 |
| Vite | 构建工具 |
| TailwindCSS | 样式框架 |
| Framer Motion | 动画效果 |
| Recharts | 图表可视化 |
| React Query | 数据请求 |
| Zustand | 状态管理 |

### 前端设计规范 (Apple Style)

- **配色**: 采用 Apple 风格的中性色调，大量留白
  - 主色: #007AFF (Apple Blue)
  - 背景: #F5F5F7 (Light Gray)
  - 文字: #1D1D1F (Primary), #86868B (Secondary)
- **圆角**: 大圆角设计 (12px - 20px)
- **阴影**: 柔和的阴影效果，层次分明
- **字体**: SF Pro Display / SF Pro Text (或 Inter 作为 Web 替代)
- **动效**: 平滑过渡动画，注重交互反馈
- **布局**: 简洁的卡片式布局，清晰的信息层级

## 快速开始

### 环境要求

- Go >= 1.21
- Node.js >= 18.x
- PostgreSQL >= 14
- Redis >= 6.0

### 后端安装

```bash
# 克隆仓库
git clone https://github.com/Veritas-Calculus/llm-router-platform.git
cd llm-router-platform

# 进入后端目录
cd server

# 下载依赖
go mod download

# 配置环境变量
cp .env.example .env

# 运行数据库迁移
go run cmd/migrate/main.go

# 启动服务
go run cmd/server/main.go
```

### 前端安装

```bash
# 进入前端目录
cd web

# 安装依赖
pnpm install

# 开发模式
pnpm dev

# 生产构建
pnpm build
```

### Docker 一键部署

```bash
# 使用 docker-compose 启动所有服务
docker-compose up -d
```

### 配置说明

后端配置 `server/.env`:

```env
# 服务配置
SERVER_PORT=8080
GIN_MODE=release

# 数据库连接
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=llm_router

# Redis 配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# LLM Provider API Keys
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_API_KEY=xxx

# 代理池配置
PROXY_POOL_ENABLED=true
PROXY_POOL_URL=http://proxy-pool:8080

# 健康检查配置
HEALTH_CHECK_ENABLED=true
HEALTH_CHECK_INTERVAL=60           # 检查间隔(秒)
HEALTH_CHECK_TIMEOUT=10            # 超时时间(秒)
HEALTH_CHECK_RETRY_COUNT=3         # 失败重试次数
HEALTH_CHECK_FAILURE_THRESHOLD=3   # 连续失败阈值，超过则标记不可用

# 告警配置
ALERT_ENABLED=true
ALERT_WEBHOOK_URL=https://your-webhook-url
ALERT_EMAIL_ENABLED=false
ALERT_EMAIL_SMTP_HOST=smtp.example.com
ALERT_EMAIL_SMTP_PORT=587
ALERT_EMAIL_FROM=alert@example.com
ALERT_EMAIL_TO=admin@example.com

# JWT 密钥
JWT_SECRET=your-jwt-secret-key
```

前端配置 `web/.env`:

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_APP_TITLE=LLM Router Platform
```

## API 文档

### 基础请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### 支持的端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/v1/chat/completions` | POST | 对话补全 |
| `/v1/completions` | POST | 文本补全 |
| `/v1/embeddings` | POST | 文本向量化 |
| `/v1/models` | GET | 获取可用模型列表 |
| `/v1/usage` | GET | 查询用量统计 |
| `/api/dashboard/stats` | GET | Dashboard 统计数据 |
| `/api/reports/usage` | GET | 用量报表 |
| `/api/reports/billing` | GET | 计费报表 |
| `/api/users` | GET/POST | 用户管理 |
| `/api/api-keys` | GET/POST/DELETE | API Key 管理 |
| `/api/health` | GET | 系统整体健康状态 |
| `/api/health/api-keys` | GET | 所有 API Key 健康状态 |
| `/api/health/api-keys/:id` | GET | 单个 API Key 健康检测 |
| `/api/health/api-keys/:id/check` | POST | 手动触发 API Key 检测 |
| `/api/health/proxies` | GET | 代理池节点健康状态 |
| `/api/health/proxies/:id/check` | POST | 手动触发代理节点检测 |
| `/api/health/history` | GET | 健康检查历史记录 |
| `/api/alerts` | GET | 告警列表 |
| `/api/alerts/config` | GET/PUT | 告警配置 |

## 项目结构

```
llm-router-platform/
├── server/                    # Go 后端
│   ├── cmd/
│   │   ├── server/           # 主程序入口
│   │   └── migrate/          # 数据库迁移
│   ├── internal/
│   │   ├── api/              # HTTP handlers
│   │   │   ├── handlers/     # 请求处理器
│   │   │   ├── middleware/   # 中间件
│   │   │   └── routes/       # 路由定义
│   │   ├── config/           # 配置管理
│   │   ├── models/           # 数据模型
│   │   ├── repository/       # 数据访问层
│   │   ├── service/          # 业务逻辑层
│   │   │   ├── billing/      # 计费服务
│   │   │   ├── memory/       # 会话记忆
│   │   │   ├── provider/     # LLM 服务商适配
│   │   │   ├── proxy/        # 代理池管理
│   │   │   ├── router/       # 路由策略
│   │   │   ├── health/       # 健康检查服务
│   │   │   └── alert/        # 告警通知服务
│   │   └── pkg/              # 公共工具包
│   ├── go.mod
│   └── go.sum
│
├── web/                       # React 前端
│   ├── src/
│   │   ├── components/       # 通用组件
│   │   │   ├── ui/           # 基础 UI 组件
│   │   │   ├── charts/       # 图表组件
│   │   │   └── layout/       # 布局组件
│   │   ├── pages/            # 页面
│   │   │   ├── Dashboard/    # 仪表盘
│   │   │   ├── Usage/        # 用量统计
│   │   │   ├── Billing/      # 费用报表
│   │   │   ├── Users/        # 用户管理
│   │   │   ├── ApiKeys/      # API Key 管理
│   │   │   ├── Health/       # 健康检查
│   │   │   ├── Alerts/       # 告警管理
│   │   │   └── Settings/     # 系统设置
│   │   ├── hooks/            # 自定义 Hooks
│   │   ├── stores/           # 状态管理
│   │   ├── services/         # API 服务
│   │   ├── styles/           # 全局样式
│   │   ├── types/            # TypeScript 类型
│   │   └── utils/            # 工具函数
│   ├── public/
│   ├── index.html
│   ├── package.json
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   └── vite.config.ts
│
├── docker/                    # Docker 配置
│   ├── Dockerfile.server
│   ├── Dockerfile.web
│   └── nginx.conf
│
├── docker-compose.yml
├── Makefile
└── README.md
```

## 开发计划

- [ ] 支持更多 LLM 服务商 (Mistral, 通义千问, 文心一言等)
- [x] 管理后台 Dashboard
- [x] API Key 权限管理
- [ ] 请求速率限制
- [ ] WebSocket 实时流式响应优化
- [ ] Kubernetes 部署支持
- [ ] 多语言国际化 (i18n)
- [ ] 深色模式支持
- [ ] 移动端适配

## 贡献指南

欢迎提交 Issue 和 Pull Request。请确保：

1. 后端代码通过 `golangci-lint` 检查
2. 前端代码通过 `ESLint` 和 `Prettier` 检查
3. 添加必要的测试用例
4. 更新相关文档

## 许可证

MIT License

