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

	repos := initRepositories(db)

	// Initialize Redis client for rate limiting
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("redis connection failed, rate limiting will be disabled", zap.Error(err))
		redisClient = nil
	} else {
		logger.Info("redis connected for rate limiting")
	}

	services := initServices(repos, cfg, logger, redisClient, db.DB)

	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	routes.Setup(engine, cfg, services, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("starting server", zap.String("port", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	if cfg.HealthCheck.Enabled {
		scheduler := health.NewScheduler(services.Health, cfg.HealthCheck.Interval, logger)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go scheduler.Start(ctx)
	}

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
			case <-quit:
				return
			}
		}
	}()

	<-quit

	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := services.Observability.Shutdown(ctx); err != nil {
		logger.Error("observability shutdown error", zap.Error(err))
	}

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
		return err
	}

	logger.Info("server stopped")
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
	}
}

func initServices(repos *Repositories, cfg *config.Config, logger *zap.Logger, redisClient *redis.Client, gormDB *gorm.DB) *routes.Services {
	userService := user.NewService(repos.User, repos.APIKey, logger)

	// Provider registry - clients are created dynamically based on database configuration
	// API keys are stored encrypted in the database and configured via Web UI
	providerRegistry := provider.NewRegistry(logger)

	routerService := router.NewRouter(repos.Provider, repos.ProviderAPIKey, repos.Proxy, repos.Model, providerRegistry, logger)
	billingService := billing.NewService(repos.UsageLog, repos.Model, logger)
	budgetService := billing.NewBudgetService(repos.UsageLog, logger)
	memoryService := memory.NewService(repos.Memory, nil, logger)
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
		RedisClient:   redisClient,
		DB:            gormDB, // For health checks
	}
}
