package user

import (
	"context"
	"fmt"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ─── Organization Role Authorization ────────────────────────────────────

// RequireOrgRole checks if userID has at least one of the allowed roles in orgID.
// Global admins bypass org-level checks for management flexibility.
func (s *Service) RequireOrgRole(ctx context.Context, userID, orgID string, allowedRoles ...string) error {
	var member struct{ Role string }
	err := s.orgRepo.DB().Table("organization_members").
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
			return nil
		}
	}

	// Global admins bypass org-level checks
	var u struct{ Role string }
	if err := s.orgRepo.DB().Table("users").Select("role").Where("id = ?", userID).First(&u).Error; err == nil && u.Role == "admin" {
		return nil
	}

	return fmt.Errorf("forbidden: requires one of %v roles", allowedRoles)
}

// RequireProjectRole checks if the user has a sufficient role in the org that owns the project.
func (s *Service) RequireProjectRole(ctx context.Context, userID, projectID string, allowedRoles ...string) error {
	var project struct{ OrgID string }
	if err := s.orgRepo.DB().Table("projects").Select("org_id").Where("id = ?", projectID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("project not found")
		}
		return fmt.Errorf("database error verifying project ownership: %w", err)
	}
	return s.RequireOrgRole(ctx, userID, project.OrgID, allowedRoles...)
}

// ─── Organization Member Management ─────────────────────────────────────

// AddOrgMember adds a user (by email) to an organization with a given role.
func (s *Service) AddOrgMember(ctx context.Context, orgID, email, role string) (*models.OrganizationMember, error) {
	var targetUser models.User
	if err := s.orgRepo.DB().Where("email = ?", email).First(&targetUser).Error; err != nil {
		return nil, fmt.Errorf("user with email %s not found", email)
	}

	var existing models.OrganizationMember
	if err := s.orgRepo.DB().Where("org_id = ? AND user_id = ?", orgID, targetUser.ID).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("user is already a member of this organization")
	}

	newMember := models.OrganizationMember{
		OrgID:  uuid.MustParse(orgID),
		UserID: targetUser.ID,
		Role:   role,
	}
	if err := s.orgRepo.DB().Create(&newMember).Error; err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	newMember.User = targetUser
	return &newMember, nil
}

// UpdateOrgMemberRole updates a member's role within an organization.
func (s *Service) UpdateOrgMemberRole(ctx context.Context, orgID, userID, role string) (*models.OrganizationMember, error) {
	var member models.OrganizationMember
	if err := s.orgRepo.DB().Preload("User").Where("org_id = ? AND user_id = ?", orgID, userID).First(&member).Error; err != nil {
		return nil, fmt.Errorf("member not found")
	}

	if err := s.orgRepo.DB().Model(&member).Where("org_id = ? AND user_id = ?", orgID, userID).Update("role", role).Error; err != nil {
		return nil, err
	}
	member.Role = role
	return &member, nil
}

// RemoveOrgMember removes a user from an organization.
func (s *Service) RemoveOrgMember(ctx context.Context, orgID, userID string) error {
	var member models.OrganizationMember
	if err := s.orgRepo.DB().Where("org_id = ? AND user_id = ?", orgID, userID).First(&member).Error; err != nil {
		return fmt.Errorf("member not found")
	}
	if member.Role == "OWNER" {
		return fmt.Errorf("cannot remove an organization owner")
	}
	return s.orgRepo.DB().Where("org_id = ? AND user_id = ?", orgID, userID).Delete(&models.OrganizationMember{}).Error
}

// ─── Identity Providers ─────────────────────────────────────────────────

// CreateIdentityProvider creates a new SSO identity provider for an org.
func (s *Service) CreateIdentityProvider(ctx context.Context, idp *models.IdentityProvider) error {
	if err := s.orgRepo.DB().Create(idp).Error; err != nil {
		return fmt.Errorf("failed to create identity provider: %w", err)
	}
	s.orgRepo.DB().Preload("Organization").First(idp, "id = ?", idp.ID)
	return nil
}

// GetIdentityProvider retrieves an IDP by ID.
func (s *Service) GetIdentityProvider(ctx context.Context, id uuid.UUID) (*models.IdentityProvider, error) {
	var idp models.IdentityProvider
	if err := s.orgRepo.DB().First(&idp, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("identity provider not found")
	}
	return &idp, nil
}

// UpdateIdentityProvider saves changes to an IDP.
func (s *Service) UpdateIdentityProvider(ctx context.Context, idp *models.IdentityProvider) error {
	if err := s.orgRepo.DB().Save(idp).Error; err != nil {
		return fmt.Errorf("failed to update identity provider: %w", err)
	}
	s.orgRepo.DB().Preload("Organization").First(idp, "id = ?", idp.ID)
	return nil
}

// DeleteIdentityProvider removes an IDP by ID.
func (s *Service) DeleteIdentityProvider(ctx context.Context, id uuid.UUID) error {
	var idp models.IdentityProvider
	if err := s.orgRepo.DB().First(&idp, "id = ?", id).Error; err != nil {
		return fmt.Errorf("identity provider not found")
	}
	return s.orgRepo.DB().Delete(&idp).Error
}

// ─── Invite Codes ───────────────────────────────────────────────────────

// ValidateAndConsumeInviteCode atomically validates and increments an invite code's usage.
func (s *Service) ValidateAndConsumeInviteCode(ctx context.Context, code string) error {
	return s.orgRepo.DB().Transaction(func(tx *gorm.DB) error {
		var ic models.InviteCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("code = ?", code).First(&ic).Error; err != nil {
			return fmt.Errorf("invalid invite code")
		}
		if !ic.IsValid() {
			return fmt.Errorf("invite code is expired or exhausted")
		}
		return tx.Model(&ic).UpdateColumn("use_count", gorm.Expr("use_count + 1")).Error
	})
}

// CreateInviteCode creates a new invite code.
func (s *Service) CreateInviteCode(ctx context.Context, code string, createdBy uuid.UUID, maxUses int, expiresAt *time.Time) (*models.InviteCode, error) {
	ic := models.InviteCode{
		Code: code, CreatedBy: createdBy, MaxUses: maxUses, ExpiresAt: expiresAt, IsActive: true,
	}
	if err := s.orgRepo.DB().Create(&ic).Error; err != nil {
		return nil, err
	}
	return &ic, nil
}

// ─── Usage Export ────────────────────────────────────────────────────────

// ExportUserUsageCSV exports the last 30 days of usage for a user.
func (s *Service) ExportUserUsageCSV(ctx context.Context, userID uuid.UUID) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	since := time.Now().AddDate(0, 0, -30)
	if err := s.orgRepo.DB().Where("user_id = ? AND created_at >= ?", userID, since).
		Order("created_at DESC").Limit(10000).Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to query usage: %w", err)
	}
	return logs, nil
}

// ─── System Config (Langfuse/SMTP) ──────────────────────────────────────

// GetSystemConfigByKey retrieves a system configuration entry by key.
func (s *Service) GetSystemConfigByKey(ctx context.Context, key string) (*models.SystemConfig, error) {
	var conf models.SystemConfig
	if err := s.orgRepo.DB().Where("key = ?", key).First(&conf).Error; err != nil {
		return nil, err
	}
	return &conf, nil
}

// SaveSystemConfig creates or updates a system config entry.
func (s *Service) SaveSystemConfig(ctx context.Context, conf *models.SystemConfig) error {
	var existing models.SystemConfig
	if err := s.orgRepo.DB().Where("key = ?", conf.Key).First(&existing).Error; err != nil {
		// Not found — create
		return s.orgRepo.DB().Create(conf).Error
	}
	// Found — update
	conf.ID = existing.ID
	return s.orgRepo.DB().Save(conf).Error
}

// ─── Helpers ────────────────────────────────────────────────────────────

// OnboardWithDB runs OnboardAccount using the underlying DB connection.
func (s *Service) OnboardWithDB(ctx context.Context, u *models.User, params OnboardAccountParams, logger *zap.Logger) error {
	return OnboardAccount(ctx, s.orgRepo.DB(), u, params, logger)
}
