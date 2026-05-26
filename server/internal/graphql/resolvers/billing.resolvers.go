package resolvers

// This file contains billing domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	billing "llm-router-platform/internal/service/billing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SetBudget is the resolver for the setBudget field.
func (r *mutationResolver) SetBudget(ctx context.Context, input model.BudgetInput) (*model.Budget, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}
	threshold := 0.8
	if input.AlertThreshold != nil {
		threshold = *input.AlertThreshold
	}
	enforce := false
	if input.EnforceHardLimit != nil {
		enforce = *input.EnforceHardLimit
	}
	limitUSD, err := input.MonthlyLimitUsd.Decimal()
	if err != nil {
		return nil, fmt.Errorf("invalid monthly limit: %w", err)
	}
	b, err := r.BudgetService.SetBudget(ctx, id, limitUSD, threshold, enforce, derefStr(input.WebhookURL), derefStr(input.Email))
	if err != nil {
		return nil, err
	}
	return budgetToGQL(b), nil
}

// DeleteBudget is the resolver for the deleteBudget field.
func (r *mutationResolver) DeleteBudget(ctx context.Context) (bool, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, _ := uuid.Parse(uid)
	return true, r.BudgetService.DeleteBudget(ctx, id)
}

// ExportUsageCSV streams the caller's last 30 days of usage as CSV.
//
// Delegates to billing.Service.ExportUsageCSV which paginates the query in
// batches of 1000 to avoid loading the entire dataset into memory. The 30-day
// window matches the previous behavior; for arbitrary ranges callers should
// use the date-bounded admin-side export.
func (r *mutationResolver) ExportUsageCSV(ctx context.Context) (string, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	userID, err := uuid.Parse(uid)
	if err != nil {
		return "", fmt.Errorf("invalid user id: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	end := time.Now()
	start := end.AddDate(0, 0, -30)
	if err := r.Billing.ExportUsageCSV(ctx, userID, start, end, w); err != nil {
		return "", fmt.Errorf("failed to export usage: %w", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("csv flush failed: %w", err)
	}
	return buf.String(), nil
}

// CreateTask is the resolver for the createTask field.
func (r *mutationResolver) CreateTask(ctx context.Context, input model.CreateTaskInput) (*model.Task, error) {
	projectID, err := r.resolveAccessibleProjectID(ctx, nil, "OWNER", "ADMIN", "MEMBER")
	if err != nil {
		return nil, err
	}
	t, err := r.TaskService.CreateTask(ctx, projectID, input.Type, input.Input, derefStr(input.WebhookURL))
	if err != nil {
		return nil, err
	}
	return asyncTaskToGQL(t), nil
}

// CancelTask is the resolver for the cancelTask field.
func (r *mutationResolver) CancelTask(ctx context.Context, id string) (*model.Task, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	tid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID")
	}
	task, err := r.TaskService.GetTask(ctx, tid)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}
	if err := r.UserSvc.RequireProjectRole(ctx, uid, task.ProjectID.String(), "OWNER", "ADMIN", "MEMBER"); err != nil {
		return nil, fmt.Errorf("forbidden: access denied")
	}
	if err := r.TaskService.CancelTask(ctx, tid); err != nil {
		return nil, err
	}
	task.Status = "cancelled"
	now := time.Now()
	task.CompletedAt = &now
	return asyncTaskToGQL(task), nil
}

// ChangePlan is the resolver for the changePlan field.
func (r *mutationResolver) ChangePlan(ctx context.Context, planID string) (*model.UserSubscription, error) {
	orgID, err := r.resolveOrgID(ctx, nil)
	if err != nil {
		return nil, err
	}
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	pid, err := uuid.Parse(planID)
	if err != nil {
		return nil, fmt.Errorf("invalid plan ID")
	}

	sub, err := r.SubscriptionSvc.ChangePlan(ctx, orgID, userID, pid)
	if err != nil {
		return nil, err
	}

	return &model.UserSubscription{
		ID: sub.ID.String(), OrgID: sub.OrgID.String(), PlanID: sub.PlanID.String(),
		PlanName: sub.Plan.Name,
		Status:   sub.Status, CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd: sub.CurrentPeriodEnd,
		TokenLimit:       int(sub.Plan.TokenLimit),
	}, nil
}

// CreateCheckoutSession is the resolver for the createCheckoutSession field.
func (r *mutationResolver) CreateCheckoutSession(ctx context.Context, planID string) (*model.CheckoutSession, error) {
	orgID, err := r.resolveOrgID(ctx, nil)
	if err != nil {
		return nil, err
	}
	pid, err := uuid.Parse(planID)
	if err != nil {
		return nil, fmt.Errorf("invalid plan ID")
	}

	url, err := r.Payment.CreateCheckoutSession(ctx, orgID, pid)
	if err != nil {
		return nil, err
	}
	return &model.CheckoutSession{URL: url}, nil
}

// CreateRechargeSession is the resolver for the createRechargeSession field.
func (r *mutationResolver) CreateRechargeSession(ctx context.Context, amount model.Money) (*model.CheckoutSession, error) {
	parsedAmount, err := amount.Decimal()
	if err != nil {
		return nil, fmt.Errorf("invalid recharge amount: %w", err)
	}
	amountMoney := models.MoneyRoundToCents(parsedAmount)
	if amountMoney.Cmp(decimal.NewFromInt(1)) < 0 {
		return nil, fmt.Errorf("minimum recharge amount is $1.00")
	}
	if amountMoney.Cmp(decimal.NewFromInt(10000)) > 0 {
		return nil, fmt.Errorf("maximum recharge amount is $10,000.00")
	}

	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	url, err := r.Payment.CreateRechargeSession(ctx, userID, amountMoney)
	if err != nil {
		return nil, err
	}

	return &model.CheckoutSession{URL: url}, nil
}

// RedeemCode is the resolver for the redeemCode field.
func (r *mutationResolver) RedeemCode(ctx context.Context, code string) (*model.RedeemResult, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}
	result, err := r.RedeemSvc.Redeem(userID, code)
	if err != nil {
		return nil, err
	}
	out := &model.RedeemResult{
		Success: result.Success,
		Message: result.Message,
	}
	if result.CreditAmount.IsPositive() {
		creditAmount := model.NewMoney(result.CreditAmount)
		out.CreditAmount = &creditAmount
	}
	if result.PlanName != "" {
		out.PlanName = &result.PlanName
	}
	return out, nil
}

// CreatePlan is the resolver for the createPlan field.
func (r *mutationResolver) CreatePlan(ctx context.Context, input model.PlanInput) (*model.Plan, error) {
	priceMonth, err := input.PriceMonth.Decimal()
	if err != nil {
		return nil, fmt.Errorf("invalid monthly price: %w", err)
	}
	plan := &models.Plan{
		Name: input.Name, PriceMonth: priceMonth.Round(models.MoneyScale),
		TokenLimit: int64(input.TokenLimit), RateLimit: input.RateLimit, IsActive: true,
	}
	if input.Description != nil {
		plan.Description = *input.Description
	}
	if input.SupportLevel != nil {
		plan.SupportLevel = *input.SupportLevel
	}
	if input.IsActive != nil {
		plan.IsActive = *input.IsActive
	}
	if input.Features != nil {
		plan.Features = *input.Features
	}
	if err := r.AdminSvc.DB().WithContext(ctx).Create(plan).Error; err != nil {
		return nil, err
	}
	var features *string
	if plan.Features != "" {
		features = &plan.Features
	}
	return &model.Plan{
		ID: plan.ID.String(), Name: plan.Name, Description: plan.Description,
		PriceMonth: model.NewMoney(plan.PriceMonth), TokenLimit: int(plan.TokenLimit),
		RateLimit: plan.RateLimit, SupportLevel: plan.SupportLevel,
		Features: features, IsActive: plan.IsActive,
	}, nil
}

// UpdatePlan is the resolver for the updatePlan field.
func (r *mutationResolver) UpdatePlan(ctx context.Context, id string, input model.PlanInput) (*model.Plan, error) {
	pid, _ := uuid.Parse(id)
	var plan models.Plan
	if err := r.AdminSvc.DB().WithContext(ctx).First(&plan, "id = ?", pid).Error; err != nil {
		return nil, fmt.Errorf("plan not found")
	}
	priceMonth, err := input.PriceMonth.Decimal()
	if err != nil {
		return nil, fmt.Errorf("invalid monthly price: %w", err)
	}
	plan.Name = input.Name
	plan.PriceMonth = priceMonth.Round(models.MoneyScale)
	plan.TokenLimit = int64(input.TokenLimit)
	plan.RateLimit = input.RateLimit
	if input.Description != nil {
		plan.Description = *input.Description
	}
	if input.SupportLevel != nil {
		plan.SupportLevel = *input.SupportLevel
	}
	if input.IsActive != nil {
		plan.IsActive = *input.IsActive
	}
	if input.Features != nil {
		plan.Features = *input.Features
	}
	if err := r.AdminSvc.DB().WithContext(ctx).Save(&plan).Error; err != nil {
		return nil, err
	}
	var features *string
	if plan.Features != "" {
		features = &plan.Features
	}
	return &model.Plan{
		ID: plan.ID.String(), Name: plan.Name, Description: plan.Description,
		PriceMonth: model.NewMoney(plan.PriceMonth), TokenLimit: int(plan.TokenLimit),
		RateLimit: plan.RateLimit, SupportLevel: plan.SupportLevel,
		Features: features, IsActive: plan.IsActive,
	}, nil
}

// MyUsageSummary is the resolver for the myUsageSummary field.
func (r *queryResolver) MyUsageSummary(ctx context.Context, orgID *string, projectID *string, channel *string) (*model.UsageSummary, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	pId := r.resolveProjectID(projectID)

	s, err := r.Billing.GetUsageSummary(ctx, oId, pId, channel, monthStart(), time.Now())
	if err != nil {
		return nil, err
	}
	return &model.UsageSummary{
		TotalRequests: safeGQLInt(s.TotalRequests), SuccessRate: s.SuccessRate,
		TotalTokens: safeGQLInt(s.TotalTokens), TotalCost: model.NewMoney(s.TotalCost),
	}, nil
}

// MyDailyUsage is the resolver for the myDailyUsage field.
func (r *queryResolver) MyDailyUsage(ctx context.Context, days *int, orgID *string, projectID *string, channel *string) ([]*model.DailyStats, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	pId := r.resolveProjectID(projectID)

	d := valInt(days, 30)
	usage, err := r.Billing.GetDailyUsage(ctx, oId, pId, channel, d)
	if err != nil {
		return nil, err
	}
	out := make([]*model.DailyStats, len(usage))
	for i, u := range usage {
		out[i] = &model.DailyStats{Date: u.Date, Requests: int(u.Requests), TotalTokens: int(u.Tokens), TotalCost: model.NewMoney(u.Cost)}
	}
	return out, nil
}

// MyUsageByProvider is the resolver for the myUsageByProvider field.
func (r *queryResolver) MyUsageByProvider(ctx context.Context, orgID *string, projectID *string, channel *string) ([]*model.ProviderUsage, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	pId := r.resolveProjectID(projectID)

	usage, err := r.Billing.GetUsageByProvider(ctx, oId, pId, channel, monthStart(), time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProviderUsage, len(usage))
	for i, u := range usage {
		out[i] = &model.ProviderUsage{ProviderName: u.ProviderName, Requests: int(u.Requests), Tokens: int(u.Tokens), Cost: model.NewMoney(u.Cost)}
	}
	return out, nil
}

// MyRecentUsage is the resolver for the myRecentUsage field.
func (r *queryResolver) MyRecentUsage(ctx context.Context, page *int, pageSize *int, orgID *string, projectID *string, channel *string) (*model.UsageConnection, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	pId := r.resolveProjectID(projectID)

	pg, ps := clampPagination(page, pageSize)
	logs, total, err := r.Billing.GetRecentUsage(ctx, oId, pId, channel, pg, ps)
	if err != nil {
		return &model.UsageConnection{Data: []*model.UsageRecord{}, Total: 0}, nil
	}
	out := make([]*model.UsageRecord, len(logs))
	for i, l := range logs {
		out[i] = &model.UsageRecord{
			ID: l.ID.String(), ModelName: l.ModelName,
			InputTokens: l.RequestTokens, OutputTokens: l.ResponseTokens,
			Cost: model.NewMoney(l.Cost), LatencyMs: int(l.Latency),
			IsSuccess: l.StatusCode >= 200 && l.StatusCode < 400,
			CreatedAt: l.CreatedAt,
		}
	}
	return &model.UsageConnection{Data: out, Total: int(total)}, nil
}

// MyBudget is the resolver for the myBudget field.
func (r *queryResolver) MyBudget(ctx context.Context, orgID *string) (*model.Budget, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	b := r.BudgetService.GetBudget(ctx, oId)
	if b == nil {
		return nil, nil
	}
	return budgetToGQL(b), nil
}

// MyBudgetStatus is the resolver for the myBudgetStatus field.
func (r *queryResolver) MyBudgetStatus(ctx context.Context, orgID *string) (*model.BudgetStatus, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	s, err := r.BudgetService.CheckBudget(ctx, oId)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	b := r.BudgetService.GetBudget(ctx, oId)
	var budget *model.Budget
	if b != nil {
		budget = budgetToGQL(b)
	}
	if s == nil {
		return &model.BudgetStatus{Budget: budget}, nil
	}
	return &model.BudgetStatus{
		Budget:          budget,
		CurrentSpend:    model.NewMoney(s.CurrentSpend),
		RemainingBudget: model.NewMoney(s.RemainingUSD),
		PercentUsed:     s.UsagePercent,
		IsOverBudget:    s.IsOverBudget,
	}, nil
}

// MySubscription is the resolver for the mySubscription field.
func (r *queryResolver) MySubscription(ctx context.Context, orgID *string) (*model.UserSubscription, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	sub, err := r.SubscriptionSvc.GetUserSubscription(ctx, oId)
	if err != nil || sub == nil {
		return nil, nil
	}

	result := &model.UserSubscription{
		ID: sub.ID.String(), OrgID: sub.OrgID.String(), PlanID: sub.PlanID.String(),
		PlanName: sub.Plan.Name,
		Status:   sub.Status, CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd: sub.CurrentPeriodEnd,
		TokenLimit:       int(sub.Plan.TokenLimit),
	}

	// Compute current period token usage
	if sub.Plan.TokenLimit > 0 {
		usedTokens, err := r.SubscriptionSvc.GetQuotaUsage(ctx, oId)
		if err == nil {
			result.UsedTokens = int(usedTokens)
			result.QuotaPercentage = float64(usedTokens) / float64(sub.Plan.TokenLimit) * 100
			if result.QuotaPercentage > 100 {
				result.QuotaPercentage = 100
			}
			result.IsQuotaExceeded = usedTokens >= sub.Plan.TokenLimit
		}
	}

	return result, nil
}

// MyOrders is the resolver for the myOrders field.
func (r *queryResolver) MyOrders(ctx context.Context, orgID *string) ([]*model.Order, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	orders, err := r.Payment.GetUserOrders(ctx, oId)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Order, len(orders))
	for i, o := range orders {
		out[i] = &model.Order{
			ID: o.ID.String(), OrderNo: o.OrderNo,
			Amount: model.NewMoney(o.Amount), Currency: o.Currency,
			Status: o.Status, PaymentMethod: o.PaymentMethod,
			CreatedAt: o.CreatedAt,
		}
	}
	return out, nil
}

// MyTasks is the resolver for the myTasks field.
func (r *queryResolver) MyTasks(ctx context.Context, page *int, pageSize *int) (*model.TaskConnection, error) {
	projectID, err := r.resolveAccessibleProjectID(ctx, nil, "OWNER", "ADMIN", "MEMBER", "READONLY")
	if err != nil {
		return nil, err
	}
	p, ps := clampPagination(page, pageSize)
	tasks, total, err := r.TaskService.ListTasks(ctx, projectID, "", ps, (p-1)*ps)
	if err != nil {
		return &model.TaskConnection{Data: []*model.Task{}, Total: 0}, nil
	}
	out := make([]*model.Task, len(tasks))
	for i := range tasks {
		out[i] = asyncTaskToGQL(&tasks[i])
	}
	return &model.TaskConnection{Data: out, Total: int(total)}, nil
}

// MyAnomalyDetection is the resolver for the myAnomalyDetection field.
func (r *queryResolver) MyAnomalyDetection(ctx context.Context) (*model.AnomalyResult, error) {
	orgID, projectID, err := r.resolveOrgProjectIDs(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	result, err := r.Billing.DetectCostAnomaly(ctx, orgID, projectID, 30, 2.0)
	if err != nil {
		return &model.AnomalyResult{HasAnomaly: false}, nil
	}
	return &model.AnomalyResult{HasAnomaly: result.IsAnomaly, Message: &result.Message}, nil
}

// MyRedeemHistory is the resolver for the myRedeemHistory field.
func (r *queryResolver) MyRedeemHistory(ctx context.Context) ([]*model.RedeemRecord, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}
	records, err := r.RedeemSvc.UserHistory(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.RedeemRecord, len(records))
	for i, rc := range records {
		redeemedAt := rc.UpdatedAt
		if rc.UsedAt != nil {
			redeemedAt = *rc.UsedAt
		}
		var planName *string
		if rc.Plan.Name != "" {
			name := rc.Plan.Name
			planName = &name
		}
		out[i] = &model.RedeemRecord{
			ID: rc.ID.String(), Code: rc.Code,
			CreditAmount: model.NewMoney(rc.CreditAmount), PlanName: planName, RedeemedAt: redeemedAt,
		}
	}
	return out, nil
}

// Dashboard is the resolver for the dashboard field.
func (r *queryResolver) Dashboard(ctx context.Context, projectID *string, channel *string) (*model.Dashboard, error) {
	orgID, scopedProjectID, systemScope, err := r.resolveUsageScope(ctx, projectID, "OWNER", "ADMIN", "MEMBER", "READONLY")
	if err != nil {
		return nil, err
	}
	now := time.Now()

	var summary *billing.UsageSummary
	var todaySummary *billing.UsageSummary
	if systemScope {
		summary, _ = r.Billing.GetSystemUsageSummary(ctx, channel, monthStart(), now)
	} else {
		summary, _ = r.Billing.GetUsageSummary(ctx, orgID, scopedProjectID, channel, monthStart(), now)
	}

	totalReq, totalTokens, errorCount, mcpCalls, mcpErrors := 0, 0, 0, 0, 0
	totalCost := decimal.Zero
	successRate, avgLatency := 0.0, 0.0
	if summary != nil {
		totalReq = int(summary.TotalRequests)
		totalTokens = int(summary.TotalTokens)
		totalCost = summary.TotalCost
		successRate = summary.SuccessRate
		avgLatency = summary.AvgLatency
		errorCount = int(summary.ErrorCount)
		mcpCalls = int(summary.MCPCallCount)
		mcpErrors = int(summary.MCPErrorCount)
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayReq, todayTokens := 0, 0
	todayCost := decimal.Zero
	if systemScope {
		todaySummary, _ = r.Billing.GetSystemUsageSummary(ctx, channel, todayStart, now)
	} else {
		todaySummary, _ = r.Billing.GetUsageSummary(ctx, orgID, scopedProjectID, channel, todayStart, now)
	}
	if todaySummary != nil {
		todayReq = int(todaySummary.TotalRequests)
		todayTokens = int(todaySummary.TotalTokens)
		todayCost = todaySummary.TotalCost
	}

	activeUsers, activeProviders, activeProxies := 0, 0, 0
	apiKeys := &model.APIKeysSummary{}
	proxies := &model.ProxiesSummary{}
	if systemScope {
		active, _ := r.UserSvc.CountActiveUsers(ctx, monthStart())
		infra := r.AdminSvc.GetInfraCounts(ctx)
		activeUsers = int(active)
		activeProviders = int(infra.ProviderActive)
		activeProxies = int(infra.ProxyActive)
		apiKeys = &model.APIKeysSummary{Total: int(infra.APIKeyTotal), Healthy: int(infra.APIKeyActive)}
		proxies = &model.ProxiesSummary{Total: int(infra.ProxyTotal), Healthy: int(infra.ProxyActive)}
	} else if scopedProjectID != nil {
		var totalKeys, activeKeys int64
		_ = r.AdminSvc.DB().WithContext(ctx).Model(&models.APIKey{}).
			Where("project_id = ?", *scopedProjectID).
			Count(&totalKeys).Error
		_ = r.AdminSvc.DB().WithContext(ctx).Model(&models.APIKey{}).
			Where("project_id = ? AND is_active = ?", *scopedProjectID, true).
			Count(&activeKeys).Error
		apiKeys = &model.APIKeysSummary{Total: int(totalKeys), Healthy: int(activeKeys)}
	}

	return &model.Dashboard{
		TotalRequests:    totalReq,
		SuccessRate:      successRate,
		TotalTokens:      totalTokens,
		TotalCost:        model.NewMoney(totalCost),
		AverageLatencyMs: avgLatency,
		ActiveUsers:      int(activeUsers),
		ActiveProviders:  activeProviders,
		ActiveProxies:    activeProxies,
		RequestsToday:    todayReq,
		CostToday:        model.NewMoney(todayCost),
		TokensToday:      todayTokens,
		ErrorCount:       errorCount,
		McpCallCount:     mcpCalls,
		McpErrorCount:    mcpErrors,
		APIKeys:          apiKeys,
		Proxies:          proxies,
	}, nil
}

// UsageChart is the resolver for the usageChart field.
func (r *queryResolver) UsageChart(ctx context.Context, days *int, projectID *string, channel *string) ([]*model.UsageChartPoint, error) {
	orgID, scopedProjectID, systemScope, err := r.resolveUsageScope(ctx, projectID, "OWNER", "ADMIN", "MEMBER", "READONLY")
	if err != nil {
		return nil, err
	}
	d := valInt(days, 30)
	var usage []billing.DailyUsage
	if systemScope {
		usage, err = r.Billing.GetSystemDailyUsage(ctx, channel, d)
	} else {
		usage, err = r.Billing.GetDailyUsage(ctx, orgID, scopedProjectID, channel, d)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*model.UsageChartPoint, len(usage))
	for i, u := range usage {
		out[i] = &model.UsageChartPoint{Date: u.Date, Requests: int(u.Requests), Tokens: int(u.Tokens), Cost: model.NewMoney(u.Cost)}
	}
	return out, nil
}

// ProviderStats is the resolver for the providerStats field.
func (r *queryResolver) ProviderStats(ctx context.Context, projectID *string, channel *string) ([]*model.ProviderStats, error) {
	orgID, scopedProjectID, systemScope, err := r.resolveUsageScope(ctx, projectID, "OWNER", "ADMIN", "MEMBER", "READONLY")
	if err != nil {
		return nil, err
	}
	var usage []billing.ProviderUsage
	if systemScope {
		usage, err = r.Billing.GetSystemUsageByProvider(ctx, channel, monthStart(), time.Now())
	} else {
		usage, err = r.Billing.GetUsageByProvider(ctx, orgID, scopedProjectID, channel, monthStart(), time.Now())
	}
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProviderStats, len(usage))
	for i, u := range usage {
		out[i] = &model.ProviderStats{
			ProviderID: u.ProviderID.String(), ProviderName: u.ProviderName, Requests: int(u.Requests),
			Tokens: int(u.Tokens), TotalCost: model.NewMoney(u.Cost),
			SuccessRate: u.SuccessRate, AvgLatencyMs: u.AvgLatency,
		}
	}
	return out, nil
}

// ModelStats is the resolver for the modelStats field.
func (r *queryResolver) ModelStats(ctx context.Context, projectID *string, channel *string) ([]*model.ModelStats, error) {
	orgID, scopedProjectID, systemScope, err := r.resolveUsageScope(ctx, projectID, "OWNER", "ADMIN", "MEMBER", "READONLY")
	if err != nil {
		return nil, err
	}
	var usage []billing.ModelUsage
	if systemScope {
		usage, err = r.Billing.GetSystemUsageByModel(ctx, channel, monthStart(), time.Now())
	} else {
		usage, err = r.Billing.GetUsageByModel(ctx, orgID, scopedProjectID, channel, monthStart(), time.Now())
	}
	if err != nil {
		return nil, err
	}
	out := make([]*model.ModelStats, len(usage))
	for i, u := range usage {
		out[i] = &model.ModelStats{
			ModelID:      u.ModelID.String(),
			ModelName:    u.ModelName,
			Requests:     int(u.Requests),
			InputTokens:  int(u.InputTokens),
			OutputTokens: int(u.OutputTokens),
			TotalCost:    model.NewMoney(u.Cost),
		}
	}
	return out, nil
}

// Plans is the resolver for the plans field.
func (r *queryResolver) Plans(ctx context.Context) ([]*model.Plan, error) {
	plans, err := r.SubscriptionSvc.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Plan, len(plans))
	for i, p := range plans {
		var features *string
		if p.Features != "" {
			features = &p.Features
		}
		out[i] = &model.Plan{
			ID: p.ID.String(), Name: p.Name, PriceMonth: model.NewMoney(p.PriceMonth),
			TokenLimit: int(p.TokenLimit), RateLimit: p.RateLimit,
			Description: p.Description, SupportLevel: p.SupportLevel,
			Features: features, IsActive: p.IsActive,
		}
	}
	return out, nil
}
