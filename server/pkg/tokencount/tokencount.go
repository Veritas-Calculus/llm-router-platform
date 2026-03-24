// Package tokencount provides accurate token counting for LLM billing.
// It uses tiktoken-go for precise BPE-based token counting compatible with
// OpenAI's tokenizer, with automatic model-to-encoding mapping and fallback
// heuristics for unsupported models.
package tokencount

import (
	"strings"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// encodingCache caches tiktoken encodings by name to avoid repeated initialization.
var (
	encodingCache   = make(map[string]*tiktoken.Tiktoken)
	encodingCacheMu sync.RWMutex
)

// modelEncodingEntry maps a model name prefix to a tiktoken encoding name.
type modelEncodingEntry struct {
	prefix   string
	encoding string
}

// modelEncodingList is ordered longest-prefix-first to ensure correct matching.
// For example, "gpt-4o" must match before "gpt-4".
var modelEncodingList = []modelEncodingEntry{
	// o200k_base family (longest prefixes first)
	{"gpt-4o", "o200k_base"},
	{"gpt-4.1", "o200k_base"},
	{"gpt-4.5", "o200k_base"},
	{"o1", "o200k_base"},
	{"o3", "o200k_base"},
	{"o4", "o200k_base"},
	// cl100k_base family
	{"gpt-4", "cl100k_base"},
	{"gpt-3.5", "cl100k_base"},
	{"gpt-35", "cl100k_base"},
	{"text-embedding", "cl100k_base"},
}

// defaultEncoding is used for models not in the map (Claude, Gemini, etc.).
// cl100k_base provides reasonable estimates for most modern models.
const defaultEncoding = "cl100k_base"

// getEncoding returns the tiktoken encoding for a given model name.
// It first tries tiktoken's built-in model lookup, then falls back to
// prefix matching, then to the default encoding.
func getEncoding(model string) (*tiktoken.Tiktoken, error) {
	// Determine encoding name
	encodingName := resolveEncodingName(model)

	// Check cache
	encodingCacheMu.RLock()
	if enc, ok := encodingCache[encodingName]; ok {
		encodingCacheMu.RUnlock()
		return enc, nil
	}
	encodingCacheMu.RUnlock()

	// Create encoding
	enc, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil, err
	}

	// Cache it
	encodingCacheMu.Lock()
	encodingCache[encodingName] = enc
	encodingCacheMu.Unlock()

	return enc, nil
}

// resolveEncodingName maps a model name to the appropriate tiktoken encoding.
func resolveEncodingName(model string) string {
	lower := strings.ToLower(model)

	// Try tiktoken's built-in model->encoding lookup first
	if enc, err := tiktoken.EncodingForModel(lower); err == nil && enc != nil {
		// tiktoken resolved it, but we need the name, not the encoding
		// So we use the model map below as our primary path
		_ = enc
	}

	// Prefix match against known model families (ordered longest-first)
	for _, entry := range modelEncodingList {
		if strings.HasPrefix(lower, entry.prefix) {
			return entry.encoding
		}
	}

	return defaultEncoding
}

// CountTokens returns the precise token count for the given text using
// the appropriate encoding for the model. If tiktoken initialization fails,
// it falls back to a heuristic estimate (~4 chars per token).
func CountTokens(text string, model string) int {
	if text == "" {
		return 0
	}

	enc, err := getEncoding(model)
	if err != nil {
		// Fallback to heuristic
		return heuristicCount(text)
	}

	tokens := enc.Encode(text, nil, nil)
	return len(tokens)
}

// heuristicCount provides a rough estimate when tiktoken is unavailable.
// Uses ~4 bytes per token for mixed-language content.
func heuristicCount(text string) int {
	charCount := len(text)
	tokens := (charCount + 3) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}
