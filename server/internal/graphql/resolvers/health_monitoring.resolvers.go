package resolvers

// This file contains health_monitoring domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"encoding/json"
	"fmt"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CheckAPIKeyHealth is the resolver for the checkApiKeyHealth field.
func (r *mutationResolver) CheckAPIKeyHealth(ctx context.Context, id string) (*model.APIKeyHealth, error) {
	kid, _ := uuid.Parse(id)
	s, err := r.Health.CheckSingleAPIKey(ctx, kid)
	if err != nil {
		return nil, err
	}
	var lc *time.Time
	if !s.LastCheck.IsZero() {
		lc = &s.LastCheck
	}
	return &model.APIKeyHealth{
		ID: s.ID.String(), ProviderID: s.ProviderID.String(),
		ProviderName: s.ProviderName, KeyPrefix: s.KeyPrefix,
		IsActive: s.IsActive, IsHealthy: s.IsHealthy,
		LastCheck: lc, ResponseTime: float64(s.ResponseTime), SuccessRate: s.SuccessRate,
	}, nil
}

// CheckProxyHealth is the resolver for the checkProxyHealth field.
func (r *mutationResolver) CheckProxyHealth(ctx context.Context, id string) (*model.ProxyHealth, error) {
	pid, _ := uuid.Parse(id)
	s, err := r.Health.CheckSingleProxy(ctx, pid)
	if err != nil {
		return nil, err
	}
	var lc *time.Time
	if !s.LastCheck.IsZero() {
		lc = &s.LastCheck
	}
	return &model.ProxyHealth{
		ID: s.ID.String(), URL: s.URL, Type: s.Type, Region: s.Region,
		IsActive: s.IsActive, IsHealthy: s.IsHealthy,
		ResponseTime: float64(s.ResponseTime), LastCheck: lc, SuccessRate: s.SuccessRate,
	}, nil
}

// CheckProviderHealth is the resolver for the checkProviderHealth field.
func (r *mutationResolver) CheckProviderHealth(ctx context.Context, id string) (*model.ProviderHealth, error) {
	pid, _ := uuid.Parse(id)
	s, err := r.Health.CheckSingleProvider(ctx, pid)
	if err != nil {
		return nil, err
	}
	var lc *time.Time
	if !s.LastCheck.IsZero() {
		lc = &s.LastCheck
	}
	var em *string
	if s.ErrorMessage != "" {
		em = &s.ErrorMessage
	}
	return &model.ProviderHealth{
		ID: s.ID.String(), Name: s.Name, BaseURL: s.BaseURL,
		IsActive: s.IsActive, IsHealthy: s.IsHealthy, UseProxy: s.UseProxy,
		ResponseTime: float64(s.ResponseTime), LastCheck: lc,
		SuccessRate: s.SuccessRate, ErrorMessage: em,
	}, nil
}

// CheckAllProviderHealth is the resolver for the checkAllProviderHealth field.
func (r *mutationResolver) CheckAllProviderHealth(ctx context.Context) ([]*model.ProviderHealth, error) {
	if err := r.Health.CheckAllProviders(ctx); err != nil {
		return nil, err
	}
	qr := &queryResolver{r.Resolver}
	return qr.HealthProviders(ctx)
}

// AcknowledgeAlert is the resolver for the acknowledgeAlert field.
func (r *mutationResolver) AcknowledgeAlert(ctx context.Context, id string) (*model.Alert, error) {
	aid, _ := uuid.Parse(id)
	if err := r.Health.AcknowledgeAlert(ctx, aid); err != nil {
		return nil, err
	}
	var alert models.Alert
	if err := r.AdminSvc.DB().First(&alert, "id = ?", aid).Error; err != nil {
		return nil, err
	}
	return alertToGQL(&alert), nil
}

// ResolveAlert is the resolver for the resolveAlert field.
func (r *mutationResolver) ResolveAlert(ctx context.Context, id string) (*model.Alert, error) {
	aid, _ := uuid.Parse(id)
	if err := r.Health.ResolveAlert(ctx, aid); err != nil {
		return nil, err
	}
	var alert models.Alert
	if err := r.AdminSvc.DB().First(&alert, "id = ?", aid).Error; err != nil {
		return nil, err
	}
	return alertToGQL(&alert), nil
}

// UpdateAlertConfig is the resolver for the updateAlertConfig field.
func (r *mutationResolver) UpdateAlertConfig(ctx context.Context, input model.AlertConfigInput) (*model.AlertConfig, error) {
	targetID, _ := uuid.Parse(input.TargetID)
	if input.WebhookURL != nil {
		if err := sanitize.ValidateWebhookURL(*input.WebhookURL, false, r.Config().Server.AllowLocalProviders); err != nil {
			return nil, fmt.Errorf("invalid webhook URL: %w", err)
		}
	}
	config := &models.AlertConfig{
		TargetType: input.TargetType, TargetID: targetID,
		IsEnabled: input.IsEnabled, FailureThreshold: input.FailureThreshold,
	}
	if input.ErrorRateThreshold != nil {
		config.ErrorRateThreshold = *input.ErrorRateThreshold
	}
	if input.LatencyThresholdMs != nil {
		config.LatencyThresholdMs = *input.LatencyThresholdMs
	}
	if input.BudgetThreshold != nil {
		config.BudgetThreshold = *input.BudgetThreshold
	}
	if input.CooldownMinutes != nil {
		config.CooldownMinutes = *input.CooldownMinutes
	}
	if input.WebhookURL != nil {
		config.WebhookURL = *input.WebhookURL
	}
	if input.Email != nil {
		config.Email = *input.Email
	}
	if err := r.Health.UpdateAlertConfig(ctx, config); err != nil {
		return nil, err
	}
	return &model.AlertConfig{
		TargetType: config.TargetType, TargetID: config.TargetID.String(),
		IsEnabled: config.IsEnabled, FailureThreshold: config.FailureThreshold,
		ErrorRateThreshold: config.ErrorRateThreshold, LatencyThresholdMs: config.LatencyThresholdMs,
		BudgetThreshold: config.BudgetThreshold, CooldownMinutes: config.CooldownMinutes,
		WebhookURL: input.WebhookURL, Email: input.Email,
	}, nil
}

// Alerts is the resolver for the alerts field.
func (r *queryResolver) Alerts(ctx context.Context, status *string) (*model.AlertConnection, error) {
	s := ""
	if status != nil {
		s = *status
	}
	alerts, total, err := r.Health.GetAlerts(ctx, s, 1, 100)
	if err != nil {
		return &model.AlertConnection{Data: []*model.Alert{}, Total: 0}, nil
	}
	out := make([]*model.Alert, len(alerts))
	for i := range alerts {
		out[i] = alertToGQL(&alerts[i])
	}
	return &model.AlertConnection{Data: out, Total: int(total)}, nil
}

// AlertConfig is the resolver for the alertConfig field.
func (r *queryResolver) AlertConfig(ctx context.Context, targetType string, targetID string) (*model.AlertConfig, error) {
	tid, _ := uuid.Parse(targetID)
	cfg, err := r.Health.GetAlertConfig(ctx, targetType, tid)
	if err != nil || cfg == nil {
		return nil, nil
	}
	var wh, em *string
	if cfg.WebhookURL != "" {
		wh = &cfg.WebhookURL
	}
	if cfg.Email != "" {
		em = &cfg.Email
	}
	idStr := cfg.ID.String()
	return &model.AlertConfig{
		ID: &idStr, TargetType: cfg.TargetType, TargetID: cfg.TargetID.String(),
		IsEnabled: cfg.IsEnabled, FailureThreshold: cfg.FailureThreshold,
		ErrorRateThreshold: cfg.ErrorRateThreshold, LatencyThresholdMs: cfg.LatencyThresholdMs,
		BudgetThreshold: cfg.BudgetThreshold, CooldownMinutes: cfg.CooldownMinutes,
		WebhookURL: wh, Email: em,
	}, nil
}

// HealthAPIKeys is the resolver for the healthApiKeys field.
func (r *queryResolver) HealthAPIKeys(ctx context.Context) ([]*model.APIKeyHealth, error) {
	statuses, err := r.Health.GetAPIKeysHealth(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.APIKeyHealth, len(statuses))
	for i, s := range statuses {
		var lc *time.Time
		if !s.LastCheck.IsZero() {
			lc = &s.LastCheck
		}
		out[i] = &model.APIKeyHealth{
			ID: s.ID.String(), ProviderID: s.ProviderID.String(),
			ProviderName: s.ProviderName, KeyPrefix: s.KeyPrefix,
			IsActive: s.IsActive, IsHealthy: s.IsHealthy,
			LastCheck: lc, ResponseTime: float64(s.ResponseTime), SuccessRate: s.SuccessRate,
		}
	}
	return out, nil
}

// HealthProxies is the resolver for the healthProxies field.
func (r *queryResolver) HealthProxies(ctx context.Context) ([]*model.ProxyHealth, error) {
	statuses, err := r.Health.GetProxiesHealth(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProxyHealth, len(statuses))
	for i, s := range statuses {
		var lc *time.Time
		if !s.LastCheck.IsZero() {
			lc = &s.LastCheck
		}
		out[i] = &model.ProxyHealth{
			ID: s.ID.String(), URL: s.URL, Type: s.Type, Region: s.Region,
			IsActive: s.IsActive, IsHealthy: s.IsHealthy,
			ResponseTime: float64(s.ResponseTime), LastCheck: lc, SuccessRate: s.SuccessRate,
		}
	}
	return out, nil
}

// HealthProviders is the resolver for the healthProviders field.
func (r *queryResolver) HealthProviders(ctx context.Context) ([]*model.ProviderHealth, error) {
	statuses, err := r.Health.GetProvidersHealth(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProviderHealth, len(statuses))
	for i, s := range statuses {
		var lc *time.Time
		if !s.LastCheck.IsZero() {
			lc = &s.LastCheck
		}
		var em *string
		if s.ErrorMessage != "" {
			em = &s.ErrorMessage
		}
		out[i] = &model.ProviderHealth{
			ID: s.ID.String(), Name: s.Name, BaseURL: s.BaseURL,
			IsActive: s.IsActive, IsHealthy: s.IsHealthy, UseProxy: s.UseProxy,
			ResponseTime: float64(s.ResponseTime), LastCheck: lc,
			SuccessRate: s.SuccessRate, ErrorMessage: em,
		}
	}
	return out, nil
}

// HealthHistory is the resolver for the healthHistory field.
func (r *queryResolver) HealthHistory(ctx context.Context) ([]*model.HealthEvent, error) {
	history, err := r.Health.GetHealthHistory(ctx, "", 100)
	if err != nil {
		return []*model.HealthEvent{}, nil
	}
	out := make([]*model.HealthEvent, len(history))
	for i, h := range history {
		var msg *string
		if h.ErrorMessage != "" {
			msg = &h.ErrorMessage
		}
		status := "healthy"
		if !h.IsHealthy {
			status = "unhealthy"
		}
		out[i] = &model.HealthEvent{
			ID: h.ID.String(), TargetType: h.TargetType,
			TargetID: h.TargetID.String(), Status: status,
			Message: msg, CreatedAt: h.CreatedAt,
		}
	}
	return out, nil
}

// SystemStatus is the resolver for the systemStatus field.
func (r *queryResolver) SystemStatus(ctx context.Context) (*model.SystemStatus, error) {
	if r.MonitoringSvc == nil {
		return nil, fmt.Errorf("monitoring service not initialized")
	}

	status := r.MonitoringSvc.CollectStatus(ctx)

	// Map dependencies
	deps := make([]*model.DependencyStatus, len(status.Dependencies))
	for i, d := range status.Dependencies {
		dep := &model.DependencyStatus{
			Name:      d.Name,
			Status:    d.Status,
			LatencyMs: d.LatencyMs,
		}
		if d.Version != "" {
			dep.Version = &d.Version
		}
		if d.Details != "" {
			dep.Details = &d.Details
		}
		deps[i] = dep
	}

	return &model.SystemStatus{
		Service: &model.ServiceInfo{
			Version:    status.Service.Version,
			GitCommit:  status.Service.GitCommit,
			BuildTime:  status.Service.BuildTime,
			Uptime:     status.Service.Uptime,
			ConfigMode: status.Service.ConfigMode,
		},
		Runtime: &model.RuntimeInfo{
			Goroutines:  status.Runtime.Goroutines,
			HeapAllocMb: status.Runtime.HeapAllocMB,
			HeapSysMb:   status.Runtime.HeapSysMB,
			GcPauseMs:   status.Runtime.GCPauseMs,
			NumGc:       status.Runtime.NumGC,
			CPUCores:    status.Runtime.CPUCores,
		},
		Dependencies:  deps,
		OverallStatus: status.OverallStatus,
	}, nil
}

// SystemLoad is the resolver for the systemLoad field.
func (r *queryResolver) SystemLoad(ctx context.Context) (*model.SystemLoad, error) {
	if r.MonitoringSvc == nil {
		return nil, fmt.Errorf("monitoring service not initialized")
	}

	load := r.MonitoringSvc.CollectLoad(ctx)

	return &model.SystemLoad{
		Service: &model.ServiceLoad{
			RequestsInFlight:  load.Service.RequestsInFlight,
			RequestsPerSecond: load.Service.RequestsPerSecond,
			AvgLatencyMs:      load.Service.AvgLatencyMs,
			P95LatencyMs:      load.Service.P95LatencyMs,
			ErrorRate:         load.Service.ErrorRate,
		},
		Database: &model.DatabaseLoad{
			ActiveConnections:     load.Database.ActiveConnections,
			MaxConnections:        load.Database.MaxConnections,
			PoolIdle:              load.Database.PoolIdle,
			PoolInUse:             load.Database.PoolInUse,
			TransactionsPerSecond: load.Database.TransactionsPerSecond,
			CacheHitRate:          load.Database.CacheHitRate,
			Deadlocks:             load.Database.Deadlocks,
		},
		Redis: &model.RedisLoad{
			ConnectedClients: load.Redis.ConnectedClients,
			UsedMemoryMb:     load.Redis.UsedMemoryMB,
			MaxMemoryMb:      load.Redis.MaxMemoryMB,
			OpsPerSecond:     load.Redis.OpsPerSecond,
			HitRate:          load.Redis.HitRate,
			KeyCount:         load.Redis.KeyCount,
		},
	}, nil
}

// BackupStatus is the resolver for the backupStatus field.
func (r *queryResolver) BackupStatus(ctx context.Context) (*model.BackupStatus, error) {
	result := &model.BackupStatus{
		Records:         []*model.BackupRecord{},
		IsConfigured:    false,
		ScheduleEnabled: false,
	}

	// Check backup configuration
	all, err := r.SystemConfig.GetAllSettingsDecrypted(ctx)
	if err == nil {
		if backupJSON, ok := all["backup"]; ok {
			var backupCfg struct {
				Enabled    bool   `json:"enabled"`
				S3Endpoint string `json:"s3Endpoint"`
				S3Bucket   string `json:"s3Bucket"`
				Schedule   string `json:"schedule"`
			}
			if json.Unmarshal([]byte(backupJSON), &backupCfg) == nil {
				result.IsConfigured = backupCfg.S3Bucket != ""
				result.ScheduleEnabled = backupCfg.Enabled
			}
		}
	}

	// Fetch recent backup records (last 20)
	var dbRecords []models.BackupRecord
	if err := r.AdminSvc.DB().WithContext(ctx).Order("started_at DESC").Limit(20).Find(&dbRecords).Error; err != nil {
		r.Logger.Warn("failed to fetch backup records", zap.Error(err))
		return result, nil
	}

	for _, rec := range dbRecords {
		br := &model.BackupRecord{
			ID:          rec.ID.String(),
			Type:        rec.Type,
			Status:      rec.Status,
			SizeBytes:   int(rec.SizeBytes),
			DurationMs:  int(rec.DurationMs),
			Destination: rec.Destination,
			StartedAt:   rec.StartedAt,
			CompletedAt: rec.CompletedAt,
		}
		if rec.ErrorMsg != "" {
			br.ErrorMessage = &rec.ErrorMsg
		}
		result.Records = append(result.Records, br)
	}

	// Set last backup
	if len(result.Records) > 0 {
		result.LastBackup = result.Records[0]
	}

	return result, nil
}

// SystemSLA is the resolver for the systemSla field.
func (r *queryResolver) SystemSLA(ctx context.Context, hours *int) (*model.SystemSLA, error) {
	h := 24
	if hours != nil {
		h = *hours
	}
	cutoff := time.Now().Add(-time.Duration(h) * time.Hour)

	var totalRequests int64
	r.AdminSvc.DB().Model(&models.UsageLog{}).Where("created_at >= ?", cutoff).Count(&totalRequests)

	var failedRequests int64
	r.AdminSvc.DB().Model(&models.UsageLog{}).Where("created_at >= ? AND status_code >= ?", cutoff, 400).Count(&failedRequests)

	failureRate := 0.0
	if totalRequests > 0 {
		failureRate = float64(failedRequests) / float64(totalRequests)
	}

	var avgLatency float64
	r.AdminSvc.DB().Model(&models.UsageLog{}).Where("created_at >= ? AND latency > 0", cutoff).Select("COALESCE(AVG(latency), 0)").Scan(&avgLatency)

	var p95, p99 float64
	if r.AdminSvc.DB().Name() == "postgres" {
		_ = r.AdminSvc.DB().Raw("SELECT COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency), 0), COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency), 0) FROM usage_logs WHERE created_at >= ? AND latency > 0", cutoff).Row().Scan(&p95, &p99)
	} else {
		var latencies []float64
		r.AdminSvc.DB().Model(&models.UsageLog{}).Where("created_at >= ? AND latency > 0", cutoff).Order("latency asc").Pluck("latency", &latencies)
		if len(latencies) > 0 {
			p95Idx := int(float64(len(latencies)-1) * 0.95)
			p99Idx := int(float64(len(latencies)-1) * 0.99)
			p95 = latencies[p95Idx]
			p99 = latencies[p99Idx]
		}
	}

	var activeProviders int64
	r.AdminSvc.DB().Model(&models.Provider{}).Where("is_active = ?", true).Count(&activeProviders)

	healthyProviders := 0
	healths, err := r.Health.GetProvidersHealth(ctx)
	if err == nil {
		for _, p := range healths {
			if p.IsHealthy {
				healthyProviders++
			}
		}
	}

	return &model.SystemSLA{
		TotalRequests:    int(totalRequests),
		FailureRate:      failureRate,
		AvgLatencyMs:     avgLatency,
		P95LatencyMs:     p95,
		P99LatencyMs:     p99,
		ActiveProviders:  int(activeProviders),
		HealthyProviders: healthyProviders,
	}, nil
}
