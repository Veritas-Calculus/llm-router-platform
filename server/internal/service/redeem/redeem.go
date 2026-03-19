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
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	CreditAmount float64
	PlanName     string
}

// Redeem consumes a code for the given user.
func (s *Service) Redeem(userID uuid.UUID, code string) (*RedeemResult, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return &RedeemResult{Success: false, Message: "Code is required"}, nil
	}

	var rc models.RedeemCode
	if err := s.db.Where("code = ?", code).First(&rc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &RedeemResult{Success: false, Message: "Invalid code"}, nil
		}
		return nil, err
	}

	if !rc.IsActive {
		return &RedeemResult{Success: false, Message: "Code has been revoked"}, nil
	}
	if rc.UsedByID != nil {
		return &RedeemResult{Success: false, Message: "Code has already been used"}, nil
	}
	if rc.ExpiresAt != nil && rc.ExpiresAt.Before(time.Now()) {
		return &RedeemResult{Success: false, Message: "Code has expired"}, nil
	}

	now := time.Now()
	result := &RedeemResult{Success: true}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Mark code as used
		rc.UsedByID = &userID
		rc.UsedAt = &now
		if err := tx.Save(&rc).Error; err != nil {
			return err
		}

		switch rc.Type {
		case "credit":
			// Add balance
			if err := tx.Model(&models.User{}).Where("id = ?", userID).
				Update("balance", gorm.Expr("balance + ?", rc.CreditAmount)).Error; err != nil {
				return err
			}
			result.CreditAmount = rc.CreditAmount
			result.Message = fmt.Sprintf("$%.2f credit added to your account", rc.CreditAmount)

		case "plan":
			if rc.PlanID != nil {
				var plan models.Plan
				if err := tx.First(&plan, "id = ?", rc.PlanID).Error; err != nil {
					return fmt.Errorf("plan not found: %w", err)
				}
				// Create or extend subscription
				sub := models.Subscription{
					UserID:             userID,
					PlanID:             *rc.PlanID,
					Status:             "active",
					CurrentPeriodStart: now,
					CurrentPeriodEnd:   now.AddDate(0, 0, rc.PlanDays),
				}
				if err := tx.Where("user_id = ?", userID).
					Assign(sub).
					FirstOrCreate(&sub).Error; err != nil {
					return err
				}
				result.PlanName = plan.Name
				result.Message = fmt.Sprintf("Plan '%s' activated for %d days", plan.Name, rc.PlanDays)
			}
		}
		return nil
	})
	if err != nil {
		s.logger.Error("redeem code failed", zap.Error(err), zap.String("code", code))
		return nil, err
	}

	s.logger.Info("code redeemed",
		zap.String("code", code),
		zap.String("user_id", userID.String()),
		zap.String("type", rc.Type),
	)
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
	creditAmount float64,
	planID *uuid.UUID,
	planDays int,
	count int,
	expiresAt *time.Time,
	note string,
) ([]string, error) {
	if count < 1 || count > 1000 {
		return nil, fmt.Errorf("count must be between 1 and 1000")
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
