// Package sanitize provides input sanitization utilities for logging, display, and security.

package sanitize

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ─── Log Sanitization ───────────────────────────────────────────────────

// logReplacer strips control characters that could enable log injection attacks.
var logReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\r", "\\r",
	"\t", "\\t",
	"\x00", "",
	"\x1b", "", // ESC
)

// LogValue removes newlines and control characters from a string
// to prevent log injection / log forging attacks.
// See: CWE-117 (Improper Output Neutralization for Logs)
func LogValue(s string) string {
	return logReplacer.Replace(s)
}

// ─── SSRF Validation ────────────────────────────────────────────────────

// privateRanges contains CIDR ranges that should be blocked for SSRF prevention.
var privateRanges = []string{
	"127.0.0.0/8",    // IPv4 loopback
	"10.0.0.0/8",     // RFC 1918 Class A
	"172.16.0.0/12",  // RFC 1918 Class B
	"192.168.0.0/16", // RFC 1918 Class C
	"169.254.0.0/16", // Link-local
	"0.0.0.0/8",      // "This" network
	"100.64.0.0/10",  // Shared address space (RFC 6598)
	"::1/128",        // IPv6 loopback
	"fc00::/7",       // IPv6 unique local
	"fe80::/10",      // IPv6 link-local
}

// parsedPrivateRanges is initialized once from privateRanges.
var parsedPrivateRanges []*net.IPNet

func init() {
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("sanitize: invalid CIDR in privateRanges: " + cidr)
		}
		parsedPrivateRanges = append(parsedPrivateRanges, network)
	}
}

// isPrivateIP checks if an IP address falls within any private/reserved range.
func isPrivateIP(ip net.IP) bool {
	for _, network := range parsedPrivateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateWebhookURL validates a URL is safe to use as a webhook callback target.
// It prevents SSRF by rejecting:
//   - Non-HTTPS schemes (unless allowHTTP is true for dev/testing)
//   - URLs pointing to private/reserved IP ranges
//   - URLs with no host or malformed structure
//
// Returns an error describing the validation failure, or nil if valid.
func ValidateWebhookURL(rawURL string, allowHTTP bool) error {
	if rawURL == "" {
		return nil // Empty URL is valid (optional field)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Scheme validation
	switch parsed.Scheme {
	case "https":
		// Always allowed
	case "http":
		if !allowHTTP {
			return fmt.Errorf("webhook URL must use HTTPS")
		}
	default:
		return fmt.Errorf("webhook URL scheme %q is not allowed (use HTTPS)", parsed.Scheme)
	}

	// Extract hostname
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL must have a hostname")
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve webhook URL hostname %q: %w", host, err)
	}

	// Check that ALL resolved IPs are public (not private/reserved)
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("webhook URL resolves to private/reserved IP address (blocked for security)")
		}
	}

	return nil
}
