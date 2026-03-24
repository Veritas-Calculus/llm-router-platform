package resolvers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// clientInfo extracts client IP and User-Agent from the Gin context.
func clientInfo(ctx context.Context) (ip, userAgent string) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return "", ""
	}
	return gc.ClientIP(), gc.Request.UserAgent()
}

// ── JWT helpers ──────────────────────────────────────────────────────

func (r *mutationResolver) generateJWT(u *models.User) (string, error) {
	ttl := r.Config().JWT.ExpiresIn
	if ttl <= 0 {
		ttl = time.Hour // Default: 1 hour (prefer short-lived access tokens)
	}
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"role": u.Role,
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(r.Config().JWT.Secret))
}

func (r *mutationResolver) generateRefreshJWT(u *models.User) (string, error) {
	ttl := r.Config().JWT.RefreshExpiresIn
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour // Default: 7 days
	}
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"type": "refresh",
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(r.Config().JWT.Secret))
}

func (r *mutationResolver) validateRefreshJWT(tokenStr string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(r.Config().JWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	// Ensure this is a refresh token, not an access token
	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}

	sub, _ := claims["sub"].(string)
	return &jwt.RegisteredClaims{Subject: sub}, nil
}

// ── Model → GQL converters ──────────────────────────────────────────

func userToGQL(u *models.User) *model.User {
	balance := u.Balance
	return &model.User{
		ID: u.ID.String(), Email: u.Email, Name: u.Name,
		Role: u.Role, IsActive: u.IsActive, Balance: &balance,
		MonthlyBudgetUsd: &u.MonthlyBudgetUSD,
		EmailVerified:    u.EmailVerified,
		CreatedAt:        u.CreatedAt,
	}
}

func userToListItem(u *models.User) *model.UserListItem {
	return &model.UserListItem{
		ID: u.ID.String(), Email: u.Email, Name: u.Name,
		Role: u.Role, IsActive: u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}

func apiKeyToGQL(k *models.APIKey) *model.APIKey {
	var lastUsed, expires *time.Time
	if !k.LastUsedAt.IsZero() {
		lastUsed = &k.LastUsedAt
	}
	if !k.ExpiresAt.IsZero() {
		expires = &k.ExpiresAt
	}
	return &model.APIKey{
		ID: k.ID.String(), ProjectID: k.ProjectID.String(), Channel: k.Channel, Name: k.Name, KeyPrefix: k.KeyPrefix,
		IsActive: k.IsActive, Scopes: k.Scopes, RateLimit: k.RateLimit, TokenLimit: int(k.TokenLimit), DailyLimit: k.DailyLimit,
		LastUsedAt: lastUsed, ExpiresAt: expires, CreatedAt: k.CreatedAt,
	}
}

func orgToGQL(o *models.Organization) *model.Organization {
	return &model.Organization{
		ID:           o.ID.String(),
		Name:         o.Name,
		BillingLimit: o.BillingLimit,
		CreatedAt:    o.CreatedAt,
	}
}

func projectToGQL(p *models.Project) *model.Project {
	var desc *string
	if p.Description != "" {
		desc = &p.Description
	}
	var ips *string
	if p.WhiteListedIps != "" {
		ips = &p.WhiteListedIps
	}
	return &model.Project{
		ID:             p.ID.String(),
		OrgID:          p.OrgID.String(),
		Name:           p.Name,
		Description:    desc,
		QuotaLimit:     p.QuotaLimit,
		WhiteListedIps: ips,
		CreatedAt:      p.CreatedAt,
	}
}

func providerToGQL(p *models.Provider) *model.Provider {
	var proxyID *string
	if p.DefaultProxyID != nil {
		s := p.DefaultProxyID.String()
		proxyID = &s
	}
	return &model.Provider{
		ID: p.ID.String(), Name: p.Name, BaseURL: p.BaseURL,
		IsActive: p.IsActive, Priority: p.Priority, Weight: p.Weight,
		MaxRetries: p.MaxRetries, Timeout: p.Timeout,
		UseProxy: p.UseProxy, DefaultProxyID: proxyID,
		RequiresAPIKey: p.RequiresAPIKey,
		CreatedAt:      p.CreatedAt,
	}
}

func modelToGQL(m *models.Model) *model.Model {
	return &model.Model{
		ID:               m.ID.String(),
		ProviderID:       m.ProviderID.String(),
		Name:             m.Name,
		DisplayName:      m.DisplayName,
		InputPricePer1k:  m.InputPricePer1K,
		OutputPricePer1k: m.OutputPricePer1K,
		PricePerSecond:   &m.PricePerSecond,
		PricePerImage:    &m.PricePerImage,
		PricePerMinute:   &m.PricePerMinute,
		MaxTokens:        m.MaxTokens,
		IsActive:         m.IsActive,
		CreatedAt:        m.CreatedAt,
	}
}

func providerAPIKeyToGQL(k *models.ProviderAPIKey) *model.ProviderAPIKey {
	return &model.ProviderAPIKey{
		ID: k.ID.String(), ProviderID: k.ProviderID.String(),
		Alias: k.Alias, KeyPrefix: k.KeyPrefix,
		IsActive: k.IsActive, Priority: k.Priority,
		Weight: k.Weight, RateLimit: k.RateLimit,
		CreatedAt: k.CreatedAt,
	}
}

func proxyToGQL(p *models.Proxy) *model.Proxy {
	var upID *string
	if p.UpstreamProxyID != nil {
		s := p.UpstreamProxyID.String()
		upID = &s
	}
	return &model.Proxy{
		ID: p.ID.String(), URL: p.URL, Type: p.Type,
		Region: p.Region, IsActive: p.IsActive,
		UpstreamProxyID: upID, CreatedAt: p.CreatedAt,
	}
}

func alertToGQL(a *models.Alert) *model.Alert {
	return &model.Alert{
		ID: a.ID.String(), TargetType: a.TargetType,
		TargetID: a.TargetID.String(), AlertType: a.AlertType,
		Message: a.Message, Status: a.Status,
		CreatedAt: a.CreatedAt,
	}
}

func mcpServerToGQL(s *models.MCPServer) *model.McpServer {
	var args []string
	if len(s.Args) > 0 {
		_ = json.Unmarshal(s.Args, &args)
	}
	return &model.McpServer{
		ID: s.ID.String(), Name: s.Name, Type: s.Type,
		Command: &s.Command, URL: &s.URL,
		Args: args, IsActive: s.IsActive,
		Status: "active", CreatedAt: s.CreatedAt,
	}
}

func mcpToolToGQL(t *models.MCPTool) *model.McpTool {
	var schema *string
	if len(t.InputSchema) > 0 {
		s := string(t.InputSchema)
		schema = &s
	}
	return &model.McpTool{
		ID: t.ID.String(), ServerID: t.ServerID.String(),
		Name: t.Name, Description: t.Description,
		InputSchema: schema, IsActive: true,
	}
}

func asyncTaskToGQL(t *models.AsyncTask) *model.Task {
	var errMsg *string
	if t.Error != "" {
		errMsg = &t.Error
	}
	return &model.Task{
		ID: t.ID.String(), ProjectID: t.ProjectID.String(),
		Type: t.Type, Status: t.Status,
		Progress: t.Progress, Error: errMsg,
		CreatedAt: t.CreatedAt,
	}
}

func budgetToGQL(b *models.Budget) *model.Budget {
	var wh, em *string
	if b.WebhookURL != "" {
		wh = &b.WebhookURL
	}
	if b.Email != "" {
		em = &b.Email
	}
	return &model.Budget{
		ID: b.ID.String(), OrgID: b.OrgID.String(),
		MonthlyLimitUsd: b.MonthlyLimitUSD, AlertThreshold: b.AlertThreshold,
		EnforceHardLimit: b.EnforceHardLimit, IsActive: b.IsActive,
		WebhookURL: wh, Email: em,
	}
}

// ── Utility helpers ─────────────────────────────────────────────────

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefStrDefault(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

func derefBool(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func valInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func monthStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func announcementToGQL(a *models.Announcement) *model.Announcement {
	return &model.Announcement{
		ID: a.ID.String(), Title: a.Title, Content: a.Content,
		Type: a.Type, Priority: a.Priority, IsActive: a.IsActive,
		StartsAt: a.StartsAt, EndsAt: a.EndsAt,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func couponToGQL(c *models.Coupon) *model.Coupon {
	return &model.Coupon{
		ID: c.ID.String(), Code: c.Code, Name: c.Name,
		Type: c.Type, DiscountValue: c.DiscountValue,
		MinAmount: c.MinAmount, MaxUses: c.MaxUses,
		UseCount: c.UseCount, MaxUsesPerUser: c.MaxUsesPerUser,
		IsActive: c.IsActive, ExpiresAt: c.ExpiresAt,
		CreatedAt: c.CreatedAt,
	}
}

func documentToGQL(d *models.Document) *model.Document {
	return &model.Document{
		ID: d.ID.String(), Title: d.Title, Slug: d.Slug,
		Content: d.Content, Category: d.Category,
		SortOrder: d.SortOrder, IsPublished: d.IsPublished,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// buildSystemSettings assembles a SystemSettings from registration mode + category JSON map.
func buildSystemSettings(registrationMode string, all map[string]string) *model.SystemSettings {
	s := &model.SystemSettings{RegistrationMode: registrationMode}
	if v, ok := all["site"]; ok {
		s.Site = &v
	}
	if v, ok := all["security"]; ok {
		s.Security = &v
	}
	if v, ok := all["defaults"]; ok {
		s.Defaults = &v
	}
	if v, ok := all["email"]; ok {
		s.Email = &v
	}
	if v, ok := all["backup"]; ok {
		s.Backup = &v
	}
	if v, ok := all["payment"]; ok {
		s.Payment = &v
	}
	if v, ok := all["oauth"]; ok {
		s.Oauth = &v
	}
	return s
}

func routingRuleToGQL(rule *models.RoutingRule) *model.RoutingRule {
	var targetProvider, fallbackProvider *model.Provider
	if rule.TargetProvider != nil {
		targetProvider = providerToGQL(rule.TargetProvider)
	}
	if rule.FallbackProvider != nil {
		fallbackProvider = providerToGQL(rule.FallbackProvider)
	}

	var fallbackID *string
	if rule.FallbackProviderID != nil {
		s := rule.FallbackProviderID.String()
		fallbackID = &s
	}

	return &model.RoutingRule{
		ID:                 rule.ID.String(),
		Name:               rule.Name,
		Description:        rule.Description,
		ModelPattern:       rule.ModelPattern,
		TargetProviderID:   rule.TargetProviderID.String(),
		FallbackProviderID: fallbackID,
		Priority:           rule.Priority,
		IsEnabled:          rule.IsEnabled,
		CreatedAt:          rule.CreatedAt,
		UpdatedAt:          rule.UpdatedAt,
		TargetProvider:     targetProvider,
		FallbackProvider:   fallbackProvider,
	}
}

func (r *Resolver) resolveOrgID(ctx context.Context, providedOrgID *string) (uuid.UUID, error) {
	if providedOrgID != nil && *providedOrgID != "" {
		return uuid.Parse(*providedOrgID)
	}
	uidStr, _ := directives.UserIDFromContext(ctx)
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID in context")
	}

	orgs, err := r.UserSvc.GetOrganizations(ctx, userID)
	if err != nil || len(orgs) == 0 {
		return uuid.Nil, fmt.Errorf("no organization found for user")
	}
	return orgs[0].ID, nil
}

func (r *Resolver) resolveProjectID(providedProjectID *string) *uuid.UUID {
	if providedProjectID != nil && *providedProjectID != "" {
		id, err := uuid.Parse(*providedProjectID)
		if err == nil {
			return &id
		}
	}
	return nil
}

func mapIdentityProviderToGraphQL(idp *models.IdentityProvider) *model.IdentityProvider {
	if idp == nil {
		return nil
	}
	return &model.IdentityProvider{
		ID:               idp.ID.String(),
		OrgID:            idp.OrgID.String(),
		Type:             idp.Type,
		Name:             idp.Name,
		IsActive:         idp.IsActive,
		Domains:          idp.Domains,
		OidcClientID:     &idp.OIDCClientID,
		OidcIssuerURL:    &idp.OIDCIssuerURL,
		SamlEntityID:     &idp.SAMLEntityID,
		SamlSsoURL:       &idp.SAMLSSOURL,
		SamlIdpCert:      &idp.SAMLIdPCert,
		EnableJit:        idp.EnableJIT,
		DefaultRole:      idp.DefaultRole,
		GroupRoleMapping: idp.GroupRoleMapping,
		CreatedAt:        idp.CreatedAt,
		UpdatedAt:        idp.UpdatedAt,
	}
}

func (r *Resolver) resolveOrgProjectIDs(ctx context.Context, providedOrgID *string, providedProjectID *string) (uuid.UUID, *uuid.UUID, error) {
	orgID, err := r.resolveOrgID(ctx, providedOrgID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	projectID := r.resolveProjectID(providedProjectID)
	return orgID, projectID, nil
}

// ── Prompt helpers ──────────────────────────────────────────────────

func promptTemplateToGQL(t *models.PromptTemplate, versionCount int) *model.PromptTemplate {
	result := &model.PromptTemplate{
		ID:           t.ID.String(),
		Name:         t.Name,
		Description:  t.Description,
		IsActive:     t.IsActive,
		VersionCount: versionCount,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
	if t.ProjectID != nil {
		pid := t.ProjectID.String()
		result.ProjectID = &pid
	}
	if t.ActiveVersionID != nil {
		avid := t.ActiveVersionID.String()
		result.ActiveVersionID = &avid
	}
	return result
}

func promptVersionToGQL(v *models.PromptVersion) *model.PromptVersion {
	result := &model.PromptVersion{
		ID:         v.ID.String(),
		TemplateID: v.TemplateID.String(),
		Version:    v.Version,
		Content:    v.Content,
		CreatedAt:  v.CreatedAt,
	}
	if v.Model != "" {
		result.Model = &v.Model
	}
	if len(v.Parameters) > 0 {
		p := string(v.Parameters)
		result.Parameters = &p
	}
	if v.ChangeLog != "" {
		result.ChangeLog = &v.ChangeLog
	}
	return result
}

func cacheConfigToGQL(cfg *models.CacheConfig) *model.CacheConfig {
	return &model.CacheConfig{
		ID:                  cfg.ID.String(),
		IsEnabled:           cfg.IsEnabled,
		SimilarityThreshold: cfg.SimilarityThreshold,
		DefaultTTLMinutes:   cfg.DefaultTTLMinutes,
		EmbeddingModel:      cfg.EmbeddingModel,
		MaxCacheSize:        cfg.MaxCacheSize,
	}
}
