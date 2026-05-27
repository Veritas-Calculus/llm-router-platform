package handlers

import (
	"encoding/json"
	"testing"

	"llm-router-platform/internal/service/provider"
)

// Audit M-04: LM Studio's reasoning phase emits ~150 empty-delta frames
// per request. We filter them server-side so SDK clients don't have to.
// finish_reason / usage / non-empty content all keep the chunk; only the
// pure heartbeats are dropped.
func TestIsEmptyDeltaChunk(t *testing.T) {
	cases := []struct {
		name string
		c    *provider.StreamChunk
		want bool
	}{
		{
			name: "nil chunk",
			c:    nil,
			want: true,
		},
		{
			name: "no choices",
			c:    &provider.StreamChunk{},
			want: true,
		},
		{
			name: "single empty delta",
			c: &provider.StreamChunk{
				Choices: []provider.DeltaChoice{
					{Delta: provider.Delta{}},
				},
			},
			want: true,
		},
		{
			name: "delta with content forwarded",
			c: &provider.StreamChunk{
				Choices: []provider.DeltaChoice{
					{Delta: provider.Delta{Content: "hello"}},
				},
			},
			want: false,
		},
		{
			name: "role-only delta forwarded (first SSE frame)",
			c: &provider.StreamChunk{
				Choices: []provider.DeltaChoice{
					{Delta: provider.Delta{Role: "assistant"}},
				},
			},
			want: false,
		},
		{
			name: "tool_call delta forwarded",
			c: &provider.StreamChunk{
				Choices: []provider.DeltaChoice{
					{Delta: provider.Delta{ToolCalls: json.RawMessage(`[{"id":"x"}]`)}},
				},
			},
			want: false,
		},
		{
			name: "terminal finish_reason forwarded even with empty delta",
			c: &provider.StreamChunk{
				Choices: []provider.DeltaChoice{
					{Delta: provider.Delta{}, FinishReason: "stop"},
				},
			},
			want: false,
		},
		{
			name: "usage block alone forwarded (final stream event)",
			c: &provider.StreamChunk{
				Choices: []provider.DeltaChoice{{Delta: provider.Delta{}}},
				Usage: &provider.Usage{
					PromptTokens:     5,
					CompletionTokens: 12,
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := isEmptyDeltaChunk(tc.c)
			if got != tc.want {
				t.Fatalf("isEmptyDeltaChunk = %v, want %v", got, tc.want)
			}
		})
	}
}
