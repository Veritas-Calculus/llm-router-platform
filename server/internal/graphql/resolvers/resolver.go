// Package resolvers implements GraphQL resolvers.
// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.
package resolvers

import (
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/admin"
	"llm-router-platform/internal/service/announcement"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/billing"
	semantic "llm-router-platform/internal/service/cache"
	"llm-router-platform/internal/service/captcha"
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
	"llm-router-platform/pkg/jwtsign"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Resolver is the root resolver holding all service dependencies.
// Infrastructure access (DB, Redis, Config) is encapsulated inside AdminSvc.
// Accessor methods provide backward compatibility during migration.
type Resolver struct {
	UserSvc          *user.Service
	PasswordResetSvc *user.PasswordResetService
	EmailVerifySvc   *user.EmailVerificationService
	Router           *router.Router
	Billing          *billing.Service
	BudgetService    *billing.BudgetService
	SubscriptionSvc  *billing.SubscriptionService
	Payment          *billing.PaymentService
	Balance          *billing.BalanceService
	SystemConfig     *configService.Service
	Health           *health.Service
	Memory           *memory.Service
	MCP              *mcp.Service
	Observability    observability.Service
	Proxy            *proxy.Service
	Provider         *provider.Registry
	TaskService      *task.Service
	AuditService     *audit.Service
	EmailService     *email.Service
	RedeemSvc        *redeem.Service
	AnnouncementSvc  *announcement.Service
	CouponSvc        *coupon.Service
	DocumentSvc      *document.Service
	WebhookSvc       webhook.Service
	MonitoringSvc    *monitoring.Collector
	TurnstileSvc     *turnstile.Service
	CaptchaSvc       *captcha.Service
	LoginLimiter     *user.LoginLimiter
	AdminSvc         *admin.Service
	Logger           *zap.Logger
	SemanticCache    *semantic.SemanticCacheService
	// JWTSigner mints + verifies access/refresh tokens. May be nil during
	// the transition; helpers_auth.go falls back to legacy HS256 in that
	// case so existing deployments continue to work without config change.
	JWTSigner *jwtsign.Signer
}

// ── Backward-compatible infrastructure accessors ────────────────────
// These delegate to AdminSvc so the Resolver struct no longer stores
// DB / Redis / Config directly. Existing resolver code calls r.DB, etc.
//
// IMPORTANT — direct DB access from resolvers is deprecated.
//
// CLAUDE.md defines the layering as Resolver → Service → Repository → Model.
// New code MUST go through a service method (extend the existing service
// package or create one if the domain doesn't have one yet). Calling r.DB()
// inside a resolver defeats testability (no mock seam), spreads query
// knowledge, and was the root cause of the SEC-C1 incident where an org-
// scope check on a write path could silently bypass org membership.
//
// The identity / sso / cache / dlp / prompt / org / billing resolver files
// still contain legacy direct calls (~100 sites at the time of writing).
// They are being migrated incrementally — see the BE-C5 task in the audit
// roadmap. Do not add new ones.

// DB returns the GORM database handle.
func (r *Resolver) DB() *gorm.DB { return r.AdminSvc.DB() }

// RedisClient returns the Redis client (may be nil).
func (r *Resolver) RedisClient() *redis.Client { return r.AdminSvc.Redis() }

// Config returns the application config.
func (r *Resolver) Config() *config.Config { return r.AdminSvc.Config() }
