// Package redeem provides redemption code management services.
package redeem

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	codeTypeCredit = "credit"
	codeTypePlan   = "plan"
)

// Service manages redeem code operations.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new redeem code service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// RedeemResult represents the outcome of a code redemption.
type RedeemResult struct {
	Success      bool
	Message      string
	CreditAmount decimal.Decimal
	PlanName     string
}

// Redeem consumes a code for the given user.
func (s *Service) Redeem(userID uuid.UUID, code string) (*RedeemResult, error) {
	code = strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(code, "\n", ""), "\r", "")))
	if code == "" {
		return &RedeemResult{Success: false, Message: "Code is required"}, nil
	}

	now := time.Now()
	result := &RedeemResult{}
	logType := ""

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var rc models.RedeemCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&rc).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				result.Success = false
				result.Message = "Invalid code"
				return nil
			}
			return err
		}

		if !rc.IsActive {
			result.Success = false
			result.Message = "Code has been revoked"
			return nil
		}
		if rc.UsedByID != nil {
			result.Success = false
			result.Message = "Code has already been used"
			return nil
		}
		if rc.ExpiresAt != nil && rc.ExpiresAt.Before(now) {
			result.Success = false
			result.Message = "Code has expired"
			return nil
		}

		codeType := normalizeCodeType(rc.Type)
		logType = codeType
		// Older admin UI could generate type=plan codes without a plan_id while
		// still carrying a credit amount. Those were no-op codes, so redeem them
		// as credit to preserve the operator's visible intent.
		legacyCreditCode := codeType == codeTypePlan && rc.PlanID == nil && rc.CreditAmount.IsPositive()
		if codeType == codeTypeCredit || legacyCreditCode {
			logType = codeTypeCredit
			return s.redeemCredit(tx, &rc, userID, now, result)
		}

		if codeType == codeTypePlan {
			if rc.PlanID == nil {
				result.Success = false
				result.Message = "Plan code is missing a plan"
				return nil
			}
			var plan models.Plan
			if err := tx.First(&plan, "id = ?", rc.PlanID).Error; err != nil {
				return fmt.Errorf("plan not found: %w", err)
			}
			return s.redeemPlan(tx, &rc, userID, now, &plan, result)
		}

		result.Success = false
		result.Message = "Unsupported code type"
		return nil
	})
	if err != nil {
		s.logger.Error("redeem code failed", zap.Error(err), zap.String("code", code))
		return nil, err
	}

	if result.Success {
		s.logger.Info("code redeemed",
			zap.String("code", code),
			zap.String("user_id", userID.String()),
			zap.String("type", logType),
		)
	}
	return result, nil
}

// UserHistory returns the redeem history for a user.
func (s *Service) UserHistory(userID uuid.UUID) ([]models.RedeemCode, error) {
	var codes []models.RedeemCode
	err := s.db.Where("used_by_id = ?", userID).
		Order("used_at DESC").
		Preload("Plan").
		Find(&codes).Error
	return codes, err
}

// ListCodes returns all codes with pagination (admin).
func (s *Service) ListCodes(page, pageSize int) ([]models.RedeemCode, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	s.db.Model(&models.RedeemCode{}).Count(&total)

	var codes []models.RedeemCode
	err := s.db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&codes).Error
	return codes, total, err
}

// GenerateCodes creates a batch of redeem codes.
func (s *Service) GenerateCodes(
	codeType string,
	creditAmount decimal.Decimal,
	planID *uuid.UUID,
	planDays int,
	count int,
	expiresAt *time.Time,
	note string,
) ([]string, error) {
	codeType = normalizeCodeType(codeType)
	if count < 1 || count > 1000 {
		return nil, fmt.Errorf("count must be between 1 and 1000")
	}
	if codeType != codeTypeCredit && codeType != codeTypePlan {
		return nil, fmt.Errorf("unsupported redeem code type: %s", codeType)
	}
	creditAmount = creditAmount.Round(models.MoneyScale)
	if codeType == codeTypeCredit && !creditAmount.IsPositive() {
		return nil, fmt.Errorf("credit amount must be positive")
	}
	if codeType == codeTypePlan && planID == nil {
		return nil, fmt.Errorf("plan code requires a plan")
	}
	if planDays <= 0 {
		planDays = 30
	}

	batchID := uuid.New().String()[:8]
	codes := make([]string, 0, count)
	records := make([]models.RedeemCode, 0, count)

	for i := 0; i < count; i++ {
		code := generateCode()
		codes = append(codes, code)
		records = append(records, models.RedeemCode{
			Code:         code,
			Type:         codeType,
			CreditAmount: creditAmount,
			PlanID:       planID,
			PlanDays:     planDays,
			ExpiresAt:    expiresAt,
			IsActive:     true,
			BatchID:      batchID,
			Note:         note,
		})
	}

	if err := s.db.Create(&records).Error; err != nil {
		return nil, err
	}

	s.logger.Info("redeem codes generated",
		zap.Int("count", count),
		zap.String("type", codeType),
		zap.String("batch_id", batchID),
	)
	return codes, nil
}

// RevokeCode deactivates a redeem code.
func (s *Service) RevokeCode(id uuid.UUID) error {
	return s.db.Model(&models.RedeemCode{}).
		Where("id = ? AND used_by_id IS NULL", id).
		Update("is_active", false).Error
}

func (s *Service) redeemCredit(tx *gorm.DB, rc *models.RedeemCode, userID uuid.UUID, redeemedAt time.Time, result *RedeemResult) error {
	if !rc.CreditAmount.IsPositive() {
		result.Success = false
		result.Message = "Credit amount must be positive"
		return nil
	}

	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		return err
	}

	amount := rc.CreditAmount.Round(models.MoneyScale)
	user.Balance = models.MoneyAdd(user.Balance, amount)
	if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance", user.Balance).Error; err != nil {
		return err
	}

	updates := map[string]any{
		"used_by_id": userID,
		"used_at":    redeemedAt,
	}
	if normalizeCodeType(rc.Type) == codeTypePlan && rc.PlanID == nil {
		updates["type"] = codeTypeCredit
	}
	if err := tx.Model(rc).Updates(updates).Error; err != nil {
		return err
	}

	transaction := &models.Transaction{
		OrgID:       userID,
		UserID:      userID,
		Type:        "recharge",
		Amount:      amount,
		Balance:     user.Balance,
		Description: "Redeem code credit",
		ReferenceID: rc.ID.String(),
	}
	if err := tx.Create(transaction).Error; err != nil {
		return err
	}

	result.Success = true
	result.CreditAmount = amount
	result.Message = fmt.Sprintf("$%s credit added to your account", amount.Round(2).StringFixed(2))
	return nil
}

func (s *Service) redeemPlan(tx *gorm.DB, rc *models.RedeemCode, userID uuid.UUID, redeemedAt time.Time, plan *models.Plan, result *RedeemResult) error {
	orgID, err := findPrimaryOrgID(tx, userID)
	if err != nil {
		return err
	}

	planDays := rc.PlanDays
	if planDays <= 0 {
		planDays = 30
	}
	periodStart := redeemedAt
	periodEnd := redeemedAt.AddDate(0, 0, planDays)

	var sub models.Subscription
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sub, "org_id = ?", orgID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		sub = models.Subscription{
			OrgID:              orgID,
			PlanID:             plan.ID,
			Status:             "active",
			CurrentPeriodStart: periodStart,
			CurrentPeriodEnd:   periodEnd,
		}
		if err := tx.Create(&sub).Error; err != nil {
			return err
		}
	} else {
		if sub.CurrentPeriodEnd.After(redeemedAt) {
			periodStart = sub.CurrentPeriodStart
			periodEnd = sub.CurrentPeriodEnd.AddDate(0, 0, planDays)
		}
		if err := tx.Model(&sub).Updates(map[string]any{
			"plan_id":              plan.ID,
			"status":               "active",
			"current_period_start": periodStart,
			"current_period_end":   periodEnd,
			"cancel_at_period_end": false,
		}).Error; err != nil {
			return err
		}
	}

	if err := tx.Model(rc).Updates(map[string]any{
		"used_by_id": userID,
		"used_at":    redeemedAt,
	}).Error; err != nil {
		return err
	}

	result.Success = true
	result.PlanName = plan.Name
	result.Message = fmt.Sprintf("Plan '%s' activated for %d days", plan.Name, planDays)
	return nil
}

func findPrimaryOrgID(tx *gorm.DB, userID uuid.UUID) (uuid.UUID, error) {
	var membership models.OrganizationMember
	err := tx.
		Order("CASE role WHEN 'OWNER' THEN 0 WHEN 'ADMIN' THEN 1 WHEN 'MEMBER' THEN 2 ELSE 3 END").
		First(&membership, "user_id = ?", userID).Error
	if err == nil {
		return membership.OrgID, nil
	}
	if err != gorm.ErrRecordNotFound {
		return uuid.Nil, err
	}

	var org models.Organization
	if err := tx.First(&org, "owner_id = ?", userID).Error; err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func normalizeCodeType(codeType string) string {
	codeType = strings.ReplaceAll(strings.ReplaceAll(codeType, "\n", ""), "\r", "")
	return strings.ToLower(strings.TrimSpace(codeType))
}

// generateCode produces a random code like "ABCD-1234-EFGH".
func generateCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // No I, O, 0, 1 for clarity
	segments := make([]string, 3)
	for s := 0; s < 3; s++ {
		seg := make([]byte, 4)
		for i := range seg {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			seg[i] = chars[n.Int64()]
		}
		segments[s] = string(seg)
	}
	return strings.Join(segments, "-")
}
