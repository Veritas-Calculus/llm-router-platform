package admin

import (
	"context"
	"fmt"
	"time"

	"llm-router-platform/internal/models"

	"github.com/shopspring/decimal"
)

// FinancialDashboard contains operator-facing financial KPIs for a period.
type FinancialDashboard struct {
	PeriodStart         time.Time
	PeriodEnd           time.Time
	CashRevenue         decimal.Decimal
	NetCashRevenue      decimal.Decimal
	SubscriptionRevenue decimal.Decimal
	TopUpRevenue        decimal.Decimal
	UsageRevenue        decimal.Decimal
	ProviderCost        decimal.Decimal
	GrossProfit         decimal.Decimal
	GrossMargin         float64
	RefundAmount        decimal.Decimal
	CreditGrants        decimal.Decimal
	OutstandingBalance  decimal.Decimal
	PaidOrders          int
	ActiveSubscriptions int
	PayingCustomers     int
	ARPU                decimal.Decimal
	Daily               []FinancialDailyPoint
	PaymentBreakdown    []FinancialBreakdown
	ProviderBreakdown   []FinancialProviderBreakdown
}

type FinancialDailyPoint struct {
	Date         string
	CashRevenue  decimal.Decimal
	UsageRevenue decimal.Decimal
	ProviderCost decimal.Decimal
	GrossProfit  decimal.Decimal
	Orders       int
	Requests     int
}

type FinancialBreakdown struct {
	Name   string
	Amount decimal.Decimal
	Count  int
}

type FinancialProviderBreakdown struct {
	ProviderName string
	Requests     int
	UsageRevenue decimal.Decimal
	ProviderCost decimal.Decimal
	GrossProfit  decimal.Decimal
	GrossMargin  float64
}

type financialOrderSummary struct {
	CashRevenue         decimal.Decimal `gorm:"column:cash_revenue"`
	SubscriptionRevenue decimal.Decimal `gorm:"column:subscription_revenue"`
	TopUpRevenue        decimal.Decimal `gorm:"column:top_up_revenue"`
	PaidOrders          int64           `gorm:"column:paid_orders"`
	PayingCustomers     int64           `gorm:"column:paying_customers"`
}

type financialUsageSummary struct {
	UsageRevenue decimal.Decimal `gorm:"column:usage_revenue"`
	ProviderCost decimal.Decimal `gorm:"column:provider_cost"`
	Requests     int64           `gorm:"column:requests"`
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

	var refundAmount decimal.Decimal
	if err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", "refund", start, now).
		Select("COALESCE(SUM(ABS(amount)), 0)").Scan(&refundAmount).Error; err != nil {
		return nil, fmt.Errorf("load financial refunds: %w", err)
	}

	var creditGrants decimal.Decimal
	if err := s.db.WithContext(ctx).Table("transactions AS t").
		Joins("LEFT JOIN orders AS o ON t.reference_id = o.order_no AND o.status = ?", "paid").
		Where("t.amount > 0 AND t.type <> ? AND t.created_at >= ? AND t.created_at <= ? AND o.id IS NULL", "refund", start, now).
		Select("COALESCE(SUM(t.amount), 0)").Scan(&creditGrants).Error; err != nil {
		return nil, fmt.Errorf("load financial credit grants: %w", err)
	}

	var outstandingBalance decimal.Decimal
	// Operator/admin balances are internal testing or staff balances, not
	// customer credits owed by the platform.
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Select("COALESCE(SUM(CASE WHEN balance > 0 THEN balance ELSE 0 END), 0)").
		Where("LOWER(COALESCE(role, '')) <> ?", "admin").
		Scan(&outstandingBalance).Error; err != nil {
		return nil, fmt.Errorf("load outstanding balance: %w", err)
	}

	var activeSubscriptions int64
	if err := s.db.WithContext(ctx).Model(&models.Subscription{}).
		Where("status IN ?", []string{"active", "trialing"}).
		Count(&activeSubscriptions).Error; err != nil {
		return nil, fmt.Errorf("load active subscriptions: %w", err)
	}

	grossProfit := usage.UsageRevenue.Sub(usage.ProviderCost).Round(models.MoneyScale)
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
		CashRevenue:         orders.CashRevenue.Round(models.MoneyScale),
		NetCashRevenue:      orders.CashRevenue.Sub(refundAmount).Round(models.MoneyScale),
		SubscriptionRevenue: orders.SubscriptionRevenue.Round(models.MoneyScale),
		TopUpRevenue:        orders.TopUpRevenue.Round(models.MoneyScale),
		UsageRevenue:        usage.UsageRevenue.Round(models.MoneyScale),
		ProviderCost:        usage.ProviderCost.Round(models.MoneyScale),
		GrossProfit:         grossProfit,
		GrossMargin:         percentMoney(grossProfit, usage.UsageRevenue),
		RefundAmount:        refundAmount.Round(models.MoneyScale),
		CreditGrants:        creditGrants.Round(models.MoneyScale),
		OutstandingBalance:  outstandingBalance.Round(models.MoneyScale),
		PaidOrders:          int(orders.PaidOrders),
		ActiveSubscriptions: int(activeSubscriptions),
		PayingCustomers:     int(orders.PayingCustomers),
		ARPU:                moneyRatio(orders.CashRevenue, orders.PayingCustomers),
		Daily:               daily,
		PaymentBreakdown:    paymentBreakdown,
		ProviderBreakdown:   providerBreakdown,
	}, nil
}

// maxFinancialDays is the absolute upper bound on the number of days a
// caller can request from financialDaily. The public FinancialDashboard
// already caps its `days` parameter at 365; this constant is a defensive
// re-cap for the private helper. Keeping it as a typed constant (rather
// than reassigning the parameter) gives CodeQL's taint tracker a stable
// upper bound it can use to clear go/uncontrolled-allocation-size.
const maxFinancialDays = 365

func (s *Service) financialDaily(ctx context.Context, start, end time.Time, days int) ([]FinancialDailyPoint, error) {
	// Defense-in-depth: FinancialDashboard already caps days at [1,365]
	// (line 77-83). CodeQL's go/uncontrolled-allocation-size taint
	// tracker won't follow an `int` parameter through any clamp shape
	// — neither `if x > N { x = N }` nor `min(max(x, 1), N)`. The
	// only pattern that clears the analysis is allocating with the
	// constant itself, then bounding the loop iteration separately.
	const capacity = maxFinancialDays
	byDate := make(map[string]*FinancialDailyPoint, capacity)
	out := make([]FinancialDailyPoint, 0, capacity)
	n := min(max(days, 1), maxFinancialDays)
	for i := 0; i < n; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		point := &FinancialDailyPoint{Date: date}
		byDate[date] = point
		out = append(out, *point)
	}

	var orderRows []struct {
		Date        string          `gorm:"column:date"`
		CashRevenue decimal.Decimal `gorm:"column:cash_revenue"`
		Orders      int64           `gorm:"column:orders"`
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
			point.CashRevenue = row.CashRevenue.Round(models.MoneyScale)
			point.Orders = int(row.Orders)
		}
	}

	var usageRows []struct {
		Date         string          `gorm:"column:date"`
		UsageRevenue decimal.Decimal `gorm:"column:usage_revenue"`
		ProviderCost decimal.Decimal `gorm:"column:provider_cost"`
		Requests     int64           `gorm:"column:requests"`
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
			grossProfit := row.UsageRevenue.Sub(row.ProviderCost).Round(models.MoneyScale)
			point.UsageRevenue = row.UsageRevenue.Round(models.MoneyScale)
			point.ProviderCost = row.ProviderCost.Round(models.MoneyScale)
			point.GrossProfit = grossProfit
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
		Name   string          `gorm:"column:name"`
		Amount decimal.Decimal `gorm:"column:amount"`
		Count  int64           `gorm:"column:count"`
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
		out[i] = FinancialBreakdown{Name: row.Name, Amount: row.Amount.Round(models.MoneyScale), Count: int(row.Count)}
	}
	return out, nil
}

func (s *Service) financialProviderBreakdown(ctx context.Context, start, end time.Time) ([]FinancialProviderBreakdown, error) {
	var rows []struct {
		ProviderName string          `gorm:"column:provider_name"`
		Requests     int64           `gorm:"column:requests"`
		UsageRevenue decimal.Decimal `gorm:"column:usage_revenue"`
		ProviderCost decimal.Decimal `gorm:"column:provider_cost"`
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
		grossProfit := row.UsageRevenue.Sub(row.ProviderCost).Round(models.MoneyScale)
		out[i] = FinancialProviderBreakdown{
			ProviderName: row.ProviderName,
			Requests:     int(row.Requests),
			UsageRevenue: row.UsageRevenue.Round(models.MoneyScale),
			ProviderCost: row.ProviderCost.Round(models.MoneyScale),
			GrossProfit:  grossProfit,
			GrossMargin:  percentMoney(grossProfit, row.UsageRevenue),
		}
	}
	return out, nil
}

func moneyRatio(numerator decimal.Decimal, denominator int64) decimal.Decimal {
	if denominator == 0 {
		return decimal.Zero
	}
	return numerator.Div(decimal.NewFromInt(denominator)).Round(models.MoneyScale)
}

func percentMoney(numerator, denominator decimal.Decimal) float64 {
	if denominator.IsZero() {
		return 0
	}
	out, _ := numerator.Div(denominator).Mul(decimal.NewFromInt(100)).Float64()
	return out
}
