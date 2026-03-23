// Package resolvers implements GraphQL resolvers.
// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.
package resolvers

import (
	"context"
	"fmt"
	
	"llm-router-platform/internal/config"
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

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Resolver is the root resolver holding all service dependencies.
type Resolver struct {
	UserSvc       *user.Service
	PasswordResetSvc *user.PasswordResetService
	EmailVerifySvc   *user.EmailVerificationService
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
	CouponSvc       *coupon.Service
	DocumentSvc     *document.Service
	WebhookSvc      webhook.Service
	MonitoringSvc   *monitoring.Collector
	TurnstileSvc    *turnstile.Service
	LoginLimiter    *user.LoginLimiter
	RedisClient     *redis.Client
	DB            *gorm.DB
	Config        *config.Config
	Logger        *zap.Logger
	SemanticCache *semantic.SemanticCacheService
}

// RequireOrgRole checks if the current user has at least one of the allowed roles in the specified organization.
func (r *Resolver) RequireOrgRole(ctx context.Context, userID, orgID string, allowedRoles ...string) error {
	var member struct {
		Role string
	}
	err := r.DB.Table("organization_members").
		Select("role").
		Where("org_id = ? AND user_id = ?", orgID, userID).
		First(&member).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("forbidden: user is not a member of this organization")
		}
		return fmt.Errorf("database error verifying organization role: %w", err)
	}

	for _, role := range allowedRoles {
		if member.Role == role || member.Role == "OWNER" {
			return nil // OWNER implicitly has all roles
		}
	}
	
	// Global admins bypass org-level checks for management flexibility
	var user struct {
		Role string
	}
	if err := r.DB.Table("users").Select("role").Where("id = ?", userID).First(&user).Error; err == nil && user.Role == "admin" {
		return nil
	}

	return fmt.Errorf("forbidden: requires one of %v roles", allowedRoles)
}

// RequireProjectRole checks if the user has a sufficient role in the organization that owns the project.
func (r *Resolver) RequireProjectRole(ctx context.Context, userID, projectID string, allowedRoles ...string) error {
	var project struct {
		OrgID string
	}
	if err := r.DB.Table("projects").Select("org_id").Where("id = ?", projectID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("project not found")
		}
		return fmt.Errorf("database error verifying project ownership: %w", err)
	}
	
	return r.RequireOrgRole(ctx, userID, project.OrgID, allowedRoles...)
}
