package router

import (
	"encoding/json"
	"regexp"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
)

// Default NSFW / dev-only name regex (audit L-05). Matches names like
// `qwen3.5-35b-a3b-uncensored-hauhaucs-aggressive` so we can mark them
// inactive before they reach customer-facing dropdowns. Overridable via
// PROVIDER_SYNC_BLOCKLIST_REGEX.
const defaultBlocklistRegex = `(?i)(uncensored|abliterated|jailbreak(ed)?|nsfw|aggressive)`

// ClassifyModel returns the most specific ModelKind for an upstream
// /v1/models entry. Exported so the OpenAI-compatible REST handler can
// reuse the same classification logic when populating the `model_kind`
// field on a /v1/models response. See classifyModel for the full
// docstring.
func ClassifyModel(info provider.ModelInfo) models.ModelKind {
	return classifyModel(info)
}

// classifyModel returns the most specific ModelKind for an upstream
// /v1/models entry. The ordering of checks is significant:
//
//   - STT/TTS name fragments are matched before "embedding" so a model
//     named "whisper-embedding-eval" still ends up as STT.
//   - "embedding" beats "chat" so a model that advertises both
//     embedding and chat capabilities (some local models do) is correctly
//     classified as embedding.
//   - Anything that didn't match a specific kind but advertises chat /
//     completion / vision capability falls into the chat bucket.
//
// The "type" field in LM Studio's response is always "llm" and is
// deliberately ignored — see audit M-02 for context.
func classifyModel(info provider.ModelInfo) models.ModelKind {
	id := strings.ToLower(info.ID)

	// Order matters. We pick the *most specific* kind for ambiguous
	// names: rerank wins over embedding (so `bge-reranker-v2-m3` is a
	// reranker, not an embedding), STT/TTS win over both (so a
	// hypothetical `whisper-embedding-eval` is still STT).
	switch {
	case matchesAny(id, sttNamePatterns):
		return models.ModelKindSTT
	case matchesAny(id, ttsNamePatterns):
		return models.ModelKindTTS
	case matchesAny(id, rerankNamePatterns):
		return models.ModelKindRerank
	case matchesAny(id, embeddingNamePatterns):
		return models.ModelKindEmbedding
	case matchesAny(id, imageNamePatterns):
		return models.ModelKindImage
	}

	// Fall back to capability hints from the upstream payload. LM Studio
	// reports capabilities for native models but the OpenAI-compatible
	// shim doesn't, so we use this only as a tiebreaker when the name
	// didn't match a specific kind.
	//
	// Note: LM Studio's `type` is always literally "llm" on the
	// OpenAI-compatible shim, which carries no real classification signal
	// — see audit M-02. We deliberately don't treat "llm" as chat here so
	// the row surfaces as `unknown` for an operator to classify.
	if info.Extra != nil {
		if raw, ok := info.Extra["capabilities"]; ok {
			var caps struct {
				Chat       bool `json:"chat"`
				Completion bool `json:"completion"`
				Vision     bool `json:"vision"`
			}
			if err := json.Unmarshal(raw, &caps); err == nil {
				if caps.Chat || caps.Completion || caps.Vision {
					return models.ModelKindChat
				}
			}
		}
		if raw, ok := info.Extra["type"]; ok {
			var t string
			if err := json.Unmarshal(raw, &t); err == nil {
				lt := strings.ToLower(t)
				if lt == "vlm" || lt == "chat" {
					return models.ModelKindChat
				}
				if lt == "embeddings" || lt == "embedding" {
					return models.ModelKindEmbedding
				}
			}
		}
	}

	// Default: no name match and no usable capability hint — surface as
	// `unknown` so the admin UI can prompt the operator to classify
	// manually. We deliberately do NOT default to chat: a brand-new audio
	// or embedding model silently appearing in the chat dropdown is a
	// worse failure than "no kind shown, please pick one" (post-audit
	// P0-4). The Playground's main Model dropdown still treats UNKNOWN
	// as chat-eligible client-side, so the dev workflow is preserved.
	return models.ModelKindUnknown
}

var (
	sttNamePatterns       = []string{"whisper", "speech-to-text", "stt-", "-stt-", "/stt-"}
	ttsNamePatterns       = []string{"tts-", "-tts-", "/tts-", "text-to-speech", "kokoro", "elevenlabs", "cosyvoice", "bark-", "parler"}
	embeddingNamePatterns = []string{"embedding", "embed-", "bge-", "nomic-embed", "-embed/"}
	imageNamePatterns     = []string{"dall-e", "stable-diffusion", "sd-", "flux-", "flux.", "midjourney"}
	rerankNamePatterns    = []string{"rerank", "reranker"}
)

func matchesAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// extractContextAndOutput pulls the context window and (optionally) the
// max output token cap out of an upstream model info blob.
//
// LM Studio's native /api/v1/models reports `max_context_length` which we
// stored under that key in the ModelInfo.Extra map and ALSO under
// `max_tokens` for backwards compatibility. Both of those describe the
// total prompt+completion window, not the per-request output cap, so we
// always treat them as context_window. Other providers that explicitly
// supply a separate output cap (currently none of our built-in clients do)
// can populate `max_output_tokens` in the Extra map and it will flow
// through here.
func extractContextAndOutput(info provider.ModelInfo) (contextWindow int, maxOutput *int) {
	if info.Extra == nil {
		return 0, nil
	}

	for _, key := range []string{"max_context_length", "context_length", "context_window"} {
		if raw, ok := info.Extra[key]; ok {
			var n int
			if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
				contextWindow = n
				break
			}
		}
	}
	if contextWindow == 0 {
		if raw, ok := info.Extra["max_tokens"]; ok {
			var n int
			if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
				// LM Studio reports max_tokens=max_context_length, so this
				// is a context window — NOT an output cap.
				contextWindow = n
			}
		}
	}
	if raw, ok := info.Extra["max_output_tokens"]; ok {
		var n int
		if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
			maxOutput = &n
		}
	}
	return contextWindow, maxOutput
}

// catalogSyncRules captures the operator-tunable parts of the auto-sync
// behaviour. Construct via newCatalogSyncRules.
type catalogSyncRules struct {
	AutoActivate bool
	Blocklist    *regexp.Regexp
}

// newCatalogSyncRules compiles the operator-supplied blocklist regex once
// per sync. If the regex is empty we fall back to the default. If it's
// malformed we log nothing here and fall back; the caller is expected to
// surface the misconfiguration during startup validation.
func newCatalogSyncRules(autoActivate bool, customRegex string) *catalogSyncRules {
	pattern := strings.TrimSpace(customRegex)
	if pattern == "" {
		pattern = defaultBlocklistRegex
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		re = regexp.MustCompile(defaultBlocklistRegex)
	}
	return &catalogSyncRules{
		AutoActivate: autoActivate,
		Blocklist:    re,
	}
}

// shouldActivate decides whether a freshly-discovered model row should
// land in the catalogue active or inactive. Rule:
//
//   - If the name matches the blocklist regex, always inactive (and stamp
//     a warning so the admin UI can surface why).
//   - Otherwise honour the operator's AutoActivate preference.
//
// Returns (isActive, warning). warning is empty for a clean activation.
func (r *catalogSyncRules) shouldActivate(name string) (bool, string) {
	if r.Blocklist != nil && r.Blocklist.MatchString(name) {
		return false, "nsfw-or-dev-name"
	}
	return r.AutoActivate, ""
}
