package admin

import (
	"context"
	"fmt"
	"time"

	"llm-router-platform/internal/models"
)

// FinancialDashboard contains operator-facing financial KPIs for a period.
type FinancialDashboard struct {
	PeriodStart         time.Time
	PeriodEnd           time.Time
	CashRevenue         float64
	NetCashRevenue      float64
	SubscriptionRevenue float64
	TopUpRevenue        float64
	UsageRevenue        float64
	ProviderCost        float64
	GrossProfit         float64
	GrossMargin         float64
	RefundAmount        float64
	CreditGrants        float64
	OutstandingBalance  float64
	PaidOrders          int
	ActiveSubscriptions int
	PayingCustomers     int
	ARPU                float64
	Daily               []FinancialDailyPoint
	PaymentBreakdown    []FinancialBreakdown
	ProviderBreakdown   []FinancialProviderBreakdown
}

type FinancialDailyPoint struct {
	Date         string
	CashRevenue  float64
	UsageRevenue float64
	ProviderCost float64
	GrossProfit  float64
	Orders       int
	Requests     int
}

type FinancialBreakdown struct {
	Name   string
	Amount float64
	Count  int
}

type FinancialProviderBreakdown struct {
	ProviderName string
	Requests     int
	UsageRevenue float64
	ProviderCost float64
	GrossProfit  float64
	GrossMargin  float64
}

type financialOrderSummary struct {
	CashRevenue         float64 `gorm:"column:cash_revenue"`
	SubscriptionRevenue float64 `gorm:"column:subscription_revenue"`
	TopUpRevenue        float64 `gorm:"column:top_up_revenue"`
	PaidOrders          int64   `gorm:"column:paid_orders"`
	PayingCustomers     int64   `gorm:"column:paying_customers"`
}

type financialUsageSummary struct {
	UsageRevenue float64 `gorm:"column:usage_revenue"`
	ProviderCost float64 `gorm:"column:provider_cost"`
	Requests     int64   `gorm:"column:requests"`
}

// FinancialDashboard returns cash, usage revenue, cost, and margin metrics.
func (s *Service) FinancialDashboard(ctx context.Context, days int) (*FinancialDashboard, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := todayStart.AddDate(0, 0, -(days - 1))

	var orders financialOrderSummary
	if err := s.db.WithContext(ctx).Table("orders").
		Select(`COALESCE(SUM(amount), 0) AS cash_revenue,
			COALESCE(SUM(CASE WHEN order_no LIKE 'ORD-%' OR (plan_id IS NOT NULL AND plan_id::text <> '00000000-0000-0000-0000-000000000000') THEN amount ELSE 0 END), 0) AS subscription_revenue,
			COALESCE(SUM(CASE WHEN order_no NOT LIKE 'ORD-%' AND (plan_id IS NULL OR plan_id::text = '00000000-0000-0000-0000-000000000000') THEN amount ELSE 0 END), 0) AS top_up_revenue,
			COUNT(*) AS paid_orders,
			COUNT(DISTINCT org_id) AS paying_customers`).
		Where("status = ? AND amount > 0 AND COALESCE(updated_at, created_at) >= ? AND COALESCE(updated_at, created_at) <= ?", "paid", start, now).
		Scan(&orders).Error; err != nil {
		return nil, fmt.Errorf("load financial order summary: %w", err)
	}

	var usage financialUsageSummary
	if err := s.db.WithContext(ctx).Table("usage_logs").
		Select(`COALESCE(SUM(COALESCE(customer_charge, cost, 0)), 0) AS usage_revenue,
			COALESCE(SUM(COALESCE(provider_cost, cost, 0)), 0) AS provider_cost,
			COUNT(*) AS requests`).
		Where("created_at >= ? AND created_at <= ?", start, now).
		Where("status_code >= 200 AND status_code < 300").
		Scan(&usage).Error; err != nil {
		return nil, fmt.Errorf("load financial usage summary: %w", err)
	}

	var refundAmount float64
	if err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", "refund", start, now).
		Select("COALESCE(SUM(ABS(amount)), 0)").Scan(&refundAmount).Error; err != nil {
		return nil, fmt.Errorf("load financial refunds: %w", err)
	}

	var creditGrants float64
	if err := s.db.WithContext(ctx).Table("transactions AS t").
		Joins("LEFT JOIN orders AS o ON t.reference_id = o.order_no AND o.status = ?", "paid").
		Where("t.amount > 0 AND t.type <> ? AND t.created_at >= ? AND t.created_at <= ? AND o.id IS NULL", "refund", start, now).
		Select("COALESCE(SUM(t.amount), 0)").Scan(&creditGrants).Error; err != nil {
		return nil, fmt.Errorf("load financial credit grants: %w", err)
	}

	var outstandingBalance float64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Select("COALESCE(SUM(CASE WHEN balance > 0 THEN balance ELSE 0 END), 0)").
		Scan(&outstandingBalance).Error; err != nil {
		return nil, fmt.Errorf("load outstanding balance: %w", err)
	}

	var activeSubscriptions int64
	if err := s.db.WithContext(ctx).Model(&models.Subscription{}).
		Where("status IN ?", []string{"active", "trialing"}).
		Count(&activeSubscriptions).Error; err != nil {
		return nil, fmt.Errorf("load active subscriptions: %w", err)
	}

	grossProfit := usage.UsageRevenue - usage.ProviderCost
	daily, err := s.financialDaily(ctx, start, now, days)
	if err != nil {
		return nil, err
	}
	paymentBreakdown, err := s.financialPaymentBreakdown(ctx, start, now)
	if err != nil {
		return nil, err
	}
	providerBreakdown, err := s.financialProviderBreakdown(ctx, start, now)
	if err != nil {
		return nil, err
	}

	return &FinancialDashboard{
		PeriodStart:         start,
		PeriodEnd:           now,
		CashRevenue:         orders.CashRevenue,
		NetCashRevenue:      orders.CashRevenue - refundAmount,
		SubscriptionRevenue: orders.SubscriptionRevenue,
		TopUpRevenue:        orders.TopUpRevenue,
		UsageRevenue:        usage.UsageRevenue,
		ProviderCost:        usage.ProviderCost,
		GrossProfit:         grossProfit,
		GrossMargin:         percent(grossProfit, usage.UsageRevenue),
		RefundAmount:        refundAmount,
		CreditGrants:        creditGrants,
		OutstandingBalance:  outstandingBalance,
		PaidOrders:          int(orders.PaidOrders),
		ActiveSubscriptions: int(activeSubscriptions),
		PayingCustomers:     int(orders.PayingCustomers),
		ARPU:                ratio(orders.CashRevenue, float64(orders.PayingCustomers)),
		Daily:               daily,
		PaymentBreakdown:    paymentBreakdown,
		ProviderBreakdown:   providerBreakdown,
	}, nil
}

func (s *Service) financialDaily(ctx context.Context, start, end time.Time, days int) ([]FinancialDailyPoint, error) {
	byDate := make(map[string]*FinancialDailyPoint, days)
	out := make([]FinancialDailyPoint, 0, days)
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		point := &FinancialDailyPoint{Date: date}
		byDate[date] = point
		out = append(out, *point)
	}

	var orderRows []struct {
		Date        string  `gorm:"column:date"`
		CashRevenue float64 `gorm:"column:cash_revenue"`
		Orders      int64   `gorm:"column:orders"`
	}
	if err := s.db.WithContext(ctx).Table("orders").
		Select(`TO_CHAR(COALESCE(updated_at, created_at), 'YYYY-MM-DD') AS date,
			COALESCE(SUM(amount), 0) AS cash_revenue,
			COUNT(*) AS orders`).
		Where("status = ? AND amount > 0 AND COALESCE(updated_at, created_at) >= ? AND COALESCE(updated_at, created_at) <= ?", "paid", start, end).
		Group("TO_CHAR(COALESCE(updated_at, created_at), 'YYYY-MM-DD')").
		Scan(&orderRows).Error; err != nil {
		return nil, fmt.Errorf("load financial daily orders: %w", err)
	}
	for _, row := range orderRows {
		if point := byDate[row.Date]; point != nil {
			point.CashRevenue = row.CashRevenue
			point.Orders = int(row.Orders)
		}
	}

	var usageRows []struct {
		Date         string  `gorm:"column:date"`
		UsageRevenue float64 `gorm:"column:usage_revenue"`
		ProviderCost float64 `gorm:"column:provider_cost"`
		Requests     int64   `gorm:"column:requests"`
	}
	if err := s.db.WithContext(ctx).Table("usage_logs").
		Select(`TO_CHAR(created_at, 'YYYY-MM-DD') AS date,
			COALESCE(SUM(COALESCE(customer_charge, cost, 0)), 0) AS usage_revenue,
			COALESCE(SUM(COALESCE(provider_cost, cost, 0)), 0) AS provider_cost,
			COUNT(*) AS requests`).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Where("status_code >= 200 AND status_code < 300").
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Scan(&usageRows).Error; err != nil {
		return nil, fmt.Errorf("load financial daily usage: %w", err)
	}
	for _, row := range usageRows {
		if point := byDate[row.Date]; point != nil {
			point.UsageRevenue = row.UsageRevenue
			point.ProviderCost = row.ProviderCost
			point.GrossProfit = row.UsageRevenue - row.ProviderCost
			point.Requests = int(row.Requests)
		}
	}

	for i := range out {
		if point := byDate[out[i].Date]; point != nil {
			out[i] = *point
		}
	}
	return out, nil
}

func (s *Service) financialPaymentBreakdown(ctx context.Context, start, end time.Time) ([]FinancialBreakdown, error) {
	var rows []struct {
		Name   string  `gorm:"column:name"`
		Amount float64 `gorm:"column:amount"`
		Count  int64   `gorm:"column:count"`
	}
	if err := s.db.WithContext(ctx).Table("orders").
		Select("COALESCE(NULLIF(payment_method, ''), 'unknown') AS name, COALESCE(SUM(amount), 0) AS amount, COUNT(*) AS count").
		Where("status = ? AND amount > 0 AND COALESCE(updated_at, created_at) >= ? AND COALESCE(updated_at, created_at) <= ?", "paid", start, end).
		Group("COALESCE(NULLIF(payment_method, ''), 'unknown')").
		Order("amount DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load financial payment breakdown: %w", err)
	}

	out := make([]FinancialBreakdown, len(rows))
	for i, row := range rows {
		out[i] = FinancialBreakdown{Name: row.Name, Amount: row.Amount, Count: int(row.Count)}
	}
	return out, nil
}

func (s *Service) financialProviderBreakdown(ctx context.Context, start, end time.Time) ([]FinancialProviderBreakdown, error) {
	var rows []struct {
		ProviderName string  `gorm:"column:provider_name"`
		Requests     int64   `gorm:"column:requests"`
		UsageRevenue float64 `gorm:"column:usage_revenue"`
		ProviderCost float64 `gorm:"column:provider_cost"`
	}
	if err := s.db.WithContext(ctx).Table("usage_logs").
		Joins("LEFT JOIN providers ON usage_logs.provider_id = providers.id").
		Select(`COALESCE(providers.name, 'unknown') AS provider_name,
			COUNT(*) AS requests,
			COALESCE(SUM(COALESCE(usage_logs.customer_charge, usage_logs.cost, 0)), 0) AS usage_revenue,
			COALESCE(SUM(COALESCE(usage_logs.provider_cost, usage_logs.cost, 0)), 0) AS provider_cost`).
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end).
		Where("usage_logs.status_code >= 200 AND usage_logs.status_code < 300").
		Group("providers.name").
		Order("usage_revenue DESC").
		Limit(10).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load financial provider breakdown: %w", err)
	}

	out := make([]FinancialProviderBreakdown, len(rows))
	for i, row := range rows {
		grossProfit := row.UsageRevenue - row.ProviderCost
		out[i] = FinancialProviderBreakdown{
			ProviderName: row.ProviderName,
			Requests:     int(row.Requests),
			UsageRevenue: row.UsageRevenue,
			ProviderCost: row.ProviderCost,
			GrossProfit:  grossProfit,
			GrossMargin:  percent(grossProfit, row.UsageRevenue),
		}
	}
	return out, nil
}

func ratio(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func percent(numerator, denominator float64) float64 {
	return ratio(numerator, denominator) * 100
}
