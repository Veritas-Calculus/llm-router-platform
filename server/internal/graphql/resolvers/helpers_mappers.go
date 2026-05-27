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
	balance := model.NewMoney(u.Balance)
	monthlyBudget := model.NewMoney(u.MonthlyBudgetUSD)
	return &model.User{
		ID: u.ID.String(), Email: u.Email, Name: u.Name,
		Role: u.Role, IsActive: u.IsActive, Balance: &balance,
		MonthlyBudgetUsd:      &monthlyBudget,
		EmailVerified:         u.EmailVerified,
		RequirePasswordChange: u.RequirePasswordChange,
		MfaEnabled:            u.MfaEnabled,
		CreatedAt:             u.CreatedAt,
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
		AllowedModels: apiKeyPolicyList(k.AllowedModels), AllowedProviders: apiKeyPolicyList(k.AllowedProviders),
		LastUsedAt: lastUsed, ExpiresAt: expires, CreatedAt: k.CreatedAt,
	}
}

func apiKeyPolicyList(values []string) []string {
	normalized := models.NormalizeAPIKeyPolicyList(values)
	if normalized == nil {
		return []string{}
	}
	return normalized
}

func orgToGQL(o *models.Organization) *model.Organization {
	return &model.Organization{
		ID:           o.ID.String(),
		Name:         o.Name,
		BillingLimit: model.NewMoney(o.BillingLimit),
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
		QuotaLimit:     model.NewMoney(p.QuotaLimit),
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
	createdAt := p.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return &model.Provider{
		ID: p.ID.String(), Name: p.Name, BaseURL: p.BaseURL,
		IsActive: p.IsActive, Priority: p.Priority, Weight: p.Weight,
		MaxRetries: p.MaxRetries, Timeout: p.Timeout,
		UseProxy: p.UseProxy, DefaultProxyID: proxyID,
		RequiresAPIKey: p.RequiresAPIKey,
		CreatedAt:      createdAt,
	}
}

func modelToGQL(m *models.Model) *model.Model {
	pricePerSecond := model.NewMoney(m.PricePerSecond)
	pricePerImage := model.NewMoney(m.PricePerImage)
	pricePerMinute := model.NewMoney(m.PricePerMinute)
	providerCostPerSecond := model.NewMoney(m.ProviderCostPerSecond)
	providerCostPerImage := model.NewMoney(m.ProviderCostPerImage)
	providerCostPerMinute := model.NewMoney(m.ProviderCostPerMinute)

	// Context window vs max_tokens (audit M-07): if context_window has
	// been populated by a recent sync, prefer that. Otherwise fall back
	// to the legacy max_tokens column so old rows keep working until the
	// next syncProviderModels call refreshes them.
	contextWindow := m.ContextWindow
	if contextWindow == 0 {
		contextWindow = m.MaxTokens
	}

	// Pass MaxOutputTokens through as a pointer so clients can
	// distinguish "no cap reported" (null) from "cap is 0" (which would
	// be nonsensical, hence treated as null too).
	var maxOutputTokens *int
	if m.MaxOutputTokens != nil && *m.MaxOutputTokens > 0 {
		v := *m.MaxOutputTokens
		maxOutputTokens = &v
	}

	return &model.Model{
		ID:                      m.ID.String(),
		ProviderID:              m.ProviderID.String(),
		Name:                    m.Name,
		DisplayName:             m.DisplayName,
		ModelKind:               domainKindToGQL(m.ModelKind),
		InputPricePer1k:         model.NewMoney(m.InputPricePer1K),
		OutputPricePer1k:        model.NewMoney(m.OutputPricePer1K),
		PricePerSecond:          &pricePerSecond,
		PricePerImage:           &pricePerImage,
		PricePerMinute:          &pricePerMinute,
		ProviderInputCostPer1k:  model.NewMoney(m.ProviderInputCostPer1K),
		ProviderOutputCostPer1k: model.NewMoney(m.ProviderOutputCostPer1K),
		ProviderCostPerSecond:   &providerCostPerSecond,
		ProviderCostPerImage:    &providerCostPerImage,
		ProviderCostPerMinute:   &providerCostPerMinute,
		ContextWindow:           contextWindow,
		MaxOutputTokens:         maxOutputTokens,
		MaxTokens:               m.MaxTokens,
		CatalogWarnings:         m.CatalogWarnings,
		IsActive:                m.IsActive,
		CreatedAt:               m.CreatedAt,
	}
}

// domainKindToGQL converts the lowercase domain ModelKind (persisted to
// the DB) to the uppercase GraphQL enum value.
func domainKindToGQL(k models.ModelKind) model.ModelKind {
	switch k {
	case models.ModelKindChat:
		return model.ModelKindChat
	case models.ModelKindEmbedding:
		return model.ModelKindEmbedding
	case models.ModelKindImage:
		return model.ModelKindImage
	case models.ModelKindSTT:
		return model.ModelKindStt
	case models.ModelKindTTS:
		return model.ModelKindTts
	case models.ModelKindRerank:
		return model.ModelKindRerank
	case models.ModelKindUnknown:
		return model.ModelKindUnknown
	default:
		return model.ModelKindUnknown
	}
}

// gqlKindToDomain converts a GraphQL ModelKind back to the lowercase
// domain string persisted to the DB. Used by the CreateModel /
// UpdateModel mutations when an admin manually sets the kind.
func gqlKindToDomain(k *model.ModelKind) models.ModelKind {
	if k == nil {
		return ""
	}
	switch *k {
	case model.ModelKindChat:
		return models.ModelKindChat
	case model.ModelKindEmbedding:
		return models.ModelKindEmbedding
	case model.ModelKindImage:
		return models.ModelKindImage
	case model.ModelKindStt:
		return models.ModelKindSTT
	case model.ModelKindTts:
		return models.ModelKindTTS
	case model.ModelKindRerank:
		return models.ModelKindRerank
	case model.ModelKindUnknown:
		return models.ModelKindUnknown
	default:
		return models.ModelKindUnknown
	}
}

func providerAPIKeyToGQL(k *models.ProviderAPIKey) *model.ProviderAPIKey {
	var proxyID *string
	if k.ProxyID != nil {
		s := k.ProxyID.String()
		proxyID = &s
	}
	var proxyPoolID *string
	if k.ProxyPoolID != nil {
		s := k.ProxyPoolID.String()
		proxyPoolID = &s
	}
	var lastUsed *time.Time
	if !k.LastUsedAt.IsZero() {
		lastUsed = &k.LastUsedAt
	}
	return &model.ProviderAPIKey{
		ID: k.ID.String(), ProviderID: k.ProviderID.String(),
		ProxyID: proxyID, ProxyPoolID: proxyPoolID,
		Alias: k.Alias, KeyPrefix: k.KeyPrefix,
		IsActive: k.IsActive, Priority: k.Priority,
		Weight: k.Weight, RateLimit: k.RateLimit,
		UsageCount: safeGQLInt(k.UsageCount),
		LastUsedAt: lastUsed, CreatedAt: k.CreatedAt,
	}
}

func proxyToGQL(p *models.Proxy) *model.Proxy {
	var poolID *string
	if p.PoolID != nil {
		s := p.PoolID.String()
		poolID = &s
	}
	var poolName *string
	if p.Pool != nil && p.Pool.Name != "" {
		poolName = &p.Pool.Name
	}
	var upID *string
	if p.UpstreamProxyID != nil {
		s := p.UpstreamProxyID.String()
		upID = &s
	}
	var lastChecked *time.Time
	if !p.LastChecked.IsZero() {
		lastChecked = &p.LastChecked
	}
	return &model.Proxy{
		ID: p.ID.String(), PoolID: poolID, PoolName: poolName,
		URL: p.URL, Type: p.Type,
		Region: p.Region, IsActive: p.IsActive,
		Weight: p.Weight, SuccessCount: safeGQLInt(p.SuccessCount),
		FailureCount: safeGQLInt(p.FailureCount), AvgLatency: p.AvgLatency,
		LastChecked: lastChecked, HasAuth: p.HasAuth(),
		UpstreamProxyID: upID, CreatedAt: p.CreatedAt,
	}
}

func proxyPoolToGQL(p *models.ProxyPool) *model.ProxyPool {
	active := 0
	for i := range p.Proxies {
		if p.Proxies[i].IsActive {
			active++
		}
	}
	return &model.ProxyPool{
		ID: p.ID.String(), Name: p.Name, Description: p.Description,
		IsActive: p.IsActive, Strategy: p.Strategy,
		ProxyCount: len(p.Proxies), ActiveProxyCount: active,
		CreatedAt: p.CreatedAt,
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
	status := s.Status
	if status == "" {
		status = "disconnected"
	}
	var lastError *string
	if s.LastError != "" {
		lastError = &s.LastError
	}
	var lastCheckedAt *time.Time
	if !s.LastCheckedAt.IsZero() {
		lastCheckedAt = &s.LastCheckedAt
	}
	tools := make([]*model.McpTool, len(s.Tools))
	for i := range s.Tools {
		tools[i] = mcpToolToGQL(&s.Tools[i])
	}
	return &model.McpServer{
		ID: s.ID.String(), Name: s.Name, Type: s.Type,
		Command: &s.Command, URL: &s.URL,
		Args: args, IsActive: s.IsActive,
		Status: status, LastError: lastError,
		LastCheckedAt: lastCheckedAt, Tools: tools,
		CreatedAt: s.CreatedAt,
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
		InputSchema: schema, IsActive: t.IsActive,
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
		MonthlyLimitUsd: model.NewMoney(b.MonthlyLimitUSD), AlertThreshold: b.AlertThreshold,
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
		Type: c.Type, DiscountValue: model.NewMoney(c.DiscountValue),
		MinAmount: model.NewMoney(c.MinAmount), MaxUses: c.MaxUses,
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
	if v, ok := all["captcha"]; ok {
		s.Captcha = &v
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
