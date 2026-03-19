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
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"role": u.Role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(r.Config.JWT.Secret))
}

func (r *mutationResolver) generateRefreshJWT(u *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"type": "refresh",
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(r.Config.JWT.Secret))
}

func (r *mutationResolver) validateRefreshJWT(tokenStr string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(r.Config.JWT.Secret), nil
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
		CreatedAt: u.CreatedAt,
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
		ID: k.ID.String(), Name: k.Name, KeyPrefix: k.KeyPrefix,
		IsActive: k.IsActive, LastUsedAt: lastUsed,
		ExpiresAt: expires, CreatedAt: k.CreatedAt,
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
		CreatedAt: p.CreatedAt,
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
		ID: t.ID.String(), UserID: t.UserID.String(),
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
		ID: b.ID.String(), UserID: b.UserID.String(),
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
	return s
}
