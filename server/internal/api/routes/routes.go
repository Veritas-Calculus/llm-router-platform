// Package routes defines API routes.
package routes

import (
	"database/sql"
	"net/http/pprof"

	"llm-router-platform/internal/api/handlers"
	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/graphql/dataloaders"
	gqlhandler "llm-router-platform/internal/graphql/handler"
	"llm-router-platform/internal/graphql/resolvers"
	announcementSvc "llm-router-platform/internal/service/announcement"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/billing"
	configService "llm-router-platform/internal/service/config"
	couponSvc "llm-router-platform/internal/service/coupon"
	documentSvc "llm-router-platform/internal/service/document"
	"llm-router-platform/internal/service/email"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/mcp"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/redeem"
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
	RedeemSvc     *redeem.Service
	AnnouncementSvc *announcementSvc.Service
	CouponSvc     *couponSvc.Service
	DocumentSvc   *documentSvc.Service
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

	// ─── GraphQL Endpoint ────────────────────────────────────────────
	emailSvcForGql := email.NewService(cfg.Email, cfg.Frontend.URL)
	gqlResolver := &resolvers.Resolver{
		UserSvc:         services.User,
		PasswordResetSvc: user.NewPasswordResetService(services.DB),
		Router:          services.Router,
		Billing:         services.Billing,
		BudgetService:   services.BudgetService,
		SubscriptionSvc: services.Subscription,
		Payment:         services.Payment,
		Balance:         services.Balance,
		SystemConfig:    services.SystemConfig,
		Health:          services.Health,
		Memory:          services.Memory,
		MCP:             services.MCP,
		Observability:   services.Observability,
		Proxy:           services.Proxy,
		Provider:        services.Provider,
		TaskService:     services.TaskService,
		AuditService:    services.AuditService,
		EmailService:    emailSvcForGql,
		RedeemSvc:       services.RedeemSvc,
		AnnouncementSvc: services.AnnouncementSvc,
		CouponSvc:       services.CouponSvc,
		DocumentSvc:     services.DocumentSvc,
		RedisClient:     services.RedisClient,
		DB:              services.DB,
		Config:          cfg,
		Logger:          logger,
	}
	graphqlHandler := gqlhandler.NewHandler(gqlResolver, cfg, logger)

	// JWT-optional: the @auth directive handles per-field authentication.
	// We apply JWT middleware but do NOT abort on missing token, because
	// some mutations (login, register) are public.
	graphqlGroup := engine.Group("/graphql")
	graphqlGroup.Use(requestIDMiddleware.Handle())
	graphqlGroup.Use(authMiddleware.OptionalJWT())
	graphqlGroup.Use(dataloaders.Middleware(services.User))
	// Inject Redis client for per-field @rateLimit directive
	if services.RedisClient != nil {
		graphqlGroup.Use(func(c *gin.Context) {
			c.Set("redis", services.RedisClient)
			c.Next()
		})
	}
	graphqlGroup.POST("", graphqlHandler.ServeGraphQL())

	if cfg.Server.Mode != "release" {
		graphqlGroup.GET("", gqlhandler.ServePlayground())
		logger.Info("graphql playground enabled at /graphql (non-release mode)")
	}

	// ─── Rate Limiter middleware ──────────────────────────────────────
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.RequestsPerMinute, services.RedisClient, logger)

	// Per-key rate limiter (used by LLM endpoints)
	perKeyLimiter := middleware.NewPerKeyRateLimiter(services.RedisClient, logger)
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
	// NOTE: Management REST API has been deprecated.
	// All management operations are now served via /graphql (Apollo Client).
	// Only LLM proxy endpoints and payment webhooks remain under /api/v1.

	chatHandler := handlers.NewChatHandler(services.Router, services.Billing, services.Memory, services.Subscription, services.Balance, services.Observability, logger)
	modelHandler := handlers.NewModelHandler(services.Router, services.Provider, logger)
	paymentHandler := handlers.NewPaymentHandler(services.Payment, logger)

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
			// ── Public / Webhooks ──────────────────────────────
			v1.POST("/payments/webhook/stripe", paymentHandler.StripeWebhook)

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
