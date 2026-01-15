// Package main provides the entry point for the LLM Router Platform server.
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
	"llm-router-platform/internal/database"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
	services := initServices(repos, cfg, logger)

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
	<-quit

	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

func initServices(repos *Repositories, cfg *config.Config, logger *zap.Logger) *routes.Services {
	userService := user.NewService(repos.User, repos.APIKey, logger)

	providerRegistry := provider.NewRegistry(logger)

	if cfg.Providers.OpenAI.APIKey != "" {
		openaiClient := provider.NewOpenAIClient(&cfg.Providers.OpenAI, logger)
		providerRegistry.Register("openai", openaiClient)
	}

	if cfg.Providers.Anthropic.APIKey != "" {
		anthropicClient := provider.NewAnthropicClient(&cfg.Providers.Anthropic, logger)
		providerRegistry.Register("anthropic", anthropicClient)
	}

	routerService := router.NewRouter(repos.Provider, repos.ProviderAPIKey, providerRegistry, logger)
	billingService := billing.NewService(repos.UsageLog, repos.Model, logger)
	memoryService := memory.NewService(repos.Memory, nil, logger)
	proxyService := proxy.NewService(repos.Proxy, logger)

	alertNotifier := health.NewAlertNotifier(repos.Alert, repos.AlertConfig, logger)
	healthService := health.NewService(
		repos.APIKey,
		repos.ProviderAPIKey,
		repos.Proxy,
		repos.HealthHistory,
		alertNotifier,
		providerRegistry,
		proxyService,
		logger,
	)

	return &routes.Services{
		User:     userService,
		Router:   routerService,
		Billing:  billingService,
		Health:   healthService,
		Memory:   memoryService,
		Proxy:    proxyService,
		Provider: providerRegistry,
	}
}
