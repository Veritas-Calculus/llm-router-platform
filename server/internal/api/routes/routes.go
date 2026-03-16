// Package routes defines API routes.
package routes

import (
	"context"
	"database/sql"
	"net/http/pprof"
	"time"

	"llm-router-platform/internal/api/handlers"
	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/health"
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
	Health        *health.Service
	Memory        *memory.Service
	Observability observability.Service
	Proxy         *proxy.Service
	Provider      *provider.Registry
	TaskService   *task.Service
	RedisClient   *redis.Client // For rate limiting
	DB            *gorm.DB      // For health checks
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
	// Basic liveness probe (always returns ok)
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Deep health check (checks PG + Redis connectivity)
	engine.GET("/healthz", func(c *gin.Context) {
		checks := gin.H{}
		healthy := true

		// Check PostgreSQL
		if services.DB != nil {
			sqlDB, err := services.DB.DB()
			if err != nil {
				checks["postgres"] = gin.H{"status": "error"}
				healthy = false
			} else {
				ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				defer cancel()
				if err := sqlDB.PingContext(ctx); err != nil {
					checks["postgres"] = gin.H{"status": "error"}
					healthy = false
				} else {
					checks["postgres"] = gin.H{"status": "ok"}
				}
			}
		}

		// Check Redis
		if services.RedisClient != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := services.RedisClient.Ping(ctx).Err(); err != nil {
				checks["redis"] = gin.H{"status": "error"}
				healthy = false
			} else {
				checks["redis"] = gin.H{"status": "ok"}
			}
		}

		status := "ok"
		httpCode := 200
		if !healthy {
			status = "degraded"
			httpCode = 503
		}

		c.JSON(httpCode, gin.H{
			"status": status,
			"checks": checks,
		})
	})

	// Readiness probe (for K8s — service is ready to accept traffic)
	engine.GET("/readyz", func(c *gin.Context) {
		// Check that critical dependencies are available
		if services.DB != nil {
			sqlDB, err := services.DB.DB()
			if err != nil {
				c.JSON(503, gin.H{"status": "not ready", "error": err.Error()})
				return
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
			defer cancel()
			if err := sqlDB.PingContext(ctx); err != nil {
				c.JSON(503, gin.H{"status": "not ready", "error": "database unavailable"})
				return
			}
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// Version endpoint
	engine.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":    Version,
			"git_commit": GitCommit,
			"build_time": BuildTime,
		})
	})

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
	auditService := audit.NewService(services.DB, logger)

	authHandler := handlers.NewAuthHandler(services.User, auditService, &cfg.JWT, cfg.Registration.Mode, cfg.Registration.InviteCode, services.RedisClient, services.DB, logger)
	apiKeyHandler := handlers.NewAPIKeyHandler(services.User, logger)
	chatHandler := handlers.NewChatHandler(services.Router, services.Billing, services.Memory, services.Observability, logger)
	modelHandler := handlers.NewModelHandler(services.Router, services.Provider, logger)
	usageHandler := handlers.NewUsageHandler(services.Billing, logger)
	healthHandler := handlers.NewHealthHandler(services.Health, logger)
	alertHandler := handlers.NewAlertHandler(services.Health, logger)
	dashboardHandler := handlers.NewDashboardHandler(services.Billing, services.Health, services.Router, services.User, logger)
	proxyHandler := handlers.NewProxyHandler(services.Proxy, logger)
	providerHandler := handlers.NewProviderHandler(services.Router, services.Health, logger)
	finopsHandler := handlers.NewFinOpsHandler(services.Billing, services.BudgetService, logger)
	userHandler := handlers.NewUserHandler(services.User, services.Billing, services.RedisClient, logger)

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
				auth.POST("/refresh", authHandler.RefreshToken)
				auth.POST("/token/rotate", authHandler.RotateRefreshToken) // Refresh token rotation
			}

			// ── All authenticated users ─────────────────────────
			protected := v1.Group("")
			protected.Use(authMiddleware.JWT())
			protected.Use(perUserLimiter.Limit())
			{
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
			}

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

				// System-wide FinOps (admin only)
				adminFinops := admin.Group("/finops")
				{
					adminFinops.GET("/anomaly/system", finopsHandler.DetectSystemAnomaly)
					adminFinops.GET("/export/system-csv", finopsHandler.ExportSystemUsage)
				}
			}

			// ─── LLM API Endpoints ──────────────────────────────
			chat := v1.Group("/chat")
			applyLLMMiddleware(chat)
			{
				chat.POST("/completions", chatHandler.ChatCompletion)
			}

			embeddings := v1.Group("/embeddings")
			applyLLMMiddleware(embeddings)
			{
				embeddings.POST("", chatHandler.Embeddings)
			}

			images := v1.Group("/images")
			applyLLMMiddleware(images)
			{
				images.POST("/generations", chatHandler.GenerateImage)
			}

			audio := v1.Group("/audio")
			applyLLMMiddleware(audio)
			{
				audio.POST("/transcriptions", chatHandler.TranscribeAudio)
				audio.POST("/speech", chatHandler.SynthesizeSpeech)
			}
			models := v1.Group("/models")
			models.Use(authMiddleware.APIKey())
			{
				models.GET("", modelHandler.List)
				models.GET("/providers", modelHandler.ListProviders)
			}
		}
	}

	// ─── OpenAI-Compatible Route Aliases ───────────────────────────
	// OpenAI-compatible clients (Cline, Open WebUI, etc.) may call
	// endpoints with or without the /v1/ prefix depending on how the
	// base URL is configured:
	//   Base URL = "http://host:80"    → SDK calls /v1/chat/completions
	//   Base URL = "http://host:80/v1" → SDK calls /chat/completions
	// We register both levels to handle either configuration.

	// With /v1/ prefix
	compat := engine.Group("/v1")
	applyLLMMiddleware(compat)
	{
		compat.POST("/chat/completions", chatHandler.ChatCompletion)
		compat.POST("/embeddings", chatHandler.Embeddings)
		compat.POST("/images/generations", chatHandler.GenerateImage)
		compat.POST("/audio/transcriptions", chatHandler.TranscribeAudio)
		compat.POST("/audio/speech", chatHandler.SynthesizeSpeech)
	}

	compatModels := engine.Group("/v1/models")
	compatModels.Use(authMiddleware.APIKey())
	{
		compatModels.GET("", modelHandler.List)
		compatModels.GET("/providers", modelHandler.ListProviders)
	}

	// Without /v1/ prefix (root-level, for SDKs that include /v1 in base URL)
	rootChat := engine.Group("/chat")
	applyLLMMiddleware(rootChat)
	{
		rootChat.POST("/completions", chatHandler.ChatCompletion)
	}

	rootEmbeddings := engine.Group("/embeddings")
	applyLLMMiddleware(rootEmbeddings)
	{
		rootEmbeddings.POST("", chatHandler.Embeddings)
	}

	rootImages := engine.Group("/images")
	applyLLMMiddleware(rootImages)
	{
		rootImages.POST("/generations", chatHandler.GenerateImage)
	}

	rootAudio := engine.Group("/audio")
	applyLLMMiddleware(rootAudio)
	{
		rootAudio.POST("/transcriptions", chatHandler.TranscribeAudio)
		rootAudio.POST("/speech", chatHandler.SynthesizeSpeech)
	}

	rootModels := engine.Group("/models")
	rootModels.Use(authMiddleware.APIKey())
	{
		rootModels.GET("", modelHandler.List)
	}
}
