package resolvers

import (
	"errors"
	"testing"

	"llm-router-platform/internal/graphql/errs"
)

// TestValidateWebhookURLOrFailClassification covers the cases the
// audit (H-05) cited:
//   - empty / blank URL is a "url" VALIDATION
//   - non-https scheme is a "url" VALIDATION with the scheme-specific
//     message
//   - loopback literal is a "url" VALIDATION with the public-address
//     message
//
// The helper bypasses DNS for the loopback literal case because the
// underlying sanitize.ValidateWebhookURL still does a LookupIP for
// 127.0.0.1, which resolves to itself (a private IP) — so the test
// remains hermetic.
func TestValidateWebhookURLOrFailClassification(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantMsg  string
		wantFld  string
	}{
		{
			name:    "empty",
			raw:     "",
			wantMsg: "Webhook URL is required",
			wantFld: "url",
		},
		{
			name:    "blank whitespace",
			raw:     "   ",
			wantMsg: "Webhook URL is required",
			wantFld: "url",
		},
		{
			name:    "ftp scheme",
			raw:     "ftp://example.com/foo",
			wantMsg: "Webhook URL must use https:// scheme",
			wantFld: "url",
		},
		{
			name:    "plain http",
			raw:     "http://example.com/webhook",
			wantMsg: "Webhook URL must use https:// scheme",
			wantFld: "url",
		},
		{
			name:    "loopback literal",
			raw:     "https://127.0.0.1/cb",
			wantMsg: "Webhook URL must resolve to a public address",
			wantFld: "url",
		},
		{
			name:    "no host",
			raw:     "https://",
			wantMsg: "Webhook URL is not valid",
			wantFld: "url",
		},
		{
			name:    "garbage",
			raw:     "::not-a-url::",
			wantMsg: "Webhook URL is not valid",
			wantFld: "url",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebhookURLOrFail(tc.raw)
			if err == nil {
				t.Fatalf("want error, got nil")
			}
			var ve *errs.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want *errs.ValidationError, got %T (%v)", err, err)
			}
			if ve.Field != tc.wantFld {
				t.Errorf("field = %q, want %q", ve.Field, tc.wantFld)
			}
			if ve.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", ve.Message, tc.wantMsg)
			}
		})
	}
}

// TestValidateWebhookURLOrFailAcceptsPublicHTTPS is a regression that
// the happy path still returns nil. We avoid hitting the network by
// using a known public domain — the helper requires DNS resolution,
// so this test reaches the real resolver. Skipped if the host runs
// in a fully offline environment.
func TestValidateWebhookURLOrFailAcceptsPublicHTTPS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network to resolve example.com")
	}
	if err := validateWebhookURLOrFail("https://example.com/webhook"); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
