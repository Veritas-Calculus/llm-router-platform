package resolvers

import (
	"testing"

	"llm-router-platform/internal/graphql/errs"
)

// Audit L-06: API key Name must reject HTML angle brackets, quotes, and
// other characters that could become an XSS / CSV-injection vector in
// downstream renderers.
func TestValidateAPIKeyName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Happy paths
		{"plain", "production key", false},
		{"with digits", "Audit key 2026", false},
		{"with dots and dashes", "team-alpha.production_v2", false},
		{"with halfwidth parens", "Backup (rotated)", false},
		{"with fullwidth parens", "审计密钥（生产）", false},
		{"unicode letters", "日本語キー", false},
		{"max length", string(makeRunes(64, 'A')), false},

		// Rejections
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"over 64 runes", string(makeRunes(65, 'A')), true},
		{"contains script tag", "<script>alert(1)</script>", true},
		{"contains lt", "evil<", true},
		{"contains gt", "evil>", true},
		{"contains quote", "evil\"", true},
		{"contains pipe", "evil|injection", true},
		{"contains semicolon", "evil; rm -rf", true},
		{"contains formula", "=SUM(A1)", true},
		{"contains slash", "evil/path", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateAPIKeyName(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("validateAPIKeyName(%q) accepted, want VALIDATION error", tc.input)
				}
				vErr, ok := errs.AsValidation(err)
				if !ok || vErr.Field != "name" {
					t.Fatalf("validateAPIKeyName(%q) returned %v; want ValidationError with field=name", tc.input, err)
				}
			} else if err != nil {
				t.Fatalf("validateAPIKeyName(%q) rejected with %v, want pass", tc.input, err)
			}
		})
	}
}

func makeRunes(n int, r rune) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return out
}
