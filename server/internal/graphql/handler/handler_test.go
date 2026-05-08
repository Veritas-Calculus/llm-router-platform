package gqlhandler

import "testing"

func TestIsClientGraphQLError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "schema validation",
			msg:  `Cannot query field "updatedAt" on type "McpServer". Did you mean "createdAt"?`,
			want: true,
		},
		{
			name: "invalid scalar",
			msg:  "cannot use string as Int",
			want: true,
		},
		{
			name: "provider validation",
			msg:  "invalid base URL: URL resolves to private/reserved IP address",
			want: true,
		},
		{
			name: "conflict",
			msg:  `provider "lmstudio" already exists`,
			want: true,
		},
		{
			name: "nullable resolver bug remains internal",
			msg:  "the requested element is null which the schema does not allow",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClientGraphQLError(tt.msg); got != tt.want {
				t.Fatalf("isClientGraphQLError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}
