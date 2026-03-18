// Package routes defines API routes.
package routes

import (
	"database/sql"
	"net/http/pprof"

	"llm-router-platform/internal/api/handlers"
	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/billing"
	configService "llm-router-platform/internal/service/config"
	"llm-router-platform/internal/service/email"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/mcp"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/task"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"

	// Import swagger docs
	_ "llm-router-platform/docs"
)

// Build information set via -ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Services holds all service dependencies.
type Services struct {
	User          *user.Service
	Router        *router.Router
	Billing       *billing.Service
	BudgetService *billing.BudgetService
	Subscription  *billing.SubscriptionService
	Payment       *billing.PaymentService
	Balance       *billing.BalanceService
	SystemConfig  *configService.Service
	Health        *health.Service
	Memory        *memory.Service
	MCP           *mcp.Service
	Observability observability.Service
	Proxy         *proxy.Service
	Provider      *provider.Registry
	TaskService   *task.Service
	AuditService  *audit.Service
	RedisClient   *redis.Client // For rate limiting
	DB            *gorm.DB      // For operational health checks only
}

// Setup configures all API routes.
func Setup(
	engine *gin.Engine,
	cfg *config.Config,
	services *Services,
	logger *zap.Logger,
) {
	// Create Prometheus metrics
	metricsCollector := middleware.NewMetrics()

	// Middleware chain (order matters):
	// 1. Request ID (first, so all downstream middleware can use it)
	// 2. Metrics (records timing/counters)
	// 3. CORS
	// 4. Logging (includes request_id)
	// 5. Recovery
	requestIDMiddleware := middleware.NewRequestIDMiddleware(logger)
	corsMiddleware := middleware.NewCORSMiddleware(cfg.Server.CORSOrigins)
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)
	recoveryMiddleware := middleware.NewRecoveryMiddleware(logger)

	engine.Use(requestIDMiddleware.Handle())
	engine.Use(metricsCollector.Middleware())
	engine.Use(middleware.SecurityHeaders())
	engine.Use(middleware.BodySizeLimit(10 << 20)) // 10 MB hard limit
	engine.Use(corsMiddleware.Handle())
	engine.Use(loggingMiddleware.Log())
	engine.Use(recoveryMiddleware.Recover())

	// Request body size limit (10MB) — prevents OOM from oversized payloads
	engine.MaxMultipartMemory = 10 << 20 // 10 MB

	// ─── Operational Endpoints ─────────────────────────────────────────
	opsHandler := handlers.NewOperationalHandler(services.DB, services.RedisClient, Version, GitCommit, BuildTime)
	engine.GET("/health", opsHandler.Liveness)
	engine.GET("/healthz", opsHandler.DeepHealth)
	engine.GET("/readyz", opsHandler.Readiness)
	engine.GET("/version", opsHandler.Version)

	// ─── Auth & Rate Limiter middleware (created early for /metrics guard) ──
	authMiddleware := middleware.NewAuthMiddleware(&cfg.JWT, services.User, logger)

	// Prometheus metrics endpoint — admin only to prevent info leakage
	metricsGroup := engine.Group("/metrics")
	metricsGroup.Use(authMiddleware.JWT())
	metricsGroup.Use(middleware.AdminOnly())
	metricsGroup.GET("", middleware.MetricsHandler())

	// Swagger API Docs — disabled in release/production mode
	if cfg.Server.Mode != "release" {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		logger.Info("swagger docs enabled at /swagger/ (non-release mode)")
	}

	// ─── Rate Limiter middleware ──────────────────────────────────────
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.RequestsPerMinute, services.RedisClient, logger)
	authorLimiter := middleware.NewAuthRateLimiter(services.RedisClient, 5, logger)

	// Per-key and per-user rate limiters
	perKeyLimiter := middleware.NewPerKeyRateLimiter(services.RedisClient, logger)
	perUserLimiter := middleware.NewPerUserRateLimiter(services.RedisClient, cfg.RateLimit.RequestsPerMinute, logger)
	quotaChecker := middleware.NewQuotaChecker(services.RedisClient, logger)

	// ─── Backpressure middleware ──────────────────────────────────────
	var sqlDB *sql.DB
	if services.DB != nil {
		sqlDB, _ = services.DB.DB()
	}
	backpressureLimiter := middleware.NewBackpressure(sqlDB, logger)

	// pprof debug endpoints (opt-in via PPROF_ENABLED=true, always requires admin auth)
	if cfg.Server.PprofEnabled {
		pprofGroup := engine.Group("/debug/pprof")
		pprofGroup.Use(authMiddleware.JWT())
		pprofGroup.Use(middleware.AdminOnly())
		{
			pprofGroup.GET("/", gin.WrapF(pprof.Index))
			pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
			pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
			pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
			pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
			pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
			pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
			pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
			pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
			pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
		}
		logger.Info("pprof debug endpoints enabled at /debug/pprof/ (admin auth required)")
	}

	// ─── API Routes ────────────────────────────────────────────────────
	emailService := email.NewService(cfg.Email, cfg.Frontend.URL)

	authHandler := handlers.NewAuthHandler(services.User, services.AuditService, emailService, &cfg.JWT, cfg.Registration.Mode, cfg.Registration.InviteCode, services.RedisClient, services.DB, logger)
	apiKeyHandler := handlers.NewAPIKeyHandler(services.User, logger)
	chatHandler := handlers.NewChatHandler(services.Router, services.Billing, services.Memory, services.Subscription, services.Balance, services.Observability, logger)
	modelHandler := handlers.NewModelHandler(services.Router, services.Provider, logger)
	usageHandler := handlers.NewUsageHandler(services.Billing, logger)
	healthHandler := handlers.NewHealthHandler(services.Health, logger)
	alertHandler := handlers.NewAlertHandler(services.Health, logger)
	dashboardHandler := handlers.NewDashboardHandler(services.Billing, services.Health, services.Router, services.User, logger)
	proxyHandler := handlers.NewProxyHandler(services.Proxy, logger)
	providerHandler := handlers.NewProviderHandler(services.Router, services.Health, logger)
	finopsHandler := handlers.NewFinOpsHandler(services.Billing, services.BudgetService, logger)
	userHandler := handlers.NewUserHandler(services.User, services.Billing, services.RedisClient, logger)
	mcpHandler := handlers.NewMCPHandler(services.MCP, logger)
	planHandler := handlers.NewPlanHandler(services.Subscription, logger)
	paymentHandler := handlers.NewPaymentHandler(services.Payment, logger)
	configHandler := handlers.NewConfigHandler(services.SystemConfig, logger)

	// Task handler (optional: only created if task service is wired)
	var taskHandler *handlers.TaskHandler
	if services.TaskService != nil {
		taskHandler = handlers.NewTaskHandler(services.TaskService, logger)
	}

	// Shared middleware chain for all LLM API endpoints.
	applyLLMMiddleware := func(g *gin.RouterGroup) {
		g.Use(authMiddleware.APIKey())
		g.Use(perKeyLimiter.Limit())
		g.Use(quotaChecker.Check())
		g.Use(rateLimiter.Limit())
		g.Use(backpressureLimiter.Protect())
	}

	api := engine.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			auth := v1.Group("/auth")
			auth.Use(authorLimiter.Limit())
			{
				auth.POST("/register", authHandler.Register)
				auth.POST("/login", authHandler.Login)
				auth.POST("/token/rotate", authHandler.RotateRefreshToken) // Refresh token rotation (validates refresh token from body)
				auth.POST("/forgot-password", authHandler.ForgotPassword)
				auth.POST("/reset-password", authHandler.ResetPassword)
			}

			// ── All authenticated users ─────────────────────────
			protected := v1.Group("")
			protected.Use(authMiddleware.JWT())
			protected.Use(perUserLimiter.Limit())
			{
				// Token refresh (requires valid JWT — H4 security fix)
				protected.POST("/auth/refresh", authHandler.RefreshToken)

				// M4: Logout — invalidates all tokens for the current user
				protected.POST("/auth/logout", authHandler.Logout)

				// Personal profile
				user := protected.Group("/user")
				{
					user.GET("/profile", authHandler.GetProfile)
					user.PUT("/profile", authHandler.UpdateProfile)
					user.PUT("/password", authHandler.ChangePassword)
				}

				// Personal API keys (scoped to user_id)
				keys := protected.Group("/api-keys")
				{
					keys.GET("", apiKeyHandler.List)
					keys.POST("", apiKeyHandler.Create)
					keys.POST("/:id/revoke", apiKeyHandler.Revoke)
					keys.DELETE("/:id", apiKeyHandler.Delete)
				}

				// Personal usage (scoped to user_id)
				usage := protected.Group("/usage")
				{
					usage.GET("/summary", usageHandler.GetSummary)
					usage.GET("/daily", usageHandler.GetDaily)
					usage.GET("/by-provider", usageHandler.GetByProvider)
					usage.GET("/recent", usageHandler.GetRecent)
					usage.GET("/export/csv", finopsHandler.ExportUsage)
				}

				// Personal budget & anomaly detection
				finops := protected.Group("/finops")
				{
					finops.PUT("/budget", finopsHandler.SetBudget)
					finops.GET("/budget", finopsHandler.GetBudget)
					finops.DELETE("/budget", finopsHandler.DeleteBudget)
					finops.GET("/budget/status", finopsHandler.GetBudgetStatus)
					finops.GET("/anomaly", finopsHandler.DetectAnomaly)
				}

				// Async task management
				if taskHandler != nil {
					tasks := protected.Group("/tasks")
					{
						tasks.POST("", taskHandler.CreateTask)
						tasks.GET("", taskHandler.ListTasks)
						tasks.GET("/:id", taskHandler.GetTask)
						tasks.POST("/:id/cancel", taskHandler.CancelTask)
					}
				}

				// Dashboard stats (returns per-user or system data depending on role)
				dashboard := protected.Group("/dashboard")
				{
					dashboard.GET("/stats", dashboardHandler.GetStats)
					dashboard.GET("/overview", dashboardHandler.GetOverview)
					dashboard.GET("/usage-chart", dashboardHandler.GetUsageChart)
					dashboard.GET("/provider-stats", dashboardHandler.GetProviderStats)
					dashboard.GET("/model-stats", dashboardHandler.GetModelStats)
				}

				// Plans & Subscriptions
				plans := protected.Group("/plans")
				{
					plans.GET("", planHandler.ListPlans)
					plans.GET("/my", planHandler.GetMySubscription)
					plans.POST("/checkout", paymentHandler.CreateCheckoutSession)
					plans.POST("/recharge", paymentHandler.CreateRechargeSession)
					plans.GET("/orders", paymentHandler.GetMyOrders)
				}
			}

			// ── Public / Webhooks ──────────────────────────────
			v1.POST("/payments/webhook/stripe", paymentHandler.StripeWebhook)

			// ── Admin only ──────────────────────────────────────
			admin := v1.Group("")
			admin.Use(authMiddleware.JWT())
			admin.Use(middleware.AdminOnly())
			{
				// User management
				users := admin.Group("/users")
				{
					users.GET("", userHandler.List)
					users.GET("/:id", userHandler.GetUser)
					users.GET("/:id/usage", userHandler.GetUserUsage)
					users.GET("/:id/api-keys", userHandler.GetUserAPIKeys)
					users.POST("/:id/toggle", userHandler.ToggleUser)
					users.PUT("/:id/role", userHandler.UpdateRole)
					users.PUT("/:id/quota", userHandler.UpdateQuota)
				}

				// System health monitoring (admin only)
				healthGroup := admin.Group("/health")
				{
					healthGroup.GET("/api-keys", healthHandler.GetAPIKeysHealth)
					healthGroup.POST("/api-keys/:id/check", healthHandler.CheckAPIKey)
					healthGroup.GET("/proxies", healthHandler.GetProxiesHealth)
					healthGroup.POST("/proxies/:id/check", healthHandler.CheckProxy)
					healthGroup.GET("/providers", healthHandler.GetProvidersHealth)
					healthGroup.POST("/providers/:id/check", healthHandler.CheckProvider)
					healthGroup.POST("/providers/check-all", healthHandler.CheckAllProviders)
					healthGroup.GET("/history", healthHandler.GetHealthHistory)
				}

				// Alert management (admin only)
				alerts := admin.Group("/alerts")
				{
					alerts.GET("", alertHandler.List)
					alerts.POST("/:id/acknowledge", alertHandler.Acknowledge)
					alerts.POST("/:id/resolve", alertHandler.Resolve)
					alerts.GET("/config/:target_type/:target_id", alertHandler.GetConfig)
					alerts.PUT("/config", alertHandler.UpdateConfig)
				}

				// Invite code management (admin only)
				invites := admin.Group("/invite-codes")
				{
					invites.POST("", authHandler.CreateInviteCode)
					invites.GET("", authHandler.ListInviteCodes)
				}

				// Proxy management (admin only)
				proxies := admin.Group("/proxies")
				{
					proxies.GET("", proxyHandler.List)
					proxies.POST("", proxyHandler.Create)
					proxies.POST("/batch", proxyHandler.BatchCreate)
					proxies.POST("/test-all", proxyHandler.TestAllProxies)
					proxies.PUT("/:id", proxyHandler.Update)
					proxies.DELETE("/:id", proxyHandler.Delete)
					proxies.POST("/:id/toggle", proxyHandler.Toggle)
					proxies.POST("/:id/test", proxyHandler.TestProxy)
				}

				// Provider management (admin only)
				providers := admin.Group("/providers")
				{
					providers.GET("", providerHandler.List)
					providers.PUT("/:id", providerHandler.Update)
					providers.POST("/:id/toggle", providerHandler.Toggle)
					providers.POST("/:id/toggle-proxy", providerHandler.ToggleProxy)
					providers.GET("/:id/health", providerHandler.CheckHealth)
					providers.GET("/:id/api-keys", providerHandler.GetAPIKeys)
					providers.POST("/:id/api-keys", providerHandler.CreateAPIKey)
					providers.POST("/:id/api-keys/:key_id/toggle", providerHandler.ToggleAPIKey)
					providers.DELETE("/:id/api-keys/:key_id", providerHandler.DeleteAPIKey)
					providers.PUT("/:id/api-keys/:key_id", providerHandler.UpdateAPIKey)
				}

				// MCP management (admin only)
				mcp := admin.Group("/mcp")
				{
					mcp.GET("/servers", mcpHandler.ListServers)
					mcp.POST("/servers", mcpHandler.CreateServer)
					mcp.GET("/servers/:id", mcpHandler.GetServer)
					mcp.PUT("/servers/:id", mcpHandler.UpdateServer)
					mcp.DELETE("/servers/:id", mcpHandler.DeleteServer)
					mcp.POST("/servers/:id/refresh", mcpHandler.RefreshTools)
					mcp.GET("/tools", mcpHandler.ListTools)
					mcp.GET("/resources", mcpHandler.ListResources)
					mcp.GET("/resources/read", mcpHandler.ReadResource)
				}

				// Plan management (admin only)
				adminPlans := admin.Group("/plans")
				{
					adminPlans.POST("", planHandler.CreatePlan)
					adminPlans.PUT("/:id", planHandler.UpdatePlan)
				}

				// System settings (admin only)
				settings := admin.Group("/settings")
				{
					settings.GET("", configHandler.GetSettings)
					settings.POST("", configHandler.UpdateSettings)
				}

				// System-wide FinOps (admin only)
				adminFinops := admin.Group("/finops")
				{
					adminFinops.GET("/anomaly/system", finopsHandler.DetectSystemAnomaly)
					adminFinops.GET("/export/system-csv", finopsHandler.ExportSystemUsage)
				}
			}

			// ─── LLM API Endpoints ──────────────────────────────
			// Registered under /api/v1 (management API namespace).
			registerLLMEndpoints(v1, applyLLMMiddleware, chatHandler, modelHandler, authMiddleware)
		}
	}

	// ─── OpenAI-Compatible Route Aliases ───────────────────────────
	// OpenAI-compatible clients (Cline, Open WebUI, etc.) may call
	// endpoints with or without the /v1/ prefix depending on how the
	// base URL is configured:
	//   Base URL = "http://host:80"    → SDK calls /v1/chat/completions
	//   Base URL = "http://host:80/v1" → SDK calls /chat/completions
	// We register both levels to handle either configuration.

	// With /v1/ prefix (OpenAI SDK default)
	registerLLMEndpoints(engine.Group("/v1"), applyLLMMiddleware, chatHandler, modelHandler, authMiddleware)

	// Without /v1/ prefix (root-level, for SDKs that include /v1 in base URL)
	registerLLMEndpoints(engine, applyLLMMiddleware, chatHandler, modelHandler, authMiddleware)
}

// registerLLMEndpoints registers the OpenAI-compatible LLM API endpoints
// (chat, embeddings, images, audio, models) on the given router group.
// This is called multiple times to support different base URL configurations.
func registerLLMEndpoints(
	parent gin.IRouter,
	applyLLMMiddleware func(*gin.RouterGroup),
	chatHandler *handlers.ChatHandler,
	modelHandler *handlers.ModelHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	chat := parent.Group("/chat")
	applyLLMMiddleware(chat)
	chat.POST("/completions", chatHandler.ChatCompletion)

	embeddings := parent.Group("/embeddings")
	applyLLMMiddleware(embeddings)
	embeddings.POST("", chatHandler.Embeddings)

	images := parent.Group("/images")
	applyLLMMiddleware(images)
	images.POST("/generations", chatHandler.GenerateImage)

	audio := parent.Group("/audio")
	applyLLMMiddleware(audio)
	audio.POST("/transcriptions", chatHandler.TranscribeAudio)
	audio.POST("/speech", chatHandler.SynthesizeSpeech)

	models := parent.Group("/models")
	models.Use(authMiddleware.APIKey())
	models.GET("", modelHandler.List)
	models.GET("/providers", modelHandler.ListProviders)

	// ─── Anthropic-Compatible Routes ───────────────────────────
	anthro := parent.Group("/v1/messages")
	applyLLMMiddleware(anthro)
	anthro.POST("", chatHandler.AnthropicMessages)
}
