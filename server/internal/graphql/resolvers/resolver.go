// Package resolvers implements GraphQL resolvers.
// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.
package resolvers

import (
	"llm-router-platform/internal/config"
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

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Resolver is the root resolver holding all service dependencies.
type Resolver struct {
	UserSvc       *user.Service
	PasswordResetSvc *user.PasswordResetService
	Router        *router.Router
	Billing       *billing.Service
	BudgetService *billing.BudgetService
	SubscriptionSvc *billing.SubscriptionService
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
	EmailService  *email.Service
	RedeemSvc     *redeem.Service
	AnnouncementSvc *announcement.Service
	CouponSvc     *coupon.Service
	DocumentSvc   *document.Service
	RedisClient   *redis.Client
	DB            *gorm.DB
	Config        *config.Config
	Logger        *zap.Logger
}
