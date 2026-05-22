package webhook

import (
	"net/http"
	"testing"
	"time"
)

func TestComputeBackoff(t *testing.T) {
	// retryCount 1 → ~backoffBase (±25% jitter)
	d := computeBackoff(1)
	if d < backoffBase*3/4 || d > backoffBase*5/4 {
		t.Fatalf("retry=1 produced %v, want ~%v ±25%%", d, backoffBase)
	}

	// retryCount=5 → 16×base, still under cap
	d = computeBackoff(5)
	want := backoffBase << 4
	if d < want*3/4 || d > want*5/4 {
		t.Fatalf("retry=5 produced %v, want ~%v ±25%%", d, want)
	}

	// retryCount very large → cap kicks in
	d = computeBackoff(100)
	if d > backoffCap*5/4 {
		t.Fatalf("retry=100 produced %v, must not exceed cap+jitter %v", d, backoffCap)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := parseRetryAfter(""); d != 0 {
		t.Fatalf("empty header should produce 0, got %v", d)
	}

	if d := parseRetryAfter("30"); d != 30*time.Second {
		t.Fatalf("delta-seconds 30 should produce 30s, got %v", d)
	}

	// delta-seconds beyond cap is clamped
	if d := parseRetryAfter("99999"); d != backoffCap {
		t.Fatalf("over-cap delta-seconds should clamp to %v, got %v", backoffCap, d)
	}

	// negative integer ignored — strconv accepts "-5", and we'd return that
	// negative duration. The repo will treat next_attempt_at <= NOW as eligible,
	// so a negative value is functionally a 0. Verify we don't propagate
	// nonsense.
	if d := parseRetryAfter("-5"); d > 0 {
		t.Fatalf("negative delta-seconds should be ignored, got %v", d)
	}

	// HTTP-date in the future
	future := time.Now().Add(45 * time.Second).UTC().Format(http.TimeFormat)
	d := parseRetryAfter(future)
	if d < 30*time.Second || d > 60*time.Second {
		t.Fatalf("http-date Retry-After expected ~45s, got %v", d)
	}

	// HTTP-date in the past
	past := time.Now().Add(-1 * time.Hour).UTC().Format(http.TimeFormat)
	if d := parseRetryAfter(past); d != 0 {
		t.Fatalf("past http-date should produce 0, got %v", d)
	}

	// Unparseable string
	if d := parseRetryAfter("not a date"); d != 0 {
		t.Fatalf("garbage Retry-After should produce 0, got %v", d)
	}
}
