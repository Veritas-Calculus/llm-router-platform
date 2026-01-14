// Package routes defines API routes.
package routes

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"llm-router-platform/internal/api/handlers"
	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/health"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/proxy"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/user"
)

// Services holds all service dependencies.
type Services struct {
	User     *user.Service
	Router   *router.Router
	Billing  *billing.Service
	Health   *health.Service
	Memory   *memory.Service
	Proxy    *proxy.Service
	Provider *provider.Registry
}

// Setup configures all API routes.
func Setup(
	engine *gin.Engine,
	cfg *config.Config,
	services *Services,
	logger *zap.Logger,
) {
	corsMiddleware := middleware.NewCORSMiddleware(nil)
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)
	recoveryMiddleware := middleware.NewRecoveryMiddleware(logger)

	engine.Use(corsMiddleware.Handle())
	engine.Use(loggingMiddleware.Log())
	engine.Use(recoveryMiddleware.Recover())

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	authMiddleware := middleware.NewAuthMiddleware(&cfg.JWT, services.User, logger)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.RequestsPerMinute, logger)

	authHandler := handlers.NewAuthHandler(services.User, &cfg.JWT, logger)
	apiKeyHandler := handlers.NewAPIKeyHandler(services.User, logger)
	chatHandler := handlers.NewChatHandler(services.Router, services.Billing, services.Memory, logger)
	modelHandler := handlers.NewModelHandler(services.Provider, logger)
	usageHandler := handlers.NewUsageHandler(services.Billing, logger)
	healthHandler := handlers.NewHealthHandler(services.Health, logger)
	alertHandler := handlers.NewAlertHandler(services.Health, logger)
	dashboardHandler := handlers.NewDashboardHandler(services.Billing, services.Health, logger)
	proxyHandler := handlers.NewProxyHandler(services.Proxy, logger)
	providerHandler := handlers.NewProviderHandler(services.Router, logger)

	api := engine.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			auth := v1.Group("/auth")
			{
				auth.POST("/register", authHandler.Register)
				auth.POST("/login", authHandler.Login)
				auth.POST("/refresh", authHandler.RefreshToken)
			}

			protected := v1.Group("")
			protected.Use(authMiddleware.JWT())
			{
				user := protected.Group("/user")
				{
					user.GET("/profile", authHandler.GetProfile)
					user.PUT("/profile", authHandler.UpdateProfile)
					user.PUT("/password", authHandler.ChangePassword)
				}

				keys := protected.Group("/api-keys")
				{
					keys.GET("", apiKeyHandler.List)
					keys.POST("", apiKeyHandler.Create)
					keys.DELETE("/:id", apiKeyHandler.Revoke)
				}

				usage := protected.Group("/usage")
				{
					usage.GET("/summary", usageHandler.GetSummary)
					usage.GET("/daily", usageHandler.GetDaily)
					usage.GET("/by-provider", usageHandler.GetByProvider)
					usage.GET("/recent", usageHandler.GetRecent)
				}

				healthGroup := protected.Group("/health")
				{
					healthGroup.GET("/api-keys", healthHandler.GetAPIKeysHealth)
					healthGroup.POST("/api-keys/:id/check", healthHandler.CheckAPIKey)
					healthGroup.GET("/proxies", healthHandler.GetProxiesHealth)
					healthGroup.POST("/proxies/:id/check", healthHandler.CheckProxy)
					healthGroup.GET("/history", healthHandler.GetHealthHistory)
				}

				alerts := protected.Group("/alerts")
				{
					alerts.GET("", alertHandler.List)
					alerts.POST("/:id/acknowledge", alertHandler.Acknowledge)
					alerts.POST("/:id/resolve", alertHandler.Resolve)
					alerts.GET("/config/:target_type/:target_id", alertHandler.GetConfig)
					alerts.PUT("/config", alertHandler.UpdateConfig)
				}

				dashboard := protected.Group("/dashboard")
				{
					dashboard.GET("/stats", dashboardHandler.GetStats)
					dashboard.GET("/overview", dashboardHandler.GetOverview)
				}

				proxies := protected.Group("/proxies")
				{
					proxies.GET("", proxyHandler.List)
					proxies.POST("", proxyHandler.Create)
					proxies.PUT("/:id", proxyHandler.Update)
					proxies.DELETE("/:id", proxyHandler.Delete)
					proxies.POST("/:id/toggle", proxyHandler.Toggle)
				}

				providers := protected.Group("/providers")
				{
					providers.GET("", providerHandler.List)
					providers.POST("/:id/toggle", providerHandler.Toggle)
					providers.GET("/:id/health", providerHandler.CheckHealth)
				}
			}

			chat := v1.Group("/chat")
			chat.Use(authMiddleware.APIKey())
			chat.Use(rateLimiter.Limit())
			{
				chat.POST("/completions", chatHandler.ChatCompletion)
			}

			models := v1.Group("/models")
			models.Use(authMiddleware.APIKey())
			{
				models.GET("", modelHandler.List)
			}
		}
	}
}
