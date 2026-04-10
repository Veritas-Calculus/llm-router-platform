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
	"fmt"
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
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/admin"
	"llm-router-platform/internal/service/announcement"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/billing"
	semantic "llm-router-platform/internal/service/cache"
	configService "llm-router-platform/internal/service/config"
	"llm-router-platform/internal/service/coupon"
	"llm-router-platform/internal/service/document"
	"llm-router-platform/internal/service/email"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/mcp"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/monitoring"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/redeem"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/task"
	"llm-router-platform/internal/service/turnstile"
	"llm-router-platform/internal/service/user"
	"llm-router-platform/internal/service/webhook"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Application — top-level lifecycle container.
//
// Lifecycle:   NewApplication → InitInfrastructure → InitServices → Start
//              (wait for signal)
//              Shutdown
// ─────────────────────────────────────────────────────────────────────────────

// Application holds all runtime state for the server.  Each lifecycle phase
// populates a different subset of fields, keeping responsibilities clear.
type Application struct {
	cfg    *config.Config
	logger *zap.Logger

	// Infrastructure (populated by InitInfrastructure)
	db          *database.Database
	redisClient *redis.Client

	// Business layer (populated by InitServices)
	repos    *Repositories
	services *routes.Services

	// HTTP (populated by Start)
	server          *http.Server
	lifecycleCancel context.CancelFunc
}

// run orchestrates the full server lifecycle.
func run() error {
	app, err := NewApplication()
	if err != nil {
		return err
	}
	defer func() { _ = app.logger.Sync() }()

	// Feature gate audit logging happens after DB merge in initServices → cfgService.InitFeatureGates

	if err := app.InitInfrastructure(); err != nil {
		return err
	}

	app.InitServices()

	app.Start()

	// Block until termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	return app.Shutdown()
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 1 — Configuration & Validation
// ─────────────────────────────────────────────────────────────────────────────

// NewApplication loads configuration, builds the logger, and runs all startup
// pre-checks (encryption key, JWT secret length, admin password strength).
func NewApplication() (*Application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger, err := buildLogger(cfg.Log)
	if err != nil {
		log.Fatalf("failed to build logger: %v", err)
	}

	// Initialize encryption — mandatory in production.
	if cfg.Encryption.Key == "" {
		logger.Fatal("ENCRYPTION_KEY is required — refusing to start without encryption")
	}

	// Vault Transit Engine or local AES-256-GCM
	if cfg.Vault.Addr != "" {
		if cfg.Vault.Token == "" {
			logger.Fatal("VAULT_TOKEN is required when VAULT_ADDR is set")
		}
		transitKey := cfg.Vault.TransitKey
		if transitKey == "" {
			transitKey = "llm-router"
		}
		if err := crypto.InitializeVault(cfg.Encryption.Key, cfg.Vault.Addr, cfg.Vault.Token, transitKey); err != nil {
			logger.Fatal("vault encryption initialization failed", zap.Error(err))
		}
		logger.Info("encryption initialized via Vault Transit Engine",
			zap.String("vault_addr", cfg.Vault.Addr),
			zap.String("transit_key", transitKey),
		)
	} else {
		if err := crypto.Initialize(cfg.Encryption.Key); err != nil {
			logger.Fatal("encryption initialization failed", zap.Error(err))
		}
		logger.Info("encryption initialized with local AES-256-GCM")
	}

	// Enforce minimum JWT secret length
	if len(cfg.JWT.Secret) < 32 {
		logger.Fatal("JWT_SECRET must be at least 32 characters")
	}

	// Validate admin password complexity
	if cfg.Admin.Password != "" && !isStrongPassword(cfg.Admin.Password) {
		if cfg.Server.Mode == "debug" {
			logger.Warn("ADMIN_PASSWORD does not meet complexity requirements (min 8 chars, upper+lower+digit). Consider updating it.")
		} else {
			logger.Fatal("ADMIN_PASSWORD does not meet complexity requirements (min 8 chars, upper+lower+digit) — refusing to start in production mode")
		}
	}

	// CORS safety enforcement
	if len(cfg.Server.CORSOrigins) > 0 && cfg.Server.CORSOrigins[0] == "*" {
		if cfg.Server.Mode == "debug" {
			logger.Warn("CORS_ORIGINS is set to '*' — this allows any origin. Consider restricting to specific domains in production.")
		} else {
			logger.Fatal("CORS_ORIGINS is set to '*' — wildcard CORS is not allowed in production/release mode. Set to specific origins.")
		}
	}

	return &Application{cfg: cfg, logger: logger}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 2 — Infrastructure (Database + Redis)
// ─────────────────────────────────────────────────────────────────────────────

// InitInfrastructure connects to PostgreSQL and Redis, runs migrations, and
// seeds default data (respecting release-mode guards).
func (app *Application) InitInfrastructure() error {
	// ── Database ─────────────────────────────────────────────────────────
	db, err := database.New(&app.cfg.Database, app.cfg.Server.Mode, app.logger)
	if err != nil {
		return err
	}
	app.db = db

	// ── SQL Migrations (always run — safe, idempotent, version-tracked) ──
	if err := database.RunSQLMigrations(&app.cfg.Database, app.logger); err != nil {
		app.logger.Warn("SQL migrations skipped or failed", zap.Error(err))
	}

	// GORM AutoMigrate — only in non-release mode for dev convenience.
	// Production schema changes must go through golang-migrate SQL migrations.
	if app.cfg.Server.Mode != "release" {
		if err := db.Migrate(); err != nil {
			return fmt.Errorf("AutoMigrate failed: %w", err)
		}
		app.logger.Info("database schema synchronization completed (AutoMigrate)")
	}

	// Seed data
	app.seedData()

	// ── Redis ────────────────────────────────────────────────────────────
	app.redisClient = app.connectRedis()

	return nil
}

// seedData populates default data.  Only the admin account is ensured on
// startup.  Providers, models, and plans must be configured manually by the
// administrator through the management UI.
func (app *Application) seedData() {
	if app.cfg.Server.Mode == "release" {
		if err := app.db.SeedDefaultAdminOnly(&app.cfg.Admin); err != nil {
			app.logger.Error("failed to seed admin user", zap.Error(err))
		}
	} else {
		if err := app.db.SeedDefaultAdmin(&app.cfg.Admin); err != nil {
			app.logger.Error("failed to seed admin user", zap.Error(err))
		}
	}
	app.logger.Info("provider/model/plan seed data skipped: configure via admin UI")
}

// connectRedis creates and tests a Redis connection.  Returns nil (with a
// warning) if the connection fails — downstream code treats nil as "disabled".
func (app *Application) connectRedis() *redis.Client {
	opts := &redis.Options{
		Addr:     app.cfg.Redis.GetRedisAddr(),
		Password: app.cfg.Redis.Password,
		DB:       app.cfg.Redis.DB,
	}
	if app.cfg.Redis.TLSEnabled {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		app.logger.Info("redis TLS enabled")
	}

	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		app.logger.Warn("redis connection failed, rate limiting will be disabled", zap.Error(err))
		return nil
	}
	app.logger.Info("redis connected for rate limiting")
	return client
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3 — Service Layer
// ─────────────────────────────────────────────────────────────────────────────

// InitServices creates all repositories and services, wiring them together.
func (app *Application) InitServices() {
	gormDB := app.db.DB
	app.repos = initRepositories(app.db, app.cfg)
	app.services = initServices(app.repos, app.cfg, app.logger, app.redisClient, gormDB)

	// Initialize MCP Service
	if err := app.services.MCP.Initialize(context.Background()); err != nil {
		app.logger.Error("failed to initialize MCP service", zap.Error(err))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 4 — HTTP Server & Background Jobs
// ─────────────────────────────────────────────────────────────────────────────

// Start configures the Gin engine, registers routes, launches the HTTP server,
// and starts all background goroutines (health checks, webhooks, cleanup).
func (app *Application) Start() {
	gin.SetMode(app.cfg.Server.Mode)
	engine := gin.New()

	// Sentry must be initialized before middleware registration
	if err := observability.InitSentry(app.cfg.Observability, app.logger); err != nil {
		app.logger.Error("sentry init failed (non-fatal)", zap.Error(err))
	}
	engine.Use(middleware.SentryMiddleware())
	engine.Use(middleware.SentryUserContext())

	routes.Setup(engine, app.cfg, app.services, app.logger)

	app.server = &http.Server{
		Addr:              ":" + app.cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Duration(app.cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(app.cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		app.logger.Info("starting server", zap.String("port", app.cfg.Server.Port))
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Fatal("server error", zap.Error(err))
		}
	}()

	app.startBackgroundJobs()
}

// startBackgroundJobs launches health scheduler, task worker pool, webhook
// dispatcher, and data cleanup goroutines — all governed by a shared lifecycle
// context that is cancelled during Shutdown.
func (app *Application) startBackgroundJobs() {
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	app.lifecycleCancel = cancel

	// Health check scheduler
	if app.cfg.HealthCheck.Enabled {
		alertNotifier := health.NewAlertNotifier(app.repos.Alert, app.repos.AlertConfig, app.logger, app.cfg.Server.AllowLocalProviders)
		scheduler := health.NewScheduler(app.services.Health, alertNotifier, app.cfg.HealthCheck.Interval, app.logger)
		go scheduler.Start(lifecycleCtx)
	}

	// Async task worker pool
	workerPool := task.NewWorkerPool(app.services.TaskService, app.repos.Task, task.DefaultWorkerPoolConfig(), app.logger)
	task.RegisterDefaultExecutors(workerPool, app.services.Router, app.logger)
	go workerPool.Start(lifecycleCtx)

	// Periodic webhook dispatcher (every 5s)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				app.services.Webhook.ProcessPendingDeliveries(context.Background())
			case <-lifecycleCtx.Done():
				return
			}
		}
	}()

	// Periodic data cleanup (daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				app.runDataCleanup()
			case <-lifecycleCtx.Done():
				return
			}
		}
	}()
}

// runDataCleanup purges old health history, alerts, and audit logs based on
// configurable retention periods.
func (app *Application) runDataCleanup() {
	if n, err := app.db.CleanupOldHealthHistory(app.cfg.Cleanup.HealthRetentionDays); err != nil {
		app.logger.Error("health history cleanup failed", zap.Error(err))
	} else if n > 0 {
		app.logger.Info("health history cleanup completed", zap.Int64("deleted", n))
	}
	if n, err := app.db.CleanupOldAlerts(app.cfg.Cleanup.AlertRetentionDays); err != nil {
		app.logger.Error("alert cleanup failed", zap.Error(err))
	} else if n > 0 {
		app.logger.Info("alert cleanup completed", zap.Int64("deleted", n))
	}
	retention := time.Duration(app.cfg.Cleanup.AuditRetentionDays) * 24 * time.Hour
	if n, err := app.services.AuditService.PurgeOlderThan(context.Background(), retention); err != nil {
		app.logger.Error("audit log cleanup failed", zap.Error(err))
	} else if n > 0 {
		app.logger.Info("audit log cleanup completed", zap.Int64("deleted", n))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 5 — Shutdown
// ─────────────────────────────────────────────────────────────────────────────

// Shutdown performs an ordered teardown: cancel background goroutines, drain
// HTTP connections, flush observability, and close infrastructure connections.
func (app *Application) Shutdown() error {
	app.logger.Info("shutting down server — draining connections…")

	// 1. Cancel background goroutines
	if app.lifecycleCancel != nil {
		app.lifecycleCancel()
	}

	// 2. Shutdown HTTP server with deadline
	shutdownTimeout := 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := app.server.Shutdown(ctx); err != nil {
		app.logger.Error("server shutdown error", zap.Error(err))
		return err
	}
	app.logger.Info("http server stopped")

	// 3. Shutdown observability (flush traces)
	if err := app.services.Observability.Shutdown(ctx); err != nil {
		app.logger.Error("observability shutdown error", zap.Error(err))
	}

	// 4. Flush Sentry events
	observability.ShutdownSentry(app.logger)

	// 5. Close Redis
	if app.redisClient != nil {
		if err := app.redisClient.Close(); err != nil {
			app.logger.Error("redis close error", zap.Error(err))
		}
		app.logger.Info("redis connection closed")
	}

	// 6. Close database
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			app.logger.Error("database close error", zap.Error(err))
		}
	}

	app.logger.Info("shutdown complete")
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Repository & Service Wiring
// ─────────────────────────────────────────────────────────────────────────────

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
	providerRegistry := provider.NewRegistry(logger)

	mcpService := mcp.NewService(repos.MCP, logger, cfg.Server.AllowLocalProviders)
	cfgService := configService.NewService(repos.Config, logger)
	cfgService.InitFeatureGates(cfg.FeatureGates)
	cfg.FeatureGates.LogGates(logger)

	// Wire Redis for FG distributed sync (Pub/Sub propagation across instances)
	if redisClient != nil {
		cfgService.SetRedis(redisClient)
		cfgService.StartFGSubscriber(context.Background(), cfg.FeatureGates)
	}
	routerService := router.NewRouter(repos.Provider, repos.ProviderAPIKey, repos.Proxy, repos.Model, repos.RoutingRule, providerRegistry, mcpService, logger, cfg.Server.AllowLocalProviders)
	if redisClient != nil {
		routerService.SetRedisClient(redisClient)
	}
	billingService := billing.NewService(repos.UsageLog, repos.Model, redisClient, logger)
	budgetService := billing.NewBudgetService(repos.UsageLog, repos.Budget, logger)
	subscriptionService := billing.NewSubscriptionService(repos.Plan, repos.Subscription, repos.UsageLog, logger)

	emailSvc := email.NewService(cfg.Email, cfg.Frontend.URL)
	balanceService := billing.NewBalanceService(gormDB, repos.User, repos.Transaction, redisClient, emailSvc, logger)

	// Dynamically get Stripe config from DB if available
	stripeCfg := cfgService.GetStripeConfig(context.Background(), cfg.Stripe)
	paymentService := billing.NewPaymentService(stripeCfg, cfg.Frontend.URL, repos.Plan, repos.Subscription, repos.Transaction, logger)
	wechatPayService := billing.NewWechatPayService(cfg.WechatPay, cfg.Frontend.URL, repos.Subscription, repos.Transaction, logger)
	alipayService := billing.NewAlipayService(cfg.Alipay, cfg.Frontend.URL, repos.Subscription, repos.Transaction, logger)

	memoryService := memory.NewService(repos.Memory, redisClient, logger)
	proxyService := proxy.NewService(repos.Proxy, logger)
	obsService := observability.NewCompositeService(
		observability.NewLangfuseService(cfg.Observability, logger),
		observability.NewOTelService(context.Background(), cfg.Observability, logger),
	)
	auditService := audit.NewService(repos.AuditLog, logger)

	alertNotifier := health.NewAlertNotifier(repos.Alert, repos.AlertConfig, logger, cfg.Server.AllowLocalProviders)
	healthService := health.NewService(
		repos.APIKey, repos.ProviderAPIKey, repos.Proxy, repos.Provider,
		repos.HealthHistory, alertNotifier, providerRegistry, proxyService, logger,
		cfg.Server.AllowLocalProviders,
	)

	taskService := task.NewService(repos.Task, logger, cfg.Server.AllowLocalProviders)
	redeemService := redeem.NewService(gormDB, logger)
	announcementService := announcement.NewService(gormDB, logger)
	couponService := coupon.NewService(gormDB, logger)
	documentService := document.NewService(gormDB, logger)
	webhookService := webhook.NewWebhookService(repos.Webhook, logger, cfg.Server.AllowLocalProviders)

	// Services previously created inside routes.Setup() — consolidated here
	passwordResetSvc := user.NewPasswordResetService(gormDB)
	emailVerifySvc := user.NewEmailVerificationService(gormDB)
	loginLimiter := user.NewLoginLimiter(redisClient, logger)
	cacheSvc := semantic.NewSemanticCacheService(gormDB, logger, 0.05)
	turnstileSvc := turnstile.New(logger, cfg.Turnstile.Enabled, cfg.Turnstile.SecretKey)
	monitoringSvc := monitoring.NewCollector(
		gormDB, redisClient,
		monitoring.BuildInfo{Version: routes.Version, GitCommit: routes.GitCommit, BuildTime: routes.BuildTime},
		cfg.Server.Mode, logger,
	)
	adminSvc := admin.NewService(gormDB, redisClient, cfg, logger)
	adminSvc.SetSystemConfig(cfgService)

	return &routes.Services{
		User:             userService,
		PasswordResetSvc: passwordResetSvc,
		EmailVerifySvc:   emailVerifySvc,
		LoginLimiter:     loginLimiter,
		Router:           routerService,
		Billing:          billingService,
		BudgetService:    budgetService,
		Subscription:     subscriptionService,
		Payment:          paymentService,
		WechatPay:        wechatPayService,
		Alipay:           alipayService,
		Balance:          balanceService,
		SystemConfig:     cfgService,
		Health:           healthService,
		Memory:           memoryService,
		Observability:    obsService,
		Proxy:            proxyService,
		Provider:         providerRegistry,
		TaskService:      taskService,
		AuditService:     auditService,
		EmailService:     emailSvc,
		RedeemSvc:        redeemService,
		AnnouncementSvc:  announcementService,
		CouponSvc:        couponService,
		DocumentSvc:      documentService,
		Webhook:          webhookService,
		MonitoringSvc:    monitoringSvc,
		TurnstileSvc:     turnstileSvc,
		SemanticCache:    cacheSvc,
		MCP:              mcpService,
		RedisClient:      redisClient,
		DB:               gormDB,
		Config:           cfg,
		AdminSvc:         adminSvc,
		Logger:           logger,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────


// buildLogger creates a zap.Logger that respects the LOG_LEVEL and LOG_FORMAT
// configuration values.
func buildLogger(logCfg config.LogConfig) (*zap.Logger, error) {
	var level zap.AtomicLevel
	switch logCfg.Level {
	case "debug":
		level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default: // "info" or unrecognised
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	encoding := "json"
	if logCfg.Format == "console" || logCfg.Format == "text" {
		encoding = "console"
	}

	zapCfg := zap.Config{
		Level:            level,
		Encoding:         encoding,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig:    zap.NewProductionEncoderConfig(),
	}

	return zapCfg.Build()
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
