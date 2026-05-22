package mcp

import "testing"

func TestValidateMCPArgs(t *testing.T) {
	cases := []struct {
		name    string
		cmd     string
		args    []string
		wantErr bool
	}{
		{"node legitimate package", "node", []string{"server.js", "--port", "3000"}, false},
		{"node -e injection", "node", []string{"-e", "<injected>"}, true},
		{"node --eval injection", "node", []string{"--eval=<injected>"}, true},
		{"python -c injection", "python3", []string{"-c", "<injected>"}, true},
		{"deno eval injection via -e", "deno", []string{"-e", "<injected>"}, true},
		{"bun -e injection", "bun", []string{"-e", "<injected>"}, true},
		{"python stdin marker", "python3", []string{"-"}, true},
		{"uvx package install", "uvx", []string{"mcp-server-foo"}, false},
		{"uvx -c injection", "uvx", []string{"-c", "<injected>"}, true},

		{"docker normal", "docker", []string{"run", "--rm", "-i", "mcp/server"}, false},
		{"docker --privileged", "docker", []string{"run", "--privileged", "mcp/server"}, true},
		{"docker --network=host", "docker", []string{"run", "--network=host", "mcp/server"}, true},
		{"docker -v host mount", "docker", []string{"run", "-v", "/:/host", "mcp/server"}, true},
		{"docker --mount", "docker", []string{"run", "--mount=type=bind,src=/,dst=/host", "mcp/server"}, true},
		{"docker --cap-add", "docker", []string{"run", "--cap-add=SYS_ADMIN", "mcp/server"}, true},

		{"npx pre-approved package", "npx", []string{"@modelcontextprotocol/server-foo"}, false},
		{"empty args", "node", []string{}, false},
		{"whitespace-only arg", "node", []string{"   "}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMCPArgs(tc.cmd, tc.args)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s %v, got nil", tc.cmd, tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %s %v, got %v", tc.cmd, tc.args, err)
			}
		})
	}
}
