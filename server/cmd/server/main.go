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

	"llm-router-platform/internal/api/routes"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/database"
	"llm-router-platform/internal/repository"
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

	if err := db.Migrate(); err != nil {
		return err
	}

	_ = db.SeedDefaultProviders()
	_ = db.SeedDefaultModels()
	_ = db.SeedDefaultAdmin(&cfg.Admin)

	gormDB := db.DB
	repos := initRepositories(db)

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

	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

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
	workerPool := task.NewWorkerPool(services.TaskService, gormDB, task.DefaultWorkerPoolConfig(), logger)
	task.RegisterDefaultExecutors(workerPool, logger)
	go workerPool.Start(lifecycleCtx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Periodic data cleanup (runs daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = db.CleanupOldHealthHistory(30) // 30-day retention
				_, _ = db.CleanupOldAlerts(90)        // 90-day retention for resolved alerts
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
}

func initRepositories(db *database.Database) *Repositories {
	return &Repositories{
		User:           repository.NewUserRepository(db.DB),
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
	}
}

func initServices(repos *Repositories, cfg *config.Config, logger *zap.Logger, redisClient *redis.Client, gormDB *gorm.DB) *routes.Services {
	userService := user.NewService(repos.User, repos.APIKey, logger)

	// Provider registry - clients are created dynamically based on database configuration
	// API keys are stored encrypted in the database and configured via Web UI
	providerRegistry := provider.NewRegistry(logger)

	routerService := router.NewRouter(repos.Provider, repos.ProviderAPIKey, repos.Proxy, repos.Model, providerRegistry, logger)
	if redisClient != nil {
		routerService.SetRedisClient(redisClient)
	}
	billingService := billing.NewService(repos.UsageLog, repos.Model, redisClient, logger)
	budgetService := billing.NewBudgetService(repos.UsageLog, repos.Budget, logger)
	memoryService := memory.NewService(repos.Memory, redisClient, logger)
	proxyService := proxy.NewService(repos.Proxy, logger)
	obsService := observability.NewLangfuseService(cfg.Observability, logger)

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

	taskService := task.NewService(gormDB, logger)

	return &routes.Services{
		User:          userService,
		Router:        routerService,
		Billing:       billingService,
		BudgetService: budgetService,
		Health:        healthService,
		Memory:        memoryService,
		Observability: obsService,
		Proxy:         proxyService,
		Provider:      providerRegistry,
		TaskService:   taskService,
		RedisClient:   redisClient,
		DB:            gormDB, // For health checks
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
