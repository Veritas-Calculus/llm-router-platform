package user

import (
	"context"
	"fmt"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// OnboardAccountParams contains the parameters for post-registration onboarding.
type OnboardAccountParams struct {
	GrantWelcomeCredit bool    // Whether to grant welcome credit
	WelcomeCreditUSD   float64 // Amount of welcome credit (default 5.0)
}

// OnboardAccount creates the default Organization, Project, membership, and
// optional welcome credit for a newly registered user. This runs inside a
// single DB transaction to ensure atomicity.
//
// This method MUST be called from a context that already has a valid *gorm.DB
// (injected via the onboarding flow, not the user service's default repos).
func OnboardAccount(ctx context.Context, db *gorm.DB, u *models.User, params OnboardAccountParams, logger *zap.Logger) error {
	if params.WelcomeCreditUSD == 0 {
		params.WelcomeCreditUSD = 5.0
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		orgName := "Default Org"
		if u.Name != "" {
			orgName = u.Name + "'s Org"
		}

		org := models.Organization{Name: orgName, OwnerID: u.ID}
		if err := tx.Create(&org).Error; err != nil {
			return fmt.Errorf("failed to create organization: %w", err)
		}

		member := models.OrganizationMember{OrgID: org.ID, UserID: u.ID, Role: "OWNER"}
		if err := tx.Create(&member).Error; err != nil {
			return fmt.Errorf("failed to create org member: %w", err)
		}

		project := models.Project{OrgID: org.ID, Name: "Default", Description: "Auto-created project"}
		if err := tx.Create(&project).Error; err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		if params.GrantWelcomeCredit {
			u.Balance = params.WelcomeCreditUSD
			if err := tx.Model(u).UpdateColumn("balance", params.WelcomeCreditUSD).Error; err != nil {
				return fmt.Errorf("failed to set welcome balance: %w", err)
			}
			txn := models.Transaction{
				OrgID:       org.ID,
				UserID:      u.ID,
				Type:        "recharge",
				Amount:      params.WelcomeCreditUSD,
				Balance:     params.WelcomeCreditUSD,
				Description: "Welcome credit",
				Currency:    "USD",
			}
			if err := tx.Create(&txn).Error; err != nil {
				return fmt.Errorf("failed to record welcome transaction: %w", err)
			}
		}

		logger.Info("onboarded new user",
			zap.String("user_id", u.ID.String()),
			zap.String("org_id", org.ID.String()),
			zap.Bool("credit_granted", params.GrantWelcomeCredit),
		)

		return nil
	})
}

// OnboardAccountForOAuth is a convenience wrapper for OAuth2 flows that have a
// *gorm.DB handle but not the full service stack.
func OnboardAccountForOAuth(db *gorm.DB, u *models.User, orgID *uuid.UUID, logger *zap.Logger) {
	if err := OnboardAccount(context.Background(), db, u, OnboardAccountParams{GrantWelcomeCredit: true}, logger); err != nil {
		logger.Error("failed to onboard OAuth user", zap.Error(err), zap.String("user_id", u.ID.String()))
	}
}
