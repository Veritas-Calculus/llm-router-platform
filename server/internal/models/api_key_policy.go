package models

import (
	"strings"
)

// NormalizeAPIKeyPolicyList trims, de-duplicates, and drops empty policy values.
func NormalizeAPIKeyPolicyList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AllowsScope reports whether the API key may access a coarse endpoint scope.
func (k *APIKey) AllowsScope(scope string) bool {
	if k == nil {
		return false
	}
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		return true
	}
	raw := strings.TrimSpace(k.Scopes)
	if raw == "" || strings.EqualFold(raw, "all") {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "all" || value == scope {
			return true
		}
	}
	return false
}

// AllowsModel reports whether the API key may request a model. Empty policy means unrestricted.
func (k *APIKey) AllowsModel(model string) bool {
	if k == nil {
		return false
	}
	patterns := NormalizeAPIKeyPolicyList(k.AllowedModels)
	if len(patterns) == 0 {
		return true
	}
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	baseModel := model
	if idx := strings.Index(model, "/"); idx >= 0 && idx < len(model)-1 {
		baseModel = model[idx+1:]
	}
	for _, pattern := range patterns {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if wildcardMatch(p, model) || wildcardMatch(p, baseModel) {
			return true
		}
	}
	return false
}

// AllowsProvider reports whether the API key may egress to a provider. Empty policy means unrestricted.
func (k *APIKey) AllowsProvider(provider *Provider) bool {
	if k == nil || provider == nil {
		return false
	}
	allowed := NormalizeAPIKeyPolicyList(k.AllowedProviders)
	if len(allowed) == 0 {
		return true
	}
	id := strings.ToLower(provider.ID.String())
	name := strings.ToLower(strings.TrimSpace(provider.Name))
	for _, value := range allowed {
		v := strings.ToLower(strings.TrimSpace(value))
		if v == id || v == name {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	value = strings.TrimSpace(value)
	if pattern == "" || value == "" {
		return false
	}
	if pattern == value {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}
	last := parts[len(parts)-1]
	if last != "" && !strings.HasSuffix(pattern, "*") && !strings.HasSuffix(value, last) {
		return false
	}
	return true
}
