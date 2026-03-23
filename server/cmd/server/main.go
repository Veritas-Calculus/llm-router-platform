// Package main provides the entry point for the LLM Router Platform server.
//
// @title           LLM Router Platform API
// @version         1.0
// @description     A unified gateway for multiple Large Language Models with billing, memory, and proxy pool support.
//
// @host            localhost:8080
// @BasePath        /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/api/routes"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/database"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/announcement"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/billing"
	configService "llm-router-platform/internal/service/config"
	"llm-router-platform/internal/service/coupon"
	"llm-router-platform/internal/service/document"
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
	"llm-router-platform/internal/service/webhook"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	// Initialize encryption — mandatory in production.
	// Refuse to start without a valid encryption key to prevent
	// silent plaintext storage of provider API keys.
	if cfg.Encryption.Key == "" {
		logger.Fatal("ENCRYPTION_KEY is required — refusing to start without encryption")
	}
	if err := crypto.Initialize(cfg.Encryption.Key); err != nil {
		logger.Fatal("encryption initialization failed", zap.Error(err))
	}
	logger.Info("encryption initialized successfully")

	// Enforce minimum JWT secret length
	if len(cfg.JWT.Secret) < 32 {
		logger.Fatal("JWT_SECRET must be at least 32 characters")
	}

	// Warn if admin password does not meet complexity requirements
	// L5: In non-debug mode, refuse to start with a weak admin password
	if cfg.Admin.Password != "" && !isStrongPassword(cfg.Admin.Password) {
		if cfg.Server.Mode == "debug" {
			logger.Warn("ADMIN_PASSWORD does not meet complexity requirements (min 8 chars, upper+lower+digit). Consider updating it.")
		} else {
			logger.Fatal("ADMIN_PASSWORD does not meet complexity requirements (min 8 chars, upper+lower+digit) — refusing to start in production mode")
		}
	}

	// R7: Warn if CORS allows all origins in non-debug mode
	if len(cfg.Server.CORSOrigins) > 0 && cfg.Server.CORSOrigins[0] == "*" && cfg.Log.Level != "debug" {
		logger.Warn("CORS_ORIGINS is set to '*' — this allows any origin. Consider restricting to specific domains in production.")
	}

	db, err := database.New(&cfg.Database, logger)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Run database migrations.
	// In release mode, AutoMigrate is bypassed to prevent accidental schema changes.
	// Use 'cmd/migrate' or explicit SQL migrations for production deployments.
	if cfg.Server.Mode == "release" {
		logger.Info("Bypassing AutoMigrate in release mode. Please ensure database migrations are already applied.")
	} else {
		if err := db.Migrate(); err != nil {
			return err
		}
	}

	_ = db.SeedDefaultProviders()
	_ = db.SeedDefaultModels()
	_ = db.SeedDefaultAdmin(&cfg.Admin)
	seedDefaultPlans(db.DB)

	gormDB := db.DB
	repos := initRepositories(db, cfg)

	// Initialize Redis client for rate limiting
	redisOpts := &redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	// H2: Enable TLS for Redis connection when configured
	if cfg.Redis.TLSEnabled {
		redisOpts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		logger.Info("redis TLS enabled")
	}
	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("redis connection failed, rate limiting will be disabled", zap.Error(err))
		redisClient = nil
	} else {
		logger.Info("redis connected for rate limiting")
	}

	services := initServices(repos, cfg, logger, redisClient, gormDB)

	// Initialize MCP Service
	if err := services.MCP.Initialize(context.Background()); err != nil {
		logger.Error("failed to initialize MCP service", zap.Error(err))
	}

	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	// Sentry must be initialized before middleware registration
	if err := observability.InitSentry(cfg.Observability, logger); err != nil {
		logger.Error("sentry init failed (non-fatal)", zap.Error(err))
	}
	defer observability.ShutdownSentry(logger)

	// Register Sentry panic capture BEFORE Recovery so panics are reported
	engine.Use(middleware.SentryMiddleware())
	engine.Use(middleware.SentryUserContext())

	routes.Setup(engine, cfg, services, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      600 * time.Second, // LLM streaming responses can take minutes
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("starting server", zap.String("port", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// Lifecycle context: cancelled when shutdown signal is received.
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())

	if cfg.HealthCheck.Enabled {
		alertNotifier := health.NewAlertNotifier(repos.Alert, repos.AlertConfig, logger)
		scheduler := health.NewScheduler(services.Health, alertNotifier, cfg.HealthCheck.Interval, logger)
		go scheduler.Start(lifecycleCtx)
	}

	// Start async task worker pool
	workerPool := task.NewWorkerPool(services.TaskService, repos.Task, task.DefaultWorkerPoolConfig(), logger)
	task.RegisterDefaultExecutors(workerPool, services.Router, logger)
	go workerPool.Start(lifecycleCtx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Periodic Webhook dispatcher (runs every 5 seconds)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				services.Webhook.ProcessPendingDeliveries(context.Background())
			case <-lifecycleCtx.Done():
				return
			}
		}
	}()

	// Periodic data cleanup (runs daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = db.CleanupOldHealthHistory(30) // 30-day retention
				_, _ = db.CleanupOldAlerts(90)        // 90-day retention for resolved alerts
				_, _ = services.AuditService.PurgeOlderThan(context.Background(), 90*24*time.Hour) // 90-day retention for audit logs
			case <-lifecycleCtx.Done():
				return
			}
		}
	}()

	<-quit

	logger.Info("shutting down server — draining connections…")

	// 1. Cancel background goroutines (health scheduler, cleanup ticker)
	lifecycleCancel()

	// 2. Shutdown HTTP server with deadline
	shutdownTimeout := 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
		return err
	}
	logger.Info("http server stopped")

	// 3. Shutdown observability (flush traces)
	if err := services.Observability.Shutdown(ctx); err != nil {
		logger.Error("observability shutdown error", zap.Error(err))
	}

	// 4. Close Redis connection
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			logger.Error("redis close error", zap.Error(err))
		}
		logger.Info("redis connection closed")
	}

	logger.Info("shutdown complete")
	return nil
}

// Repositories holds all repository instances.
type Repositories struct {
	User           *repository.UserRepository
	Organization   *repository.OrganizationRepository
	Project        *repository.ProjectRepository
	APIKey         *repository.APIKeyRepository
	Provider       *repository.ProviderRepository
	ProviderAPIKey *repository.ProviderAPIKeyRepository
	Model          *repository.ModelRepository
	Proxy          *repository.ProxyRepository
	UsageLog       *repository.UsageLogRepository
	HealthHistory  *repository.HealthHistoryRepository
	Memory         *repository.ConversationMemoryRepository
	Alert          *repository.AlertRepository
	AlertConfig    *repository.AlertConfigRepository
	Budget         *repository.BudgetRepository
	Task           *repository.TaskRepository
	AuditLog       *repository.AuditLogRepository
	MCP            *repository.MCPRepository
	Plan           *repository.PlanRepository
	Subscription   *repository.SubscriptionRepository
	Transaction    *repository.TransactionRepository
	Config         *repository.ConfigRepository
	RoutingRule    repository.RoutingRuleRepo
	Webhook        repository.WebhookRepository
}

func initRepositories(db *database.Database, cfg *config.Config) *Repositories {
	return &Repositories{
		User:           repository.NewUserRepository(db.DB),
		Organization:   repository.NewOrganizationRepository(db.DB),
		Project:        repository.NewProjectRepository(db.DB),
		APIKey:         repository.NewAPIKeyRepository(db.DB),
		Provider:       repository.NewProviderRepository(db.DB),
		ProviderAPIKey: repository.NewProviderAPIKeyRepository(db.DB),
		Model:          repository.NewModelRepository(db.DB),
		Proxy:          repository.NewProxyRepository(db.DB),
		UsageLog:       repository.NewUsageLogRepository(db.DB),
		HealthHistory:  repository.NewHealthHistoryRepository(db.DB),
		Memory:         repository.NewConversationMemoryRepository(db.DB),
		Alert:          repository.NewAlertRepository(db.DB),
		AlertConfig:    repository.NewAlertConfigRepository(db.DB),
		Budget:         repository.NewBudgetRepository(db.DB),
		Task:           repository.NewTaskRepository(db.DB),
		AuditLog:       repository.NewAuditLogRepository(db.DB, cfg.Encryption.Key),
		MCP:            repository.NewMCPRepository(db.DB),
		Plan:           repository.NewPlanRepository(db.DB),
		Subscription:   repository.NewSubscriptionRepository(db.DB),
		Transaction:    repository.NewTransactionRepository(db.DB),
		Config:         repository.NewConfigRepository(db.DB),
		RoutingRule:    repository.NewRoutingRuleRepository(db.DB),
		Webhook:        repository.NewWebhookRepository(db.DB),
	}
}

func initServices(repos *Repositories, cfg *config.Config, logger *zap.Logger, redisClient *redis.Client, gormDB *gorm.DB) *routes.Services {
	userService := user.NewService(repos.User, repos.APIKey, repos.Project, repos.Organization, logger)

	// Provider registry - clients are created dynamically based on database configuration
	// API keys are stored encrypted in the database and configured via Web UI
	providerRegistry := provider.NewRegistry(logger)

	mcpService := mcp.NewService(repos.MCP, logger)
	configService := configService.NewService(repos.Config, logger)

	routerService := router.NewRouter(repos.Provider, repos.ProviderAPIKey, repos.Proxy, repos.Model, repos.RoutingRule, providerRegistry, mcpService, logger)
	if redisClient != nil {
		routerService.SetRedisClient(redisClient)
	}
	billingService := billing.NewService(repos.UsageLog, repos.Model, redisClient, logger)
	budgetService := billing.NewBudgetService(repos.UsageLog, repos.Budget, logger)
	subscriptionService := billing.NewSubscriptionService(repos.Plan, repos.Subscription, repos.UsageLog, logger)
	
	emailSvc := email.NewService(cfg.Email, cfg.Frontend.URL)
	balanceService := billing.NewBalanceService(gormDB, repos.User, repos.Transaction, redisClient, emailSvc, logger)
	
	// Dynamically get Stripe config from DB if available
	stripeCfg := configService.GetStripeConfig(context.Background(), cfg.Stripe)
	paymentService := billing.NewPaymentService(stripeCfg, cfg.Frontend.URL, repos.Plan, repos.Subscription, repos.Transaction, logger)
	
	memoryService := memory.NewService(repos.Memory, redisClient, logger)
	proxyService := proxy.NewService(repos.Proxy, logger)
	obsService := observability.NewCompositeService(
		observability.NewLangfuseService(cfg.Observability, logger),
		observability.NewOTelService(context.Background(), cfg.Observability, logger),
	)
	auditService := audit.NewService(repos.AuditLog, logger)

	alertNotifier := health.NewAlertNotifier(repos.Alert, repos.AlertConfig, logger)
	healthService := health.NewService(
		repos.APIKey,
		repos.ProviderAPIKey,
		repos.Proxy,
		repos.Provider,
		repos.HealthHistory,
		alertNotifier,
		providerRegistry,
		proxyService,
		logger,
	)

	taskService := task.NewService(repos.Task, logger)
	redeemService := redeem.NewService(gormDB, logger)
	announcementService := announcement.NewService(gormDB, logger)
	couponService := coupon.NewService(gormDB, logger)
	documentService := document.NewService(gormDB, logger)
	webhookService := webhook.NewWebhookService(repos.Webhook, logger)

	return &routes.Services{
		User:          userService,
		Router:        routerService,
		Billing:       billingService,
		BudgetService: budgetService,
		Subscription:  subscriptionService,
		Payment:       paymentService,
		Balance:       balanceService,
		SystemConfig:  configService,
		Health:        healthService,
		Memory:        memoryService,
		Observability: obsService,
		Proxy:         proxyService,
		Provider:      providerRegistry,
		TaskService:   taskService,
		AuditService:  auditService,
		RedeemSvc:     redeemService,
		AnnouncementSvc: announcementService,
		CouponSvc:     couponService,
		DocumentSvc:   documentService,
		Webhook:       webhookService,
		MCP:           mcpService,
		RedisClient:   redisClient,
		DB:            gormDB, // For health checks (operational handler)
	}
}

func seedDefaultPlans(db *gorm.DB) {
	plans := []models.Plan{
		{
			Name:         "Free",
			Description:  "Perfect for trying out the platform",
			PriceMonth:   0,
			TokenLimit:   100000,
			RateLimit:    5,
			SupportLevel: "standard",
			Features:     "Basic models access, 100K tokens/month, 5 RPM",
		},
		{
			Name:         "Pro",
			Description:  "For individuals and small teams",
			PriceMonth:   20,
			TokenLimit:   5000000,
			RateLimit:    50,
			SupportLevel: "priority",
			Features:     "All models access, MCP support, 5M tokens/month, 50 RPM",
		},
	}

	for _, p := range plans {
		var existing models.Plan
		if err := db.Where("name = ?", p.Name).First(&existing).Error; err != nil {
			db.Create(&p)
		}
	}
}

// isStrongPassword checks if a password contains at least 8 chars,
// one uppercase, one lowercase, and one digit.
func isStrongPassword(pw string) bool {
	if len(pw) < 8 {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range pw {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}
