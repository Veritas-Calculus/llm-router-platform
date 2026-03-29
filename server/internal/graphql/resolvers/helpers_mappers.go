package resolvers

// Domain helpers: helpers_mappers

import (
	"encoding/json"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"time"
)

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
