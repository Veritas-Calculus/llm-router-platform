package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock repositories ---

type mockProviderRepo struct {
	providers []models.Provider
	err       error
}

func (m *mockProviderRepo) Create(_ context.Context, _ *models.Provider) error { return nil }
func (m *mockProviderRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Provider, error) {
	for i := range m.providers {
		if m.providers[i].ID == id {
			return &m.providers[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockProviderRepo) GetByName(_ context.Context, name string) (*models.Provider, error) {
	for i := range m.providers {
		if m.providers[i].Name == name {
			return &m.providers[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockProviderRepo) GetActive(_ context.Context) ([]models.Provider, error) {
	if m.err != nil {
		return nil, m.err
	}
	var active []models.Provider
	for _, p := range m.providers {
		if p.IsActive {
			active = append(active, p)
		}
	}
	return active, nil
}
func (m *mockProviderRepo) GetAll(_ context.Context) ([]models.Provider, error) {
	return m.providers, m.err
}
func (m *mockProviderRepo) Update(_ context.Context, _ *models.Provider) error { return nil }
func (m *mockProviderRepo) Delete(_ context.Context, _ uuid.UUID) error        { return nil }

type mockProviderAPIKeyRepo struct {
	keys map[uuid.UUID][]models.ProviderAPIKey // providerID -> keys
	err  error
}

func (m *mockProviderAPIKeyRepo) Create(_ context.Context, _ *models.ProviderAPIKey) error {
	return nil
}
func (m *mockProviderAPIKeyRepo) GetActiveByProvider(_ context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	var active []models.ProviderAPIKey
	for _, k := range m.keys[providerID] {
		if k.IsActive {
			active = append(active, k)
		}
	}
	return active, nil
}
func (m *mockProviderAPIKeyRepo) GetByProvider(_ context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error) {
	return m.keys[providerID], m.err
}
func (m *mockProviderAPIKeyRepo) GetByID(_ context.Context, id uuid.UUID) (*models.ProviderAPIKey, error) {
	for _, keys := range m.keys {
		for i := range keys {
			if keys[i].ID == id {
				return &keys[i], nil
			}
		}
	}
	return nil, errors.New("not found")
}
func (m *mockProviderAPIKeyRepo) GetAll(_ context.Context) ([]models.ProviderAPIKey, error) {
	var all []models.ProviderAPIKey
	for _, keys := range m.keys {
		all = append(all, keys...)
	}
	return all, nil
}
func (m *mockProviderAPIKeyRepo) Update(_ context.Context, _ *models.ProviderAPIKey) error {
	return nil
}
func (m *mockProviderAPIKeyRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

type mockProxyRepo struct{}

func (m *mockProxyRepo) Create(_ context.Context, _ *models.Proxy) error { return nil }
func (m *mockProxyRepo) GetByID(_ context.Context, _ uuid.UUID) (*models.Proxy, error) {
	return nil, errors.New("not found")
}
func (m *mockProxyRepo) GetByPoolID(_ context.Context, _ uuid.UUID) ([]models.Proxy, error) {
	return nil, nil
}
func (m *mockProxyRepo) GetActiveByPoolID(_ context.Context, _ uuid.UUID) ([]models.Proxy, error) {
	return nil, nil
}
func (m *mockProxyRepo) GetActive(_ context.Context) ([]models.Proxy, error) { return nil, nil }
func (m *mockProxyRepo) GetAll(_ context.Context) ([]models.Proxy, error)    { return nil, nil }
func (m *mockProxyRepo) Update(_ context.Context, _ *models.Proxy) error     { return nil }
func (m *mockProxyRepo) Delete(_ context.Context, _ uuid.UUID) error         { return nil }
func (m *mockProxyRepo) CreatePool(_ context.Context, _ *models.ProxyPool) error {
	return nil
}
func (m *mockProxyRepo) GetPools(_ context.Context) ([]models.ProxyPool, error) { return nil, nil }
func (m *mockProxyRepo) GetPoolByID(_ context.Context, _ uuid.UUID) (*models.ProxyPool, error) {
	return nil, errors.New("not found")
}
func (m *mockProxyRepo) UpdatePool(_ context.Context, _ *models.ProxyPool) error { return nil }
func (m *mockProxyRepo) DeletePool(_ context.Context, _ uuid.UUID) error         { return nil }

type mockModelRepo struct {
	models map[uuid.UUID][]models.Model // providerID -> models
}

func (m *mockModelRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Model, error) {
	for _, mods := range m.models {
		for i := range mods {
			if mods[i].ID == id {
				return &mods[i], nil
			}
		}
	}
	return nil, errors.New("not found")
}
func (m *mockModelRepo) GetByName(_ context.Context, name string) (*models.Model, error) {
	for _, mods := range m.models {
		for i := range mods {
			if mods[i].Name == name {
				return &mods[i], nil
			}
		}
	}
	return nil, errors.New("not found")
}
func (m *mockModelRepo) GetByProviderAndName(_ context.Context, providerID uuid.UUID, name string) (*models.Model, error) {
	for i := range m.models[providerID] {
		if m.models[providerID][i].Name == name {
			return &m.models[providerID][i], nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockModelRepo) GetByProvider(_ context.Context, providerID uuid.UUID) ([]models.Model, error) {
	return m.models[providerID], nil
}
func (m *mockModelRepo) GetByProviderSorted(_ context.Context, providerID uuid.UUID) ([]models.Model, error) {
	return m.models[providerID], nil
}
func (m *mockModelRepo) Create(_ context.Context, _ *models.Model) error { return nil }
func (m *mockModelRepo) Update(_ context.Context, _ *models.Model) error { return nil }
func (m *mockModelRepo) Delete(_ context.Context, _ uuid.UUID) error     { return nil }

type mockRoutingRuleRepo struct {
	rules []models.RoutingRule
}

func (m *mockRoutingRuleRepo) Create(_ context.Context, _ *models.RoutingRule) error { return nil }
func (m *mockRoutingRuleRepo) GetByID(_ context.Context, _ uuid.UUID) (*models.RoutingRule, error) {
	return nil, errors.New("not found")
}
func (m *mockRoutingRuleRepo) GetAll(_ context.Context) ([]models.RoutingRule, error) {
	return m.rules, nil
}
func (m *mockRoutingRuleRepo) GetActive(_ context.Context) ([]models.RoutingRule, error) {
	var active []models.RoutingRule
	for _, r := range m.rules {
		if r.IsEnabled {
			active = append(active, r)
		}
	}
	return active, nil
}
func (m *mockRoutingRuleRepo) Update(_ context.Context, _ *models.RoutingRule) error { return nil }
func (m *mockRoutingRuleRepo) Delete(_ context.Context, _ uuid.UUID) error           { return nil }

// --- Helper to create a test router ---

func newTestRouter(providerRepo *mockProviderRepo, keyRepo *mockProviderAPIKeyRepo) *Router {
	if keyRepo == nil {
		keyRepo = &mockProviderAPIKeyRepo{keys: make(map[uuid.UUID][]models.ProviderAPIKey)}
	}
	logger, _ := zap.NewDevelopment()
	return NewRouter(
		providerRepo,
		keyRepo,
		&mockProxyRepo{},
		&mockModelRepo{models: make(map[uuid.UUID][]models.Model)},
		&mockRoutingRuleRepo{rules: []models.RoutingRule{}},
		provider.NewRegistry(logger),
		nil, // mcpService
		logger,
	)
}

// --- Tests ---

func TestRoute_NoProviders(t *testing.T) {
	r := newTestRouter(&mockProviderRepo{}, nil)
	_, _, err := r.Route(context.Background(), "gpt-4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active providers")
}

func TestRoute_NoProviders_DBError(t *testing.T) {
	r := newTestRouter(&mockProviderRepo{err: errors.New("db down")}, nil)
	_, _, err := r.Route(context.Background(), "gpt-4")
	require.Error(t, err)
}

func TestRoute_SingleProvider_NoAPIKey(t *testing.T) {
	pid := uuid.New()
	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "ollama", IsActive: true, RequiresAPIKey: false},
		},
	}
	repo.providers[0].ID = pid

	r := newTestRouter(repo, nil)
	p, key, err := r.Route(context.Background(), "llama3")
	require.NoError(t, err)
	assert.Equal(t, "ollama", p.Name)
	assert.Nil(t, key) // No API key needed
}

func TestRoute_SingleProvider_WithAPIKey(t *testing.T) {
	pid := uuid.New()
	kid := uuid.New()
	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "openai", IsActive: true, RequiresAPIKey: true},
		},
	}
	repo.providers[0].ID = pid

	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			pid: {
				{ProviderID: pid, IsActive: true, Priority: 1, Weight: 1.0, Alias: "key1"},
			},
		},
	}
	keyRepo.keys[pid][0].ID = kid

	r := newTestRouter(repo, keyRepo)
	p, key, err := r.Route(context.Background(), "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name)
	require.NotNil(t, key)
	assert.Equal(t, kid, key.ID)
}

func TestRouteForAPIKey_EnforcesModelAndProviderPolicy(t *testing.T) {
	openAIID := uuid.New()
	anthropicID := uuid.New()
	repo := &mockProviderRepo{
		providers: []models.Provider{
			{BaseModel: models.BaseModel{ID: openAIID}, Name: "openai", IsActive: true, RequiresAPIKey: true, Weight: 1},
			{BaseModel: models.BaseModel{ID: anthropicID}, Name: "anthropic", IsActive: true, RequiresAPIKey: true, Weight: 1},
		},
	}
	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			openAIID:    {{BaseModel: models.BaseModel{ID: uuid.New()}, ProviderID: openAIID, IsActive: true, Priority: 1, Weight: 1}},
			anthropicID: {{BaseModel: models.BaseModel{ID: uuid.New()}, ProviderID: anthropicID, IsActive: true, Priority: 1, Weight: 1}},
		},
	}

	r := newTestRouter(repo, keyRepo)
	callerKey := &models.APIKey{
		AllowedModels:    []string{"gpt-*"},
		AllowedProviders: []string{"openai"},
	}

	p, key, err := r.RouteForAPIKey(context.Background(), "gpt-4o", callerKey)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NotNil(t, key)
	assert.Equal(t, "openai", p.Name)

	_, _, err = r.RouteForAPIKey(context.Background(), "claude-3-opus", callerKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model is not allowed")

	callerKey.AllowedProviders = []string{"missing-provider"}
	_, _, err = r.RouteForAPIKey(context.Background(), "gpt-4o", callerKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider is not allowed")
}

func TestRoute_RequiresAPIKey_NoActiveKeys(t *testing.T) {
	pid := uuid.New()
	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "openai", IsActive: true, RequiresAPIKey: true},
		},
	}
	repo.providers[0].ID = pid

	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			pid: {
				{ProviderID: pid, IsActive: false, Priority: 1, Weight: 1.0, Alias: "disabled"},
			},
		},
	}

	r := newTestRouter(repo, keyRepo)
	_, _, err := r.Route(context.Background(), "gpt-4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active API keys")
}

func TestSelectAPIKey_Priority(t *testing.T) {
	pid := uuid.New()
	kid1 := uuid.New()
	kid2 := uuid.New()
	kid3 := uuid.New()

	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			pid: {
				{ProviderID: pid, IsActive: true, Priority: 2, Weight: 1.0, Alias: "low-prio"},
				{ProviderID: pid, IsActive: true, Priority: 1, Weight: 1.0, Alias: "high-prio"},
				{ProviderID: pid, IsActive: true, Priority: 3, Weight: 1.0, Alias: "lowest-prio"},
			},
		},
	}
	keyRepo.keys[pid][0].ID = kid1
	keyRepo.keys[pid][1].ID = kid2
	keyRepo.keys[pid][2].ID = kid3

	r := newTestRouter(&mockProviderRepo{}, keyRepo)

	key, err := r.selectAPIKey(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, key)
	// Should pick the highest priority (lowest number)
	assert.Equal(t, kid2, key.ID)
}

func TestSelectNextAPIKey_ExcludesCurrent(t *testing.T) {
	pid := uuid.New()
	kid1 := uuid.New()
	kid2 := uuid.New()

	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			pid: {
				{ProviderID: pid, IsActive: true, Priority: 1, Weight: 1.0, Alias: "key1"},
				{ProviderID: pid, IsActive: true, Priority: 1, Weight: 1.0, Alias: "key2"},
			},
		},
	}
	keyRepo.keys[pid][0].ID = kid1
	keyRepo.keys[pid][1].ID = kid2

	r := newTestRouter(&mockProviderRepo{}, keyRepo)

	// Excluding kid1 should return kid2
	key, err := r.SelectNextAPIKey(context.Background(), pid, kid1)
	require.NoError(t, err)
	require.NotNil(t, key)
	assert.Equal(t, kid2, key.ID)

	// Excluding kid2 should return kid1
	key, err = r.SelectNextAPIKey(context.Background(), pid, kid2)
	require.NoError(t, err)
	require.NotNil(t, key)
	assert.Equal(t, kid1, key.ID)
}

func TestSelectNextAPIKey_SingleKey_NoAlternative(t *testing.T) {
	pid := uuid.New()
	kid := uuid.New()

	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			pid: {
				{ProviderID: pid, IsActive: true, Priority: 1, Weight: 1.0, Alias: "only-key"},
			},
		},
	}
	keyRepo.keys[pid][0].ID = kid

	r := newTestRouter(&mockProviderRepo{}, keyRepo)

	key, err := r.SelectNextAPIKey(context.Background(), pid, kid)
	require.Error(t, err)
	assert.Nil(t, key)
}

func TestMarkKeyFailed_InMemory(t *testing.T) {
	r := newTestRouter(&mockProviderRepo{}, nil)
	kid := uuid.New()

	r.MarkKeyFailed(kid, "quota exceeded")

	r.failedKeysMu.RLock()
	info, exists := r.failedKeys[kid]
	r.failedKeysMu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, "quota exceeded", info.Reason)
}

func TestClearKeyFailure_InMemory(t *testing.T) {
	r := newTestRouter(&mockProviderRepo{}, nil)
	kid := uuid.New()

	r.MarkKeyFailed(kid, "quota exceeded")
	r.ClearKeyFailure(kid)

	r.failedKeysMu.RLock()
	_, exists := r.failedKeys[kid]
	r.failedKeysMu.RUnlock()

	assert.False(t, exists)
}

func TestSetStrategy(t *testing.T) {
	r := newTestRouter(&mockProviderRepo{}, nil)
	assert.Equal(t, StrategyWeighted, r.strategy)

	r.SetStrategy(StrategyLeastLatency)
	assert.Equal(t, StrategyLeastLatency, r.strategy)
}

func TestRoute_MultipleProviders_WeightedStrategy(t *testing.T) {
	pid1 := uuid.New()
	pid2 := uuid.New()

	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "openai", IsActive: true, RequiresAPIKey: false, Priority: 10, Weight: 0.7},
			{Name: "anthropic", IsActive: true, RequiresAPIKey: false, Priority: 10, Weight: 0.3},
		},
	}
	repo.providers[0].ID = pid1
	repo.providers[1].ID = pid2

	r := newTestRouter(repo, nil)

	// Route many times; both providers should be selected
	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		p, _, err := r.Route(context.Background(), "some-model")
		require.NoError(t, err)
		counts[p.Name]++
	}

	assert.Greater(t, counts["openai"], 0, "openai should be selected at least once")
	assert.Greater(t, counts["anthropic"], 0, "anthropic should be selected at least once")
	// With 70/30 weights, openai should be selected more often
	assert.Greater(t, counts["openai"], counts["anthropic"],
		"openai (weight 0.7) should be selected more than anthropic (weight 0.3)")
}

func TestRouteWithFallback_PicksHighestPriority(t *testing.T) {
	pid1 := uuid.New()
	pid2 := uuid.New()
	kid1 := uuid.New()
	kid2 := uuid.New()

	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "openai", IsActive: true, RequiresAPIKey: true, Priority: 20, Weight: 1.0},
			{Name: "anthropic", IsActive: true, RequiresAPIKey: true, Priority: 10, Weight: 1.0},
		},
	}
	repo.providers[0].ID = pid1
	repo.providers[1].ID = pid2

	keyRepo := &mockProviderAPIKeyRepo{
		keys: map[uuid.UUID][]models.ProviderAPIKey{
			pid1: {{ProviderID: pid1, IsActive: true, Priority: 1, Weight: 1.0, Alias: "key1"}},
			pid2: {{ProviderID: pid2, IsActive: true, Priority: 1, Weight: 1.0, Alias: "key2"}},
		},
	}
	keyRepo.keys[pid1][0].ID = kid1
	keyRepo.keys[pid2][0].ID = kid2

	r := newTestRouter(repo, keyRepo)

	// RouteWithFallback sorts by priority desc, so should always pick "openai" (priority 20)
	for i := 0; i < 10; i++ {
		p, key, err := r.RouteWithFallback(context.Background(), "gpt-4", 3)
		require.NoError(t, err)
		assert.Equal(t, "openai", p.Name)
		assert.NotNil(t, key)
	}
}

func TestIsQuotaOrRateLimitError(t *testing.T) {
	tests := []struct {
		msg    string
		expect bool
	}{
		{"quota exceeded", true},
		{"rate limit hit", true},
		{"429 Too Many Requests", true},
		{"insufficient_quota", true},
		{"resource exhausted", true},
		{"billing issue detected", true},
		{"connection refused", false},
		{"timeout", false},
		{"internal server error", false},
	}
	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			assert.Equal(t, tc.expect, messageLooksQuotaLimited(tc.msg))
		})
	}
}

func TestIsQuotaOrRateLimitErrorTyped(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{"429 from typed provider error", &provider.ProviderError{StatusCode: 429, Message: "Too Many Requests"}, true},
		{"402 quota exhausted typed", &provider.ProviderError{StatusCode: 402, Message: "Payment Required"}, true},
		{"403 with quota body → rotate", &provider.ProviderError{StatusCode: 403, Body: []byte(`{"error":"quota exceeded"}`)}, true},
		{"403 plain auth failure → don't rotate", &provider.ProviderError{StatusCode: 403, Message: "Forbidden"}, false},
		{"500 typed → not quota", &provider.ProviderError{StatusCode: 500, Message: "Internal Server Error"}, false},
		{"nil error", nil, false},
		{"plain error with 'billing in metadata'", fmt.Errorf("billing portal misconfigured"), true},
		{"plain network error", fmt.Errorf("dial tcp: i/o timeout"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, isQuotaOrRateLimitError(tc.err))
		})
	}
}

func TestIsProviderLevelErrorTyped(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{"500 typed", &provider.ProviderError{StatusCode: 500}, true},
		{"502 typed", &provider.ProviderError{StatusCode: 502}, true},
		{"504 typed", &provider.ProviderError{StatusCode: 504}, true},
		{"429 typed → not provider-level", &provider.ProviderError{StatusCode: 429}, false},
		{"400 typed → not provider-level", &provider.ProviderError{StatusCode: 400}, false},
		{"nil error", nil, false},
		{"plain error containing 'timeout'", fmt.Errorf("context deadline exceeded"), true},
		{"plain quota-only error", fmt.Errorf("billing portal misconfigured"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, isProviderLevelError(tc.err))
		})
	}
}

func TestMatchesGlobPattern(t *testing.T) {
	tests := []struct {
		model   string
		pattern string
		match   bool
	}{
		{"gpt-4", "gpt-*", true},
		{"gpt-4-turbo", "gpt-*", true},
		{"o1-preview", "o1*", true},
		{"claude-3-opus", "claude*", true},
		{"llama3", "llama*", true},
		{"gpt-4", "claude*", false},
		{"dall-e-3", "dall-e*", true},
		{"whisper-1", "whisper*", true},
		{"deepseek-v3", "deepseek*", true},
		{"some-model", "*", true},
		{"gemini-pro", "gemini*", true},
	}
	for _, tc := range tests {
		t.Run(tc.model+"_"+tc.pattern, func(t *testing.T) {
			assert.Equal(t, tc.match, matchesGlobPattern(tc.model, tc.pattern))
		})
	}
}

func TestRoute_ModelPatterns_OverridesHeuristic(t *testing.T) {
	pid1 := uuid.New()
	pid2 := uuid.New()

	// Set up a "custom" provider with patterns that match "gpt-*"
	// This should override the hardcoded heuristic that would route to "openai"
	patterns1, _ := json.Marshal([]string{"gpt-*", "o1*"})
	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "openai", IsActive: true, RequiresAPIKey: false, Priority: 10, Weight: 1.0},
			{Name: "custom-provider", IsActive: true, RequiresAPIKey: false, Priority: 10, Weight: 1.0,
				ModelPatterns: patterns1},
		},
	}
	repo.providers[0].ID = pid1
	repo.providers[1].ID = pid2

	r := newTestRouter(repo, nil)

	// "gpt-4" should match custom-provider's patterns, NOT the hardcoded openai heuristic
	p, _, err := r.Route(context.Background(), "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, "custom-provider", p.Name)

	// "o1-mini" should also match custom-provider's patterns
	p, _, err = r.Route(context.Background(), "o1-mini")
	require.NoError(t, err)
	assert.Equal(t, "custom-provider", p.Name)
}

func TestRoute_ModelPatterns_NoMatch_FallsBackToHeuristic(t *testing.T) {
	pid1 := uuid.New()
	pid2 := uuid.New()

	// custom-provider has patterns but they don't match "claude-3-opus"
	patterns, _ := json.Marshal([]string{"gpt-*"})
	repo := &mockProviderRepo{
		providers: []models.Provider{
			{Name: "anthropic", IsActive: true, RequiresAPIKey: false, Priority: 10, Weight: 1.0},
			{Name: "custom-provider", IsActive: true, RequiresAPIKey: false, Priority: 10, Weight: 1.0,
				ModelPatterns: patterns},
		},
	}
	repo.providers[0].ID = pid1
	repo.providers[1].ID = pid2

	r := newTestRouter(repo, nil)

	// "claude-3-opus" should NOT match custom-provider's patterns
	// and should fall back to the hardcoded anthropic heuristic
	p, _, err := r.Route(context.Background(), "claude-3-opus")
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name)
}
