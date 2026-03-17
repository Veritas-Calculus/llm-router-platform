package sanitize

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── SafeDialer / DNS Rebinding Tests ───────────────────────────────────

func TestIsPrivateIPDetectsPrivateRanges(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1",    // loopback
		"10.0.0.1",     // RFC 1918 Class A
		"172.16.0.1",   // RFC 1918 Class B
		"192.168.1.1",  // RFC 1918 Class C
		"169.254.1.1",  // link-local
		"0.0.0.1",      // "this" network
		"100.64.0.1",   // shared address space (RFC 6598)
		"::1",          // IPv6 loopback
	}

	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			assert.True(t, IsPrivateIP(parsed), "expected %s to be private", ip)
		})
	}
}

func TestIsPrivateIPAllowsPublicIPs(t *testing.T) {
	publicIPs := []string{
		"8.8.8.8",       // Google DNS
		"1.1.1.1",       // Cloudflare DNS
		"93.184.216.34", // example.com
		"203.0.113.1",   // documentation range (TEST-NET-3, but not RFC 1918)
	}

	for _, ip := range publicIPs {
		t.Run(ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			assert.False(t, IsPrivateIP(parsed), "expected %s to be public", ip)
		})
	}
}

func TestSafeTransportReturnsTransport(t *testing.T) {
	transport := SafeTransport()
	assert.NotNil(t, transport)
	assert.NotNil(t, transport.DialContext, "SafeTransport must have a custom DialContext")
}

func TestSafeDialContextBlocksPrivateIPs(t *testing.T) {
	// Test that dialing 127.0.0.1 directly is blocked
	ctx := t.Context()
	_, err := safeDialContext(ctx, "tcp", "127.0.0.1:80")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private/reserved IP")
}

func TestSafeDialContextBlocksLoopbackHostname(t *testing.T) {
	// "localhost" resolves to 127.0.0.1, should be blocked
	ctx := t.Context()
	_, err := safeDialContext(ctx, "tcp", "localhost:80")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private/reserved IP")
}
