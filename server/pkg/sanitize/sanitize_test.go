package sanitize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogValueStripsControlChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"newline", "line1\nline2", "line1\\nline2"},
		{"carriage return", "line1\rline2", "line1\\rline2"},
		{"tab", "line1\tline2", "line1\\tline2"},
		{"null byte", "before\x00after", "beforeafter"},
		{"escape char", "before\x1bafter", "beforeafter"},
		{"clean input", "no-special-chars", "no-special-chars"},
		{"combined", "user\ninput\rwith\ttabs\x00", "user\\ninput\\rwith\\ttabs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LogValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateWebhookURLBlocksPrivateIPs(t *testing.T) {
	blockedURLs := []string{
		"http://127.0.0.1/callback",
		"http://10.0.0.1/callback",
		"http://172.16.0.1/callback",
		"http://192.168.1.1/callback",
		"http://169.254.169.254/latest/meta-data/", // AWS IMDS
		"http://[::1]/callback",                     // IPv6 loopback
	}

	for _, u := range blockedURLs {
		t.Run(u, func(t *testing.T) {
			err := ValidateWebhookURL(u, true, false) // allowHTTP=true, allowLocal=false to isolate IP check
			require.Error(t, err, "should block private IP URL: %s", u)
			assert.Contains(t, err.Error(), "private")
		})
	}
}

func TestValidateWebhookURLRequiresHTTPS(t *testing.T) {
	err := ValidateWebhookURL("http://example.com/callback", false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestValidateWebhookURLAllowsHTTPWhenConfigured(t *testing.T) {
	// Public IPs with HTTP allowed — should pass
	err := ValidateWebhookURL("http://example.com/callback", true, false)
	assert.NoError(t, err)
}

func TestValidateWebhookURLAllowsHTTPS(t *testing.T) {
	err := ValidateWebhookURL("https://example.com/callback", false, false)
	assert.NoError(t, err)
}

func TestValidateWebhookURLRejectsInvalidSchemes(t *testing.T) {
	schemes := []string{
		"ftp://example.com/callback",
		"file:///etc/passwd",
		"gopher://evil.com",
	}

	for _, u := range schemes {
		t.Run(u, func(t *testing.T) {
			err := ValidateWebhookURL(u, true, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed")
		})
	}
}

func TestValidateWebhookURLAllowsEmpty(t *testing.T) {
	err := ValidateWebhookURL("", false, false)
	assert.NoError(t, err, "empty URL should be valid (optional field)")
}

func TestValidateWebhookURLRejectsNoHost(t *testing.T) {
	err := ValidateWebhookURL("https://", false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hostname")
}

func TestValidateWebhookURLAllowsPrivateIPsWhenAllowLocal(t *testing.T) {
	// With allowLocal=true, private IPs should be allowed
	err := ValidateWebhookURL("http://127.0.0.1/callback", true, true)
	assert.NoError(t, err, "should allow private IP when allowLocal=true")
}
