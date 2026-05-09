package admin

import (
	"strings"
	"testing"
	"time"
)

func TestBuildLogQLQueryEscapesFilters(t *testing.T) {
	requestID := `req-"abc"`
	level := "error"

	query := buildLogQLQuery(`{app="llm-router"}`, &requestID, &level)

	if !strings.HasPrefix(query, `{app="llm-router"}`) {
		t.Fatalf("expected configured selector, got %q", query)
	}
	if !strings.Contains(query, `|= "req-\"abc\""`) {
		t.Fatalf("expected escaped request id filter, got %q", query)
	}
	if !strings.Contains(query, `|= "\"level\":\"error\""`) {
		t.Fatalf("expected escaped level filter, got %q", query)
	}
}

func TestBuildLogQLQueryDefaultsSelector(t *testing.T) {
	query := buildLogQLQuery("", nil, nil)
	if !strings.HasPrefix(query, `{container="llm-router-server"}`) {
		t.Fatalf("expected default selector, got %q", query)
	}
}

func TestLokiIntegrationOverridesRuntimeIgnoresEmptyDefaultRows(t *testing.T) {
	if lokiIntegrationOverridesRuntime(false, []byte(`{}`)) {
		t.Fatal("disabled empty integration row should not override environment config")
	}
	if !lokiIntegrationOverridesRuntime(true, []byte(`{}`)) {
		t.Fatal("enabled integration row should override environment config")
	}
	if !lokiIntegrationOverridesRuntime(false, []byte(`{"endpoint":"http://loki:3100"}`)) {
		t.Fatal("disabled non-empty integration row should override environment config")
	}
}

func TestQueryDirectionUsesRecentFirstForEmptySearch(t *testing.T) {
	if got := queryDirection(nil); got != "backward" {
		t.Fatalf("empty search direction = %q, want backward", got)
	}

	requestID := "req-123"
	if got := queryDirection(&requestID); got != "forward" {
		t.Fatalf("request trace direction = %q, want forward", got)
	}
}

func TestParseTimeRangeRejectsInvertedWindow(t *testing.T) {
	start := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	end := time.Date(2026, 5, 9, 11, 0, 0, 0, time.UTC).Format(time.RFC3339)

	if _, _, err := parseTimeRange(&start, &end); err == nil {
		t.Fatal("expected inverted time range to fail")
	}
}

func TestParseLogLineKeepsCleanRawForPlainText(t *testing.T) {
	entry := parseLogLine("1715155660000000000", "\x1b[31;1mrecord not found\x1b[0m")

	if entry.Message != "record not found" {
		t.Fatalf("message = %q, want cleaned plain text", entry.Message)
	}
	if entry.RawJSON == nil || *entry.RawJSON != "record not found" {
		t.Fatalf("raw log = %v, want cleaned raw line", entry.RawJSON)
	}
}
