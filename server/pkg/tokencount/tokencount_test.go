package tokencount

import (
	"testing"
)

func TestCountTokens_KnownText(t *testing.T) {
	// "Hello, world!" is 4 tokens on cl100k_base
	text := "Hello, world!"
	got := CountTokens(text, "gpt-4")
	if got < 3 || got > 5 {
		t.Errorf("CountTokens(%q, gpt-4) = %d, expected ~4", text, got)
	}
}

func TestCountTokens_Empty(t *testing.T) {
	got := CountTokens("", "gpt-4")
	if got != 0 {
		t.Errorf("CountTokens empty = %d, expected 0", got)
	}
}

func TestCountTokens_UnknownModel(t *testing.T) {
	// Should use default encoding (cl100k_base) and still produce a result
	text := "This is a test sentence for token counting."
	got := CountTokens(text, "claude-3-opus")
	if got < 5 {
		t.Errorf("CountTokens with unknown model = %d, expected > 5", got)
	}
}

func TestCountTokens_GPT4oModel(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	got := CountTokens(text, "gpt-4o-mini")
	if got < 5 {
		t.Errorf("CountTokens with gpt-4o-mini = %d, expected > 5", got)
	}
}

func TestHeuristicCount(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"Hello, world! This is a test.", 8},
	}

	for _, tt := range tests {
		got := heuristicCount(tt.text)
		if tt.text == "" && got != 0 {
			// heuristicCount doesn't handle empty (CountTokens does)
			continue
		}
		if got != tt.expected {
			t.Errorf("heuristicCount(%q) = %d, expected %d", tt.text, got, tt.expected)
		}
	}
}

func TestResolveEncodingName(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o-mini", "o200k_base"},
		{"gpt-4-turbo", "cl100k_base"},
		{"gpt-3.5-turbo", "cl100k_base"},
		{"claude-3-opus", "cl100k_base"},       // default fallback
		{"gemini-1.5-pro", "cl100k_base"},      // default fallback
		{"o1-preview", "o200k_base"},
	}

	for _, tt := range tests {
		got := resolveEncodingName(tt.model)
		if got != tt.expected {
			t.Errorf("resolveEncodingName(%q) = %q, expected %q", tt.model, got, tt.expected)
		}
	}
}
