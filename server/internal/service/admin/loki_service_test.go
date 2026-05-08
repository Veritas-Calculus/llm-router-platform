package admin

import (
	"strings"
	"testing"
)

func TestBuildLogQLQueryEscapesFilters(t *testing.T) {
	requestID := `req-"abc"`
	level := "error"

	query := buildLogQLQuery(&requestID, &level)

	if !strings.Contains(query, `|= "req-\"abc\""`) {
		t.Fatalf("expected escaped request id filter, got %q", query)
	}
	if !strings.Contains(query, `|= "\"level\":\"error\""`) {
		t.Fatalf("expected escaped level filter, got %q", query)
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

func TestParseLogLineKeepsCleanRawForPlainText(t *testing.T) {
	entry := parseLogLine("1715155660000000000", "\x1b[31;1mrecord not found\x1b[0m")

	if entry.Message != "record not found" {
		t.Fatalf("message = %q, want cleaned plain text", entry.Message)
	}
	if entry.RawJSON == nil || *entry.RawJSON != "record not found" {
		t.Fatalf("raw log = %v, want cleaned raw line", entry.RawJSON)
	}
}
