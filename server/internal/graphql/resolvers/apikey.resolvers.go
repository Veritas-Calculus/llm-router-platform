package resolvers

// This file contains apikey domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"fmt"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/pkg/sanitize"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateAPIKey is the resolver for the createApiKey field.
func (r *mutationResolver) CreateAPIKey(ctx context.Context, projectID string, name string, scopes *string, rateLimit *int, tokenLimit *int) (*model.APIKeyWithSecret, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	if err := r.UserSvc.RequireProjectRole(ctx, uid, projectID, "admin", "member"); err != nil {
		r.Logger.Error("RequireProjectRole failed in CreateAPIKey", zap.Error(err), zap.String("uid", sanitize.LogValue(uid)), zap.String("projectID", sanitize.LogValue(projectID)))
		return nil, err
	}

	id, err := uuid.Parse(projectID)
	if err != nil {
		r.Logger.Error("UUID parse failed in CreateAPIKey", zap.Error(err), zap.String("projectID", sanitize.LogValue(projectID)))
		return nil, fmt.Errorf("invalid project ID")
	}

	userID, err := uuid.Parse(uid)
	if err != nil {
		r.Logger.Error("User UUID parse failed in CreateAPIKey", zap.Error(err), zap.String("uid", sanitize.LogValue(uid)))
		return nil, fmt.Errorf("invalid user ID")
	}

	scopeStr := "all"
	if scopes != nil && *scopes != "" {
		scopeStr = *scopes
	}

	key, secret, err := r.UserSvc.CreateAPIKey(ctx, userID, id, name, scopeStr, rateLimit, tokenLimit)
	if err != nil {
		r.Logger.Error("Failed to create API key in resolver", zap.Error(err), zap.String("projectID", sanitize.LogValue(projectID)))
		return nil, err
	}

	r.Logger.Info("Successfully created API Key in DB", zap.String("keyID", key.ID.String()))

	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionAPIKeyCreate, id, key.ID, ip, ua, map[string]interface{}{"name": name})

	return &model.APIKeyWithSecret{
		ID:         key.ID.String(),
		ProjectID:  key.ProjectID.String(),
		Channel:    key.Channel,
		Name:       key.Name,
		Key:        secret,
		KeyPrefix:  key.KeyPrefix,
		IsActive:   key.IsActive,
		Scopes:     key.Scopes,
		RateLimit:  key.RateLimit,
		TokenLimit: int(key.TokenLimit),
		DailyLimit: key.DailyLimit,
		ExpiresAt:  &key.ExpiresAt,
		CreatedAt:  key.CreatedAt,
	}, nil
}

// UpdateAPIKey is the resolver for the updateApiKey field.
func (r *mutationResolver) UpdateAPIKey(ctx context.Context, id string, name *string, scopes *string, rateLimit *int, tokenLimit *int, isActive *bool) (*model.APIKey, error) {
	uid, _ := directives.UserIDFromContext(ctx)

	keyID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID")
	}

	// Resolve the key's owning project and enforce membership before mutation.
	existing, err := r.UserSvc.GetAPIKeyByID(ctx, keyID)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("API key not found")
	}
	if err := r.UserSvc.RequireProjectRole(ctx, uid, existing.ProjectID.String(), "admin", "member"); err != nil {
		r.Logger.Warn("UpdateAPIKey authorization denied",
			zap.String("uid", sanitize.LogValue(uid)),
			zap.String("key_id", keyID.String()))
		return nil, err
	}

	key, err := r.UserSvc.UpdateAPIKey(ctx, keyID, name, scopes, rateLimit, tokenLimit, isActive)
	if err != nil {
		return nil, err
	}

	ip, ua := clientInfo(ctx)
	userID, _ := uuid.Parse(uid)
	r.AuditService.Log(ctx, audit.ActionAPIKeyRevoke, userID, keyID, ip, ua, map[string]interface{}{"event": "update"})

	return apiKeyToGQL(key), nil
}

// RevokeAPIKey is the resolver for the revokeApiKey field.
func (r *mutationResolver) RevokeAPIKey(ctx context.Context, projectID string, id string) (*model.APIKey, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	if err := r.UserSvc.RequireProjectRole(ctx, uid, projectID, "admin", "member"); err != nil {
		return nil, err
	}

	userID, _ := uuid.Parse(uid)
	keyID, _ := uuid.Parse(id)
	if err := r.UserSvc.RevokeAPIKey(ctx, userID, keyID); err != nil {
		return nil, err
	}
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionAPIKeyRevoke, userID, keyID, ip, ua, nil)
	key, _ := r.UserSvc.GetAPIKeyByID(ctx, keyID)
	return apiKeyToGQL(key), nil
}

// DeleteAPIKey is the resolver for the deleteApiKey field.
func (r *mutationResolver) DeleteAPIKey(ctx context.Context, projectID string, id string) (bool, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	if err := r.UserSvc.RequireProjectRole(ctx, uid, projectID, "admin"); err != nil {
		return false, err
	}

	userID, _ := uuid.Parse(uid)
	keyID, _ := uuid.Parse(id)
	if err := r.UserSvc.DeleteAPIKey(ctx, userID, keyID); err != nil {
		return false, err
	}
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionAPIKeyRevoke, userID, keyID, ip, ua, map[string]interface{}{"action": "delete"})
	return true, nil
}

// MyAPIKeys is the resolver for the myApiKeys field.
func (r *queryResolver) MyAPIKeys(ctx context.Context, projectID string) ([]*model.APIKey, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	if err := r.UserSvc.RequireProjectRole(ctx, uid, projectID, "admin", "member"); err != nil {
		return nil, err
	}

	pId, err := uuid.Parse(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID")
	}
	keys, err := r.UserSvc.GetAPIKeys(ctx, pId)
	if err != nil {
		return nil, err
	}
	out := make([]*model.APIKey, len(keys))
	for i := range keys {
		out[i] = apiKeyToGQL(&keys[i])
	}
	return out, nil
}

// APIKeyRateLimitStatus is the resolver for the apiKeyRateLimitStatus field.
func (r *queryResolver) APIKeyRateLimitStatus(ctx context.Context, keyID string) (*model.APIKeyRateLimitStatus, error) {
	uid, _ := directives.UserIDFromContext(ctx)

	parsedID, err := uuid.Parse(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID")
	}

	// Look up API key to get its configured limits
	var apiKey models.APIKey
	if err := r.AdminSvc.DB().Where("id = ?", parsedID).First(&apiKey).Error; err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	// Enforce project-level ownership before exposing counters.
	if err := r.UserSvc.RequireProjectRole(ctx, uid, apiKey.ProjectID.String(), "admin", "member", "readonly"); err != nil {
		return nil, err
	}

	result := &model.APIKeyRateLimitStatus{
		KeyID:      keyID,
		RpmLimit:   apiKey.RateLimit,
		TpmLimit:   int(apiKey.TokenLimit),
		DailyLimit: apiKey.DailyLimit,
		Status:     "ok",
	}

	if r.RedisClient() != nil {
		r.readRateLimitCounters(result, &apiKey, keyID)
		computeRateLimitStatus(result, &apiKey)
	}

	// Check subscription plan quota (project -> org -> subscription)
	if r.SubscriptionSvc != nil {
		var proj struct{ OrgID uuid.UUID }
		if err := r.AdminSvc.DB().Table("projects").Select("org_id").Where("id = ?", apiKey.ProjectID).First(&proj).Error; err == nil {
			if allowed, _, _ := r.SubscriptionSvc.CheckQuota(ctx, proj.OrgID); !allowed {
				result.Status = "quota_exceeded"
			}
		}
	}

	return result, nil
}

// readRateLimitCounters reads RPM, TPM, and daily counters from Redis for the given API key.
func (r *queryResolver) readRateLimitCounters(result *model.APIKeyRateLimitStatus, apiKey *models.APIKey, keyID string) {
	rctx := context.Background()
	now := time.Now()

	// 1. RPM — per-key sliding window sorted set (nanosecond timestamps)
	rpmKey := fmt.Sprintf("rl:key:%s:m", keyID)
	windowStartNano := now.Add(-time.Minute)
	rpmCount, _ := r.RedisClient().ZCount(rctx, rpmKey,
		strconv.FormatInt(windowStartNano.UnixNano(), 10),
		strconv.FormatInt(now.UnixNano(), 10)).Result()

	// Also check global rate limiter sorted set (millisecond timestamps)
	globalKey := fmt.Sprintf("ratelimit:%s", keyID)
	windowStartMs := now.Add(-time.Minute).UnixMilli()
	globalCount, _ := r.RedisClient().ZCount(rctx, globalKey,
		strconv.FormatInt(windowStartMs, 10),
		strconv.FormatInt(now.UnixMilli(), 10)).Result()

	// Use the higher of the two RPM counters
	effectiveRpm := rpmCount
	if globalCount > effectiveRpm {
		effectiveRpm = globalCount
	}
	result.RpmCurrent = int(effectiveRpm)
	if apiKey.RateLimit > 0 && effectiveRpm >= int64(apiKey.RateLimit) {
		result.RpmExceeded = true
	}

	// 2. TPM — simple counter keyed by minute
	tpmKey := fmt.Sprintf("rl:tpm:%s:%d", keyID, now.Unix()/60)
	tpmStr, _ := r.RedisClient().Get(rctx, tpmKey).Result()
	tpmVal, _ := strconv.ParseInt(tpmStr, 10, 32)
	result.TpmCurrent = int(tpmVal)
	if apiKey.TokenLimit > 0 && tpmVal >= apiKey.TokenLimit {
		result.TpmExceeded = true
	}

	// 3. Daily — simple counter keyed by date
	today := now.Format("2006-01-02")
	dailyKey := fmt.Sprintf("rl:key:%s:d:%s", keyID, today)
	dailyCount, _ := r.RedisClient().Get(rctx, dailyKey).Result()
	dVal, _ := strconv.ParseInt(dailyCount, 10, 32)
	result.DailyCurrent = int(dVal)
	if apiKey.DailyLimit > 0 && dVal >= int64(apiKey.DailyLimit) {
		result.DailyExceeded = true
	}
}

// computeRateLimitStatus determines the overall rate limit status based on counter values.
func computeRateLimitStatus(result *model.APIKeyRateLimitStatus, apiKey *models.APIKey) {
	if result.RpmExceeded || result.TpmExceeded || result.DailyExceeded {
		result.Status = "rate_limited"
		return
	}

	// Near limit: any counter > 80% of its limit
	if apiKey.RateLimit > 0 && float64(result.RpmCurrent)/float64(apiKey.RateLimit) > 0.8 {
		result.Status = "near_limit"
		return
	}
	if apiKey.TokenLimit > 0 && float64(result.TpmCurrent)/float64(apiKey.TokenLimit) > 0.8 {
		result.Status = "near_limit"
		return
	}
	if apiKey.DailyLimit > 0 && float64(result.DailyCurrent)/float64(apiKey.DailyLimit) > 0.8 {
		result.Status = "near_limit"
	}
}

