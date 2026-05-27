package resolvers

import (
	"errors"
	"testing"
)

// TestIsPasswordPolicyError pins the exact set of error prefixes the
// GraphQL ErrorPresenter is allowed to surface as VALIDATION + field=password.
// Anything outside the allowlist must fall through to INTERNAL masking — see
// post-audit P0-3 for the reasoning.
func TestIsPasswordPolicyError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		// Exact messages produced by user.ValidatePassword today. If you
		// rename one of these, update passwordPolicyErrorPrefixes too.
		{"nil error", nil, false},
		{"length rule", errors.New("password must be at least 8 characters"), true},
		{"uppercase rule", errors.New("password must contain at least one uppercase letter"), true},
		{"lowercase rule", errors.New("password must contain at least one lowercase letter"), true},
		{"digit rule", errors.New("password must contain at least one digit"), true},
		{"common-password rule", errors.New("password is too common, please choose a stronger password"), true},

		// Negative cases — these must continue to be masked. The prior
		// implementation matched the broad prefix "password " and would
		// have leaked these to the client.
		{"password reset failed (not a validation error)", errors.New("password reset failed"), false},
		{"password hash unavailable (infra error)", errors.New("password hash unavailable"), false},
		{"current password incorrect (auth error, not policy)", errors.New("current password is incorrect"), false},
		{"unrelated error", errors.New("database connection refused"), false},
		// Empty string is not a policy violation either.
		{"empty error message", errors.New(""), false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := isPasswordPolicyError(tc.err)
			if got != tc.want {
				t.Fatalf("isPasswordPolicyError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
