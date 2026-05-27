package router

import (
	"encoding/json"
	"testing"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
)

func TestClassifyModel(t *testing.T) {
	cases := []struct {
		name     string
		modelID  string
		extra    map[string]any
		expected models.ModelKind
	}{
		// Audit M-02 examples — these are the ones that were leaking
		// into the Playground's STT/TTS dropdowns.
		{"whisper turbo", "whisper-large-v3-turbo", nil, models.ModelKindSTT},
		{"kokoro tts", "kokoro-82m-tts", nil, models.ModelKindTTS},
		{"bge embedding", "text-embedding-bge-m3", nil, models.ModelKindEmbedding},
		{"nomic embed", "text-embedding-nomic-embed-text-v1.5", nil, models.ModelKindEmbedding},
		// qwen-style chat names carry no embedding/audio/image keyword, so
		// the classifier has no signal — it must surface as `unknown` and
		// let the operator (or the next sync round with capabilities) set
		// the real kind. Post-audit P0-4 changed this from a silent
		// "default to chat" to "default to unknown".
		{"qwen name with no hint defaults to unknown", "qwen/qwen3.6-35b-a3b", nil, models.ModelKindUnknown},
		// With an explicit `chat` capability hint the same name correctly
		// resolves to chat — the capability is the real signal, not the
		// name. Mirrors the original audit M-02 contract for providers
		// that report capabilities.
		{"qwen name with chat capability resolves to chat", "qwen/qwen3.6-35b-a3b", map[string]any{
			"capabilities": map[string]bool{"chat": true},
		}, models.ModelKindChat},
		{"dall-e image", "dall-e-3", nil, models.ModelKindImage},
		{"flux image", "flux-pro-1.1", nil, models.ModelKindImage},
		{"reranker", "bge-reranker-v2-m3", nil, models.ModelKindRerank},
		// Lock in the rerank-before-embedding ordering. The name contains
		// both `bge-` (an embedding hint) and `reranker`; the more specific
		// rerank match must win. Without this guard a refactor that moves
		// embedding above rerank in the switch would silently mis-classify
		// reranker models as embeddings — which is what migration 000025
		// repaired in the live DB. See post-audit follow-up P0-1.
		{"bge-reranker name (rerank > embedding)", "bge-reranker-v2-m3", nil, models.ModelKindRerank},
		{"jina rerank name", "jina-rerank-v2-multilingual", nil, models.ModelKindRerank},

		// Unclassifiable name + no capability hint surfaces as `unknown`
		// rather than silently defaulting to chat. This lets the admin UI
		// prompt the operator to set the kind by hand instead of dumping
		// a newly-published audio/embedding/rerank model into the chat
		// dropdown (post-audit follow-up P0-4).
		{"unknown when no signal", "random-experimental-model-v3", nil, models.ModelKindUnknown},
		{"unknown ignores llm type alone", "qwen/qwen3-4b", map[string]any{
			"type": "llm",
		}, models.ModelKindUnknown},

		// Mixed-capability edge cases.
		{"embedding wins over chat", "qwen-embedding-3b", map[string]any{
			"capabilities": map[string]bool{"chat": true, "completion": true},
		}, models.ModelKindEmbedding},
		{"capability fallback to chat", "some-mystery-model", map[string]any{
			"capabilities": map[string]bool{"chat": true},
		}, models.ModelKindChat},
		{"vlm via type", "some-vlm", map[string]any{
			"type": "vlm",
		}, models.ModelKindChat},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := provider.ModelInfo{ID: tc.modelID}
			if tc.extra != nil {
				info.Extra = map[string]json.RawMessage{}
				for k, v := range tc.extra {
					b, err := json.Marshal(v)
					if err != nil {
						t.Fatalf("marshal extra %q: %v", k, err)
					}
					info.Extra[k] = b
				}
			}
			got := classifyModel(info)
			if got != tc.expected {
				t.Fatalf("classifyModel(%q) = %q, want %q", tc.modelID, got, tc.expected)
			}
		})
	}
}

func TestExtractContextAndOutput(t *testing.T) {
	// LM Studio shape: max_context_length in Extra. We treat it as the
	// context window (audit M-07).
	t.Run("lm studio max_context_length", func(t *testing.T) {
		info := provider.ModelInfo{ID: "x", Extra: map[string]json.RawMessage{
			"max_context_length": json.RawMessage(`262144`),
		}}
		ctx, out := extractContextAndOutput(info)
		if ctx != 262144 {
			t.Fatalf("context_window = %d, want 262144", ctx)
		}
		if out != nil {
			t.Fatalf("max_output_tokens should be nil for LM Studio payloads, got %v", *out)
		}
	})

	// LM Studio also reports max_tokens (= max_context_length) in its
	// shim payload. Treat it as context window, NOT as an output cap.
	t.Run("lm studio max_tokens alias", func(t *testing.T) {
		info := provider.ModelInfo{ID: "x", Extra: map[string]json.RawMessage{
			"max_tokens": json.RawMessage(`262144`),
		}}
		ctx, out := extractContextAndOutput(info)
		if ctx != 262144 {
			t.Fatalf("context_window = %d, want 262144", ctx)
		}
		if out != nil {
			t.Fatalf("max_output_tokens should be nil, got %v", *out)
		}
	})

	t.Run("explicit max_output_tokens passes through", func(t *testing.T) {
		info := provider.ModelInfo{ID: "x", Extra: map[string]json.RawMessage{
			"max_context_length": json.RawMessage(`128000`),
			"max_output_tokens":  json.RawMessage(`4096`),
		}}
		ctx, out := extractContextAndOutput(info)
		if ctx != 128000 {
			t.Fatalf("context_window = %d, want 128000", ctx)
		}
		if out == nil || *out != 4096 {
			t.Fatalf("max_output_tokens = %v, want 4096", out)
		}
	})

	t.Run("empty extra returns zeros", func(t *testing.T) {
		ctx, out := extractContextAndOutput(provider.ModelInfo{ID: "x"})
		if ctx != 0 || out != nil {
			t.Fatalf("expected zeros, got ctx=%d out=%v", ctx, out)
		}
	})
}

func TestCatalogSyncRules(t *testing.T) {
	t.Run("nsfw match forces inactive", func(t *testing.T) {
		rules := newCatalogSyncRules(true, "")
		active, warning := rules.shouldActivate("qwen3.5-35b-a3b-uncensored-hauhaucs-aggressive")
		if active {
			t.Fatalf("NSFW match must be inactive even when AutoActivate=true")
		}
		if warning == "" {
			t.Fatalf("warning string must be non-empty for NSFW match")
		}
	})

	t.Run("auto activate off", func(t *testing.T) {
		rules := newCatalogSyncRules(false, "")
		active, warning := rules.shouldActivate("qwen3.6-35b-a3b")
		if active {
			t.Fatalf("AutoActivate=false should leave model inactive")
		}
		if warning != "" {
			t.Fatalf("clean model should not carry a warning, got %q", warning)
		}
	})

	t.Run("auto activate on", func(t *testing.T) {
		rules := newCatalogSyncRules(true, "")
		active, warning := rules.shouldActivate("qwen3.6-35b-a3b")
		if !active {
			t.Fatalf("AutoActivate=true should activate clean model")
		}
		if warning != "" {
			t.Fatalf("clean model should not carry a warning, got %q", warning)
		}
	})

	t.Run("custom regex", func(t *testing.T) {
		rules := newCatalogSyncRules(true, `(?i)beta-only`)
		active, _ := rules.shouldActivate("foo-beta-only")
		if active {
			t.Fatalf("custom regex should mark model inactive")
		}
		active, _ = rules.shouldActivate("foo-stable")
		if !active {
			t.Fatalf("custom regex should leave non-matching model active")
		}
	})

	t.Run("malformed regex falls back to default", func(t *testing.T) {
		rules := newCatalogSyncRules(true, `[invalid(`)
		// Should still mark NSFW names inactive via the default fallback.
		active, _ := rules.shouldActivate("uncensored-foo")
		if active {
			t.Fatalf("malformed regex should fall back to default and still block NSFW")
		}
	})
}
