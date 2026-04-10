package resolvers

// This file contains admin domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/pkg/sanitize"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ToggleUser is the resolver for the toggleUser field.
func (r *mutationResolver) ToggleUser(ctx context.Context, id string) (*model.User, error) {
	uid, _ := uuid.Parse(id)
	u, err := r.UserSvc.ToggleUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	actorID, _ := directives.UserIDFromContext(ctx)
	aid, _ := uuid.Parse(actorID)
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionUserToggle, aid, uid, ip, ua, map[string]interface{}{"is_active": u.IsActive})
	return userToGQL(u), nil
}

// UpdateUserRole is the resolver for the updateUserRole field.
func (r *mutationResolver) UpdateUserRole(ctx context.Context, id string, role string) (*model.User, error) {
	uid, _ := uuid.Parse(id)
	u, err := r.UserSvc.UpdateRole(ctx, uid, role)
	if err != nil {
		return nil, err
	}
	actorID, _ := directives.UserIDFromContext(ctx)
	aid, _ := uuid.Parse(actorID)
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionRoleChange, aid, uid, ip, ua, map[string]interface{}{"new_role": role})
	return userToGQL(u), nil
}

// UpdateUserQuota is the resolver for the updateUserQuota field.
func (r *mutationResolver) UpdateUserQuota(ctx context.Context, id string, input model.QuotaInput) (*model.User, error) {
	uid, _ := uuid.Parse(id)
	var tokenLimit *int64
	if input.MonthlyTokenLimit != nil {
		v := int64(*input.MonthlyTokenLimit)
		tokenLimit = &v
	}
	u, err := r.UserSvc.UpdateQuota(ctx, uid, tokenLimit, input.MonthlyBudgetUsd)
	if err != nil {
		return nil, err
	}
	return userToGQL(u), nil
}

// UpdateSystemSettings is the resolver for the updateSystemSettings field.
func (r *mutationResolver) UpdateSystemSettings(ctx context.Context, input model.SystemSettingsInput) (*model.SystemSettings, error) {
	if err := r.SystemConfig.UpdateSettings(ctx, input.Category, input.Data); err != nil {
		return nil, err
	}
	// Return full settings after update
	all, err := r.SystemConfig.GetAllSettingsDecrypted(ctx)
	if err != nil {
		return nil, err
	}
	return buildSystemSettings(r.Config().Registration.Mode, all), nil
}

// SendTestEmail is the resolver for the sendTestEmail field.
func (r *mutationResolver) SendTestEmail(ctx context.Context, to string) (bool, error) {
	return r.AdminSvc.SendTestEmail(ctx, to)
}

// TriggerBackup is the resolver for the triggerBackup field.
func (r *mutationResolver) TriggerBackup(ctx context.Context) (bool, error) {
	return r.AdminSvc.TriggerBackup(ctx)
}

// CreateInviteCode is the resolver for the createInviteCode field.
func (r *mutationResolver) CreateInviteCode(ctx context.Context, input model.InviteCodeInput) (*model.InviteCode, error) {
	maxUses := 1
	if input.MaxUses != nil {
		maxUses = *input.MaxUses
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %w", err)
	}
	code := "inv_" + hex.EncodeToString(buf)
	adminID, _ := directives.UserIDFromContext(ctx)
	aid, _ := uuid.Parse(adminID)
	ic := models.InviteCode{Code: code, CreatedBy: aid, MaxUses: maxUses, ExpiresAt: input.ExpiresAt, IsActive: true}
	if err := r.AdminSvc.DB().Create(&ic).Error; err != nil {
		return nil, err
	}
	return &model.InviteCode{
		ID: ic.ID.String(), Code: ic.Code, CreatedBy: ic.CreatedBy.String(),
		MaxUses: ic.MaxUses, UseCount: ic.UseCount, ExpiresAt: ic.ExpiresAt,
		IsActive: ic.IsActive, CreatedAt: ic.CreatedAt,
	}, nil
}

// ExportSystemUsageCSV is the resolver for the exportSystemUsageCsv field.
func (r *mutationResolver) ExportSystemUsageCSV(ctx context.Context) (string, error) {
	type logWithUser struct {
		models.UsageLog
		UserEmail    string `gorm:"column:user_email"`
		ProviderName string `gorm:"column:provider_name"`
	}

	since := time.Now().AddDate(0, 0, -30)
	var logs []logWithUser
	if err := r.AdminSvc.DB().Table("usage_logs").
		Select("usage_logs.*, users.email as user_email, providers.name as provider_name").
		Joins("LEFT JOIN users ON users.id = usage_logs.user_id").
		Joins("LEFT JOIN providers ON providers.id = usage_logs.provider_id").
		Where("usage_logs.created_at >= ?", since).
		Order("usage_logs.created_at DESC").Limit(50000).
		Find(&logs).Error; err != nil {
		return "", fmt.Errorf("failed to query system usage: %w", err)
	}

	var buf strings.Builder
	buf.WriteString("date,user_email,provider,model,channel,input_tokens,output_tokens,total_tokens,cost_usd,latency_ms,status\n")
	for _, l := range logs {
		status := "ok"
		if l.StatusCode >= 400 {
			status = "error"
		}
		fmt.Fprintf(&buf, "%s,%s,%s,%s,%s,%d,%d,%d,%.6f,%d,%s\n",
			l.CreatedAt.Format("2006-01-02 15:04:05"),
			l.UserEmail,
			l.ProviderName,
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

// GenerateRedeemCodes is the resolver for the generateRedeemCodes field.
func (r *mutationResolver) GenerateRedeemCodes(ctx context.Context, input model.GenerateRedeemCodesInput) (*model.GenerateRedeemCodesResult, error) {
	var planID *uuid.UUID
	if input.PlanID != nil {
		p, _ := uuid.Parse(*input.PlanID)
		planID = &p
	}
	var expiresAt *time.Time
	if input.ExpiresAt != nil {
		t := *input.ExpiresAt
		expiresAt = &t
	}
	creditAmount := float64(0)
	if input.CreditAmount != nil {
		creditAmount = *input.CreditAmount
	}
	planDays := 30
	if input.PlanDays != nil {
		planDays = *input.PlanDays
	}
	note := ""
	if input.Note != nil {
		note = *input.Note
	}
	codes, err := r.RedeemSvc.GenerateCodes(input.Type, creditAmount, planID, planDays, input.Count, expiresAt, note)
	if err != nil {
		return nil, err
	}
	return &model.GenerateRedeemCodesResult{Codes: codes, Count: len(codes)}, nil
}

// RevokeRedeemCode is the resolver for the revokeRedeemCode field.
func (r *mutationResolver) RevokeRedeemCode(ctx context.Context, id string) (bool, error) {
	uid, _ := uuid.Parse(id)
	return true, r.RedeemSvc.RevokeCode(uid)
}

// AdminDashboard is the resolver for the adminDashboard field.
func (r *queryResolver) AdminDashboard(ctx context.Context) (*model.AdminDashboard, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// User counts
	totalUsers := r.AdminSvc.TotalUserCount(ctx)

	activeUsersToday, _ := r.UserSvc.CountActiveUsers(ctx, todayStart)
	activeUsersMonth, _ := r.UserSvc.CountActiveUsers(ctx, monthStart)

	// Revenue
	totalRevenue, revenueMonth := r.AdminSvc.RevenueStats(ctx, monthStart)

	// System usage summary
	sysSummary, _ := r.Billing.GetSystemUsageSummary(ctx, nil, monthStart, now)
	totalReq, totalTokens, errorCount, mcpCalls, mcpErrors := 0, 0, 0, 0, 0
	totalCost, successRate, avgLatency := 0.0, 0.0, 0.0
	if sysSummary != nil {
		totalReq = int(sysSummary.TotalRequests)
		totalTokens = int(sysSummary.TotalTokens)
		totalCost = sysSummary.TotalCost
		successRate = sysSummary.SuccessRate
		errorCount = int(sysSummary.ErrorCount)
		mcpCalls = int(sysSummary.MCPCallCount)
		mcpErrors = int(sysSummary.MCPErrorCount)
		avgLatency = sysSummary.AvgLatency
	}

	// Today
	todayReq, todayTokens := 0, 0
	todayCost := 0.0
	if ts, err := r.Billing.GetSystemUsageSummary(ctx, nil, todayStart, now); err == nil && ts != nil {
		todayReq = int(ts.TotalRequests)
		todayTokens = int(ts.TotalTokens)
		todayCost = ts.TotalCost
	}

	// Infrastructure
	infra := r.AdminSvc.GetInfraCounts(ctx)

	return &model.AdminDashboard{
		TotalUsers:       int(totalUsers),
		ActiveUsersToday: int(activeUsersToday),
		ActiveUsersMonth: int(activeUsersMonth),
		TotalRevenue:     totalRevenue,
		RevenueThisMonth: revenueMonth,
		TotalRequests:    totalReq,
		RequestsToday:    todayReq,
		TotalTokens:      totalTokens,
		TokensToday:      todayTokens,
		TotalCost:        totalCost,
		CostToday:        todayCost,
		SuccessRate:      successRate,
		ErrorCount:       errorCount,
		AvgLatencyMs:     avgLatency,
		ActiveProviders:  int(infra.ProviderActive),
		TotalProviders:   int(infra.ProviderTotal),
		ActiveProxies:    int(infra.ProxyActive),
		TotalProxies:     int(infra.ProxyTotal),
		APIKeysTotal:     int(infra.APIKeyTotal),
		APIKeysHealthy:   int(infra.APIKeyActive),
		McpCallCount:     mcpCalls,
		McpErrorCount:    mcpErrors,
	}, nil
}

// AdminUsageByUser is the resolver for the adminUsageByUser field.
func (r *queryResolver) AdminUsageByUser(ctx context.Context, days *int) ([]*model.AdminUsageByUser, error) {
	d := valInt(days, 30)
	since := time.Now().AddDate(0, 0, -d)

	type row struct {
		UserID   string
		UserName string
		Email    string
		Requests int
		Tokens   int
		Cost     float64
	}
	var rows []row
	err := r.AdminSvc.DB().WithContext(ctx).
		Table("usage_logs").
		Select("usage_logs.user_id, users.name as user_name, users.email, COUNT(*) as requests, COALESCE(SUM(usage_logs.total_tokens), 0) as tokens, COALESCE(SUM(usage_logs.cost), 0) as cost").
		Joins("LEFT JOIN users ON users.id = usage_logs.user_id").
		Where("usage_logs.created_at >= ?", since).
		Group("usage_logs.user_id, users.name, users.email").
		Order("requests DESC").
		Limit(50).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]*model.AdminUsageByUser, len(rows))
	for i, row := range rows {
		out[i] = &model.AdminUsageByUser{
			UserID:   row.UserID,
			UserName: row.UserName,
			Email:    row.Email,
			Requests: row.Requests,
			Tokens:   row.Tokens,
			Cost:     row.Cost,
		}
	}
	return out, nil
}

// AdminRevenueChart is the resolver for the adminRevenueChart field.
func (r *queryResolver) AdminRevenueChart(ctx context.Context, days *int) ([]*model.RevenueChartPoint, error) {
	d := valInt(days, 30)
	since := time.Now().AddDate(0, 0, -d)

	type row struct {
		Date         string
		Revenue      float64
		Transactions int
	}
	var rows []row
	err := r.AdminSvc.DB().WithContext(ctx).
		Table("transactions").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, COALESCE(SUM(amount), 0) as revenue, COUNT(*) as transactions").
		Where("type = ? AND amount > 0 AND created_at >= ?", "recharge", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]*model.RevenueChartPoint, len(rows))
	for i, row := range rows {
		out[i] = &model.RevenueChartPoint{
			Date:         row.Date,
			Revenue:      row.Revenue,
			Transactions: row.Transactions,
		}
	}
	return out, nil
}

// AdminUserGrowth is the resolver for the adminUserGrowth field.
func (r *queryResolver) AdminUserGrowth(ctx context.Context, days *int) ([]*model.UserGrowthPoint, error) {
	d := valInt(days, 30)
	since := time.Now().AddDate(0, 0, -d)

	type row struct {
		Date     string
		NewUsers int
	}
	var rows []row
	err := r.AdminSvc.DB().WithContext(ctx).
		Table("users").
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, COUNT(*) as new_users").
		Where("created_at >= ?", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Calculate cumulative total
	var totalBefore int64
	r.AdminSvc.DB().WithContext(ctx).Model(&models.User{}).Where("created_at < ?", since).Count(&totalBefore)

	out := make([]*model.UserGrowthPoint, len(rows))
	running := int(totalBefore)
	for i, row := range rows {
		running += row.NewUsers
		out[i] = &model.UserGrowthPoint{
			Date:       row.Date,
			NewUsers:   row.NewUsers,
			TotalUsers: running,
		}
	}
	return out, nil
}

// Users is the resolver for the users field.
func (r *queryResolver) Users(ctx context.Context, q *string, page *int, pageSize *int) (*model.UserConnection, error) {
	var users []models.User
	var err error
	if q != nil && *q != "" {
		users, err = r.UserSvc.SearchUsers(ctx, *q)
	} else {
		users, err = r.UserSvc.ListUsers(ctx)
	}
	if err != nil {
		return nil, err
	}
	p, ps := clampPagination(page, pageSize)
	total := len(users)
	start := (p - 1) * ps
	end := start + ps
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	paged := users[start:end]
	out := make([]*model.UserListItem, len(paged))
	for i := range paged {
		out[i] = userToListItem(&paged[i])
	}
	return &model.UserConnection{Data: out, Total: total}, nil
}

// User is the resolver for the user field.
func (r *queryResolver) User(ctx context.Context, id string) (*model.UserDetail, error) {
	uid, _ := uuid.Parse(id)
	u, err := r.UserSvc.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	ud := &model.UserDetail{
		ID: u.ID.String(), Email: u.Email, Name: u.Name,
		Role: u.Role, IsActive: u.IsActive,
		CreatedAt: u.CreatedAt,
	}
	summary, _ := r.Billing.GetUsageSummary(ctx, uid, nil, nil, monthStart(), time.Now())
	if summary != nil {
		ud.UsageMonth = &model.UserMonthlyUsage{
			TotalRequests: safeGQLInt(summary.TotalRequests),
			TotalTokens:   safeGQLInt(summary.TotalTokens),
			TotalCost:     summary.TotalCost,
		}
	}
	return ud, nil
}

// UserUsage is the resolver for the userUsage field.
func (r *queryResolver) UserUsage(ctx context.Context, id string, days *int) ([]*model.DailyStats, error) {
	uid, _ := uuid.Parse(id)
	d := valInt(days, 30)
	usage, err := r.Billing.GetDailyUsage(ctx, uid, nil, nil, d)
	if err != nil {
		return nil, err
	}
	out := make([]*model.DailyStats, len(usage))
	for i, u := range usage {
		out[i] = &model.DailyStats{Date: u.Date, Requests: int(u.Requests), TotalTokens: int(u.Tokens), TotalCost: u.Cost}
	}
	return out, nil
}

// UserAPIKeys is the resolver for the userApiKeys field.
func (r *queryResolver) UserAPIKeys(ctx context.Context, id string) ([]*model.APIKey, error) {
	uid, _ := uuid.Parse(id)
	keys, err := r.UserSvc.GetAPIKeys(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]*model.APIKey, len(keys))
	for i := range keys {
		out[i] = apiKeyToGQL(&keys[i])
	}
	return out, nil
}

// SystemSettings is the resolver for the systemSettings field.
func (r *queryResolver) SystemSettings(ctx context.Context) (*model.SystemSettings, error) {
	all, err := r.SystemConfig.GetAllSettingsDecrypted(ctx)
	if err != nil {
		return &model.SystemSettings{RegistrationMode: r.Config().Registration.Mode}, nil
	}
	return buildSystemSettings(r.Config().Registration.Mode, all), nil
}

// InviteCodes is the resolver for the inviteCodes field.
func (r *queryResolver) InviteCodes(ctx context.Context) ([]*model.InviteCode, error) {
	var codes []models.InviteCode
	if err := r.AdminSvc.DB().Order("created_at DESC").Find(&codes).Error; err != nil {
		return nil, err
	}
	out := make([]*model.InviteCode, len(codes))
	for i, c := range codes {
		out[i] = &model.InviteCode{
			ID: c.ID.String(), Code: c.Code, CreatedBy: c.CreatedBy.String(),
			MaxUses: c.MaxUses, UseCount: c.UseCount, ExpiresAt: c.ExpiresAt,
			IsActive: c.IsActive, CreatedAt: c.CreatedAt,
		}
	}
	return out, nil
}

// SystemAnomalyDetection is the resolver for the systemAnomalyDetection field.
func (r *queryResolver) SystemAnomalyDetection(ctx context.Context) (*model.AnomalyResult, error) {
	return &model.AnomalyResult{HasAnomaly: false}, nil
}

// RedeemCodes is the resolver for the redeemCodes field.
func (r *queryResolver) RedeemCodes(ctx context.Context, page *int, pageSize *int) (*model.RedeemCodeConnection, error) {
	p, ps := clampPagination(page, pageSize)
	codes, total, err := r.RedeemSvc.ListCodes(p, ps)
	if err != nil {
		return nil, err
	}
	out := make([]*model.RedeemCode, len(codes))
	for i, c := range codes {
		var usedBy, planID *string
		if c.UsedByID != nil {
			s := c.UsedByID.String()
			usedBy = &s
		}
		if c.PlanID != nil {
			s := c.PlanID.String()
			planID = &s
		}
		out[i] = &model.RedeemCode{
			ID: c.ID.String(), Code: c.Code, Type: c.Type,
			CreditAmount: c.CreditAmount, PlanID: planID,
			PlanDays: c.PlanDays,
			UsedBy:   usedBy, UsedAt: c.UsedAt,
			ExpiresAt: c.ExpiresAt, IsActive: c.IsActive,
			BatchID: c.BatchID, Note: &c.Note, CreatedAt: c.CreatedAt,
		}
	}
	return &model.RedeemCodeConnection{Nodes: out, Total: int(total)}, nil
}

// AuditLogs is the resolver for the auditLogs field.
func (r *queryResolver) AuditLogs(ctx context.Context, page *int, pageSize *int, action *string) (*model.AuditLogConnection, error) {
	p, ps := clampPagination(page, pageSize)

	query := r.AdminSvc.DB().Model(&models.AuditLog{})
	if action != nil && *action != "" {
		query = query.Where("action = ?", *action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var logs []models.AuditLog
	offset := (p - 1) * ps
	if err := query.Order("created_at DESC").Offset(offset).Limit(ps).Find(&logs).Error; err != nil {
		return nil, err
	}

	out := make([]*model.AuditLog, len(logs))
	for i, l := range logs {
		out[i] = &model.AuditLog{
			ID:        l.ID.String(),
			CreatedAt: l.CreatedAt,
			Action:    l.Action,
			ActorID:   l.ActorID.String(),
			TargetID:  l.TargetID.String(),
			IP:        l.IP,
			UserAgent: l.UserAgent,
			Detail:    l.Detail,
		}
	}

	return &model.AuditLogConnection{
		Data:     out,
		Total:    int(total),
		Page:     p,
		PageSize: ps,
	}, nil
}

// ErrorLogs is the resolver for the errorLogs field.
func (r *queryResolver) ErrorLogs(ctx context.Context, page *int, pageSize *int) (*model.ErrorLogConnection, error) {
	p, ps := clampPagination(page, pageSize)

	var total int64
	if err := r.AdminSvc.DB().Model(&models.ErrorLog{}).Count(&total).Error; err != nil {
		return nil, err
	}

	var list []models.ErrorLog
	if err := r.AdminSvc.DB().Order("created_at desc").Offset((p - 1) * ps).Limit(ps).Find(&list).Error; err != nil {
		return nil, err
	}

	out := make([]*model.ErrorLog, len(list))
	for i, l := range list {
		out[i] = &model.ErrorLog{
			ID:           l.ID.String(),
			TrajectoryID: l.TrajectoryID,
			TraceID:      l.TraceID,
			Provider:     l.Provider,
			Model:        l.Model,
			StatusCode:   l.StatusCode,
			Headers:      string(l.Headers),
			ResponseBody: string(l.ResponseBody),
			CreatedAt:    l.CreatedAt,
		}
	}

	return &model.ErrorLogConnection{
		Data:     out,
		Total:    int(total),
		Page:     p,
		PageSize: ps,
	}, nil
}

// RequestLogs is the resolver for the requestLogs field.
func (r *queryResolver) RequestLogs(ctx context.Context, requestID *string, level *string, startTime *string, endTime *string, limit *int) ([]*model.LogEntry, error) {
	// Clamp limit to avoid unbounded log pulls. Loki / DB queries will honor
	// whatever is passed in, so we must bound it here.
	clamped := clampLimit(limit, 100, 1000)
	entries, err := r.AdminSvc.GetRequestLogs(ctx, requestID, level, startTime, endTime, &clamped)
	if err != nil {
		r.Logger.Error("failed to get request logs", zap.Error(err), zap.Stringp("request_id", sanitize.SafeStringPtr(requestID)))
		return nil, fmt.Errorf("failed to fetch request logs: %w", err)
	}

	var result []*model.LogEntry
	for _, e := range entries {
		result = append(result, &model.LogEntry{
			Timestamp:  e.Timestamp,
			Level:      e.Level,
			Message:    e.Message,
			RequestID:  e.RequestID,
			Caller:     e.Caller,
			Error:      e.Error,
			Method:     e.Method,
			Path:       e.Path,
			StatusCode: e.StatusCode,
			Latency:    e.Latency,
			ClientIP:   e.ClientIP,
			UserAgent:  e.UserAgent,
			RawJSON:    e.RawJSON,
		})
	}
	return result, nil
}

// Integrations is the resolver for the integrations field.
func (r *queryResolver) Integrations(ctx context.Context) ([]*model.IntegrationConfig, error) {
	var list []models.IntegrationConfig
	if err := r.AdminSvc.DB().Find(&list).Error; err != nil {
		return nil, err
	}

	defaults := []string{"sentry", "loki", "langfuse"}
	existingNames := make(map[string]bool)
	for _, l := range list {
		existingNames[l.Name] = true
	}
	for _, n := range defaults {
		if !existingNames[n] {
			c := models.IntegrationConfig{
				ID:      uuid.New(),
				Name:    n,
				Enabled: false,
				Config:  []byte("{}"),
			}
			r.AdminSvc.DB().Create(&c)
			list = append(list, c)
		}
	}

	out := make([]*model.IntegrationConfig, len(list))
	for i, l := range list {
		out[i] = &model.IntegrationConfig{
			ID:        l.ID.String(),
			Name:      l.Name,
			Enabled:   l.Enabled,
			Config:    string(l.Config),
			UpdatedAt: l.UpdatedAt,
		}
	}
	return out, nil
}
