package resolvers

import (
	"net/url"
	"strings"

	"llm-router-platform/internal/graphql/errs"
	"llm-router-platform/internal/graphql/model"
	gdbModels "llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"
)

// validateWebhookURLOrFail wraps sanitize.ValidateWebhookURL so that
// failures surface to the GraphQL client as a typed VALIDATION error
// with extensions.field = "url". The audit (H-05) found that the
// service-layer fmt.Errorf wrappers were getting masked to
// `internal error [request_id]` in the error presenter, leaving the
// webhook dialog silent.
//
// We pre-classify each known failure mode into a user-readable string
// rather than passing through the helper's internal message verbatim —
// the helper's messages are appropriate for ops logs but reveal more
// than the webhook form should expose (e.g. raw scheme strings).
//
// SECURITY: this function does NOT loosen any rule. It re-runs the
// exact same checks the service layer would and short-circuits before
// the service is invoked, so the SSRF guard at the dial layer is
// untouched. The only thing that changes is the wire format of the
// error.
func validateWebhookURLOrFail(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errs.Validation("url", "Webhook URL is required")
	}

	// Cheap up-front parse so we can disambiguate "non-HTTPS scheme" from
	// "malformed URL" without depending on the helper's error text.
	parsed, parseErr := url.Parse(raw)
	if parseErr != nil || parsed == nil || parsed.Host == "" {
		return errs.Validation("url", "Webhook URL is not valid")
	}
	switch parsed.Scheme {
	case "https":
		// allowed
	case "http":
		// http:// is rejected because allowHTTP=false in production. We
		// surface a specific message so the user knows to flip to https.
		return errs.Validation("url", "Webhook URL must use https:// scheme")
	default:
		return errs.Validation("url", "Webhook URL must use https:// scheme")
	}

	// Defer everything else (hostname presence, DNS, private-IP) to the
	// shared sanitize helper so we don't drift from the dial-time guard.
	// allowHTTP=false because the GraphQL surface is the public API; the
	// service layer also passes false. allowLocal=false: webhook targets
	// are user-owned URLs and must be publicly reachable.
	if vErr := sanitize.ValidateWebhookURL(raw, false, false); vErr != nil {
		msg := vErr.Error()
		// Map the helper's known messages onto user-facing strings.
		// Keep the catch-all generic to avoid leaking implementation
		// details for unexpected branches.
		switch {
		case strings.Contains(msg, "must use HTTPS"),
			strings.Contains(msg, "is not allowed"):
			return errs.Validation("url", "Webhook URL must use https:// scheme")
		case strings.Contains(msg, "must have a hostname"):
			return errs.Validation("url", "Webhook URL is not valid")
		case strings.Contains(msg, "cannot resolve"):
			return errs.Validation("url", "Webhook URL must be reachable")
		case strings.Contains(msg, "private/reserved IP"):
			return errs.Validation("url", "Webhook URL must resolve to a public address")
		default:
			return errs.Validation("url", "Webhook URL is not valid")
		}
	}
	return nil
}

func mapWebhookEndpoint(m *gdbModels.WebhookEndpoint, includeSecret bool) *model.WebhookEndpoint {
	if m == nil {
		return nil
	}
	var secret *string
	if includeSecret {
		s := m.Secret
		secret = &s
	}
	var desc *string
	if m.Description != "" {
		s := m.Description
		desc = &s
	}
	return &model.WebhookEndpoint{
		ID:          m.ID.String(),
		ProjectID:   m.ProjectID.String(),
		URL:         m.URL,
		Secret:      secret,
		Events:      m.Events,
		IsActive:    m.IsActive,
		Description: desc,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func mapWebhookDelivery(m *gdbModels.WebhookDelivery) *model.WebhookDelivery {
	if m == nil {
		return nil
	}
	var resp *string
	if m.ResponseBody != "" {
		s := m.ResponseBody
		resp = &s
	}
	var errMsg *string
	if m.ErrorMessage != "" {
		s := m.ErrorMessage
		errMsg = &s
	}
	return &model.WebhookDelivery{
		ID:           m.ID.String(),
		EndpointID:   m.EndpointID.String(),
		EventType:    m.EventType,
		Payload:      m.Payload,
		StatusCode:   m.StatusCode,
		ResponseBody: resp,
		ErrorMessage: errMsg,
		CreatedAt:    m.CreatedAt,
	}
}
