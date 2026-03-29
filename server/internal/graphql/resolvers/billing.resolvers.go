package resolvers

// This file contains billing domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"fmt"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SetBudget is the resolver for the setBudget field.
func (r *mutationResolver) SetBudget(ctx context.Context, input model.BudgetInput) (*model.Budget, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, _ := uuid.Parse(uid)
	threshold := 80.0
	if input.AlertThreshold != nil {
		threshold = *input.AlertThreshold
	}
	b, err := r.BudgetService.SetBudget(ctx, id, input.MonthlyLimitUsd, threshold, derefStr(input.WebhookURL), derefStr(input.Email))
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

// ExportUsageCSV is the resolver for the exportUsageCsv field.
func (r *mutationResolver) ExportUsageCSV(ctx context.Context) (string, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	userID, _ := uuid.Parse(uid)

	var logs []models.UsageLog
	since := time.Now().AddDate(0, 0, -30)
	if err := r.AdminSvc.DB().Where("user_id = ? AND created_at >= ?", userID, since).
		Order("created_at DESC").Limit(10000).Find(&logs).Error; err != nil {
		return "", fmt.Errorf("failed to query usage: %w", err)
	}

	var buf strings.Builder
	buf.WriteString("date,model,channel,input_tokens,output_tokens,total_tokens,cost_usd,latency_ms,status\n")
	for _, l := range logs {
		status := "ok"
		if l.StatusCode >= 400 {
			status = "error"
		}
		fmt.Fprintf(&buf, "%s,%s,%s,%d,%d,%d,%.6f,%d,%s\n",
			l.CreatedAt.Format("2006-01-02 15:04:05"),
			l.ModelName,
			l.Channel,
			l.RequestTokens,
			l.ResponseTokens,
			l.TotalTokens,
			l.Cost,
			l.Latency,
			status,
		)
	}
	return buf.String(), nil
}

// CreateTask is the resolver for the createTask field.
func (r *mutationResolver) CreateTask(ctx context.Context, input model.CreateTaskInput) (*model.Task, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, _ := uuid.Parse(uid)
	t, err := r.TaskService.CreateTask(ctx, id, input.Type, input.Input, derefStr(input.WebhookURL))
	if err != nil {
		return nil, err
	}
	return asyncTaskToGQL(t), nil
}

// CancelTask is the resolver for the cancelTask field.
func (r *mutationResolver) CancelTask(ctx context.Context, id string) (*model.Task, error) {
	tid, _ := uuid.Parse(id)
	if err := r.TaskService.CancelTask(ctx, tid); err != nil {
		return nil, err
	}
	// Return a minimal task with cancelled status
	return &model.Task{ID: id, Status: "cancelled"}, nil
}

// ChangePlan is the resolver for the changePlan field.
func (r *mutationResolver) ChangePlan(ctx context.Context, planID string) (*model.UserSubscription, error) {
	orgID, err := r.resolveOrgID(ctx, nil)
	if err != nil {
		return nil, err
	}

	pid, err := uuid.Parse(planID)
	if err != nil {
		return nil, fmt.Errorf("invalid plan ID")
	}

	sub, err := r.SubscriptionSvc.ChangePlan(ctx, orgID, pid)
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

// CreateRechargeSession is the resolver for the createRechargeSession field.
func (r *mutationResolver) CreateRechargeSession(ctx context.Context, amount float64) (*model.CheckoutSession, error) {
	if amount < 1.0 {
		return nil, fmt.Errorf("minimum recharge amount is $1.00")
	}
	if amount > 10000.0 {
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

	url, err := r.Payment.CreateRechargeSession(ctx, userID, amount)
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
	userID, _ := uuid.Parse(uid)
	result, err := r.RedeemSvc.Redeem(userID, code)
	if err != nil {
		return nil, err
	}
	return &model.RedeemResult{
		Success:      true,
		Message:      result.Message,
		CreditAmount: &result.CreditAmount,
		PlanName:     &result.PlanName,
	}, nil
}

// CreatePlan is the resolver for the createPlan field.
func (r *mutationResolver) CreatePlan(ctx context.Context, input model.PlanInput) (*model.Plan, error) {
	plan := &models.Plan{
		Name: input.Name, PriceMonth: input.PriceMonth,
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
		ID: plan.ID.String(), Name: plan.Name, PriceMonth: plan.PriceMonth,
		TokenLimit: int(plan.TokenLimit), RateLimit: plan.RateLimit,
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
	plan.Name = input.Name
	plan.PriceMonth = input.PriceMonth
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
		ID: plan.ID.String(), Name: plan.Name, PriceMonth: plan.PriceMonth,
		TokenLimit: int(plan.TokenLimit), RateLimit: plan.RateLimit,
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
		TotalTokens: safeGQLInt(s.TotalTokens), TotalCost: s.TotalCost,
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
		out[i] = &model.DailyStats{Date: u.Date, Requests: int(u.Requests), TotalTokens: int(u.Tokens), TotalCost: u.Cost}
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
		out[i] = &model.ProviderUsage{ProviderName: u.ProviderName, Requests: int(u.Requests), Tokens: int(u.Tokens), Cost: u.Cost}
	}
	return out, nil
}

// MyRecentUsage is the resolver for the myRecentUsage field.
func (r *queryResolver) MyRecentUsage(ctx context.Context, page *int, pageSize *int, orgID *string, projectID *string) (*model.UsageConnection, error) {
	oId, err := r.resolveOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	pId := r.resolveProjectID(projectID)

	pg := valInt(page, 1)
	ps := valInt(pageSize, 20)
	logs, total, err := r.Billing.GetRecentUsage(ctx, oId, pId, pg, ps)
	if err != nil {
		return &model.UsageConnection{Data: []*model.UsageRecord{}, Total: 0}, nil
	}
	out := make([]*model.UsageRecord, len(logs))
	for i, l := range logs {
		out[i] = &model.UsageRecord{
			ID: l.ID.String(), ModelName: l.ModelName,
			InputTokens: l.RequestTokens, OutputTokens: l.ResponseTokens,
			Cost: l.Cost, LatencyMs: int(l.Latency),
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
	b := r.BudgetService.GetBudget(ctx, oId)
	var budget *model.Budget
	if b != nil {
		budget = budgetToGQL(b)
	}
	return &model.BudgetStatus{
		Budget: budget, CurrentSpend: s.CurrentSpend,
		PercentUsed: s.UsagePercent, IsOverBudget: s.IsOverBudget,
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
			Amount: o.Amount, Currency: o.Currency,
			Status: o.Status, PaymentMethod: o.PaymentMethod,
			CreatedAt: o.CreatedAt,
		}
	}
	return out, nil
}

// MyTasks is the resolver for the myTasks field.
func (r *queryResolver) MyTasks(ctx context.Context, page *int, pageSize *int) (*model.TaskConnection, error) {
	projectID := r.resolveProjectID(nil)
	if projectID == nil {
		return nil, fmt.Errorf("no active project")
	}
	p, ps := valInt(page, 1), valInt(pageSize, 20)
	tasks, total, err := r.TaskService.ListTasks(ctx, *projectID, "", ps, (p-1)*ps)
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
	userID, _ := uuid.Parse(uid)
	records, err := r.RedeemSvc.UserHistory(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.RedeemRecord, len(records))
	for i, rc := range records {
		out[i] = &model.RedeemRecord{
			ID: rc.ID.String(), Code: rc.Code,
			CreditAmount: rc.CreditAmount, RedeemedAt: rc.UpdatedAt,
		}
	}
	return out, nil
}

// Dashboard is the resolver for the dashboard field.
func (r *queryResolver) Dashboard(ctx context.Context, projectID *string, channel *string) (*model.Dashboard, error) {
	activeUsers, _ := r.UserSvc.CountActiveUsers(ctx, monthStart())
	_ = r.resolveProjectID(projectID) // reserved for future project-level filter
	now := time.Now()

	// Monthly summary
	sysSummary, _ := r.Billing.GetSystemUsageSummary(ctx, channel, monthStart(), now)
	totalReq, totalTokens, errorCount, mcpCalls, mcpErrors := 0, 0, 0, 0, 0
	totalCost, successRate := 0.0, 0.0
	if sysSummary != nil {
		totalReq = int(sysSummary.TotalRequests)
		totalTokens = int(sysSummary.TotalTokens)
		totalCost = sysSummary.TotalCost
		successRate = sysSummary.SuccessRate
		errorCount = int(sysSummary.ErrorCount)
		mcpCalls = int(sysSummary.MCPCallCount)
		mcpErrors = int(sysSummary.MCPErrorCount)
	}

	// Today's summary
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayReq, todayTokens := 0, 0
	todayCost := 0.0
	if todaySummary, err := r.Billing.GetSystemUsageSummary(ctx, channel, todayStart, now); err == nil && todaySummary != nil {
		todayReq = int(todaySummary.TotalRequests)
		todayTokens = int(todaySummary.TotalTokens)
		todayCost = todaySummary.TotalCost
	}

	// Infrastructure counts from service
	infra := r.AdminSvc.GetInfraCounts(ctx)

	return &model.Dashboard{
		TotalRequests: totalReq, SuccessRate: successRate,
		TotalTokens: totalTokens, TotalCost: totalCost,
		ActiveUsers:     int(activeUsers),
		ActiveProviders: int(infra.ProviderActive),
		ActiveProxies:   int(infra.ProxyActive),
		RequestsToday:   todayReq,
		CostToday:       todayCost,
		TokensToday:     todayTokens,
		ErrorCount:      errorCount,
		McpCallCount:    mcpCalls,
		McpErrorCount:   mcpErrors,
		APIKeys:         &model.APIKeysSummary{Total: int(infra.APIKeyTotal), Healthy: int(infra.APIKeyActive)},
		Proxies:         &model.ProxiesSummary{Total: int(infra.ProxyTotal), Healthy: int(infra.ProxyActive)},
	}, nil
}

// UsageChart is the resolver for the usageChart field.
func (r *queryResolver) UsageChart(ctx context.Context, days *int, projectID *string, channel *string) ([]*model.UsageChartPoint, error) {
	d := valInt(days, 30)
	usage, err := r.Billing.GetSystemDailyUsage(ctx, channel, d)
	if err != nil {
		return nil, err
	}
	out := make([]*model.UsageChartPoint, len(usage))
	for i, u := range usage {
		out[i] = &model.UsageChartPoint{Date: u.Date, Requests: int(u.Requests), Tokens: int(u.Tokens), Cost: u.Cost}
	}
	return out, nil
}

// ProviderStats is the resolver for the providerStats field.
func (r *queryResolver) ProviderStats(ctx context.Context, projectID *string, channel *string) ([]*model.ProviderStats, error) {
	usage, err := r.Billing.GetSystemUsageByProvider(ctx, channel, monthStart(), time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProviderStats, len(usage))
	for i, u := range usage {
		out[i] = &model.ProviderStats{
			ProviderName: u.ProviderName, Requests: int(u.Requests),
			Tokens: int(u.Tokens), TotalCost: u.Cost,
			SuccessRate: u.SuccessRate, AvgLatencyMs: u.AvgLatency,
		}
	}
	return out, nil
}

// ModelStats is the resolver for the modelStats field.
func (r *queryResolver) ModelStats(ctx context.Context, projectID *string, channel *string) ([]*model.ModelStats, error) {
	usage, err := r.Billing.GetSystemUsageByModel(ctx, channel, monthStart(), time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]*model.ModelStats, len(usage))
	for i, u := range usage {
		out[i] = &model.ModelStats{ModelName: u.ModelName, Requests: int(u.Requests), TotalCost: u.Cost}
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
			ID: p.ID.String(), Name: p.Name, PriceMonth: p.PriceMonth,
			TokenLimit: int(p.TokenLimit), RateLimit: p.RateLimit,
			Features: features, IsActive: p.IsActive,
		}
	}
	return out, nil
}
