package safety

import (
	"context"
	"regexp"

	"llm-router-platform/internal/service/provider"
)

// ─── Built-in Prompt Injection Detection Rules ──────────────────────────────
//
// These rules detect high-confidence prompt injection patterns that can be
// evaluated with zero latency (regex-only, no model inference). They serve
// as a fast pre-filter before Llama Guard (which requires model inference).

// Rule represents a single detection rule.
type Rule struct {
	// Name is a human-readable identifier for the rule.
	Name string
	// Category maps to Llama Guard taxonomy (e.g., "S13" for prompt injection).
	Category string
	// Pattern is a compiled regex pattern to match against message content.
	Pattern *regexp.Regexp
	// Enabled controls whether this rule is active.
	Enabled bool
}

// RuleEngine is a regex-based safety classifier that detects known prompt
// injection patterns. It implements the Classifier interface and can be
// composed with other classifiers (e.g., NoopClassifier, LlamaGuardClassifier).
type RuleEngine struct {
	rules []Rule
}

// NewRuleEngine creates a RuleEngine with the default built-in rules.
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules: defaultRules(),
	}
}

// defaultRules returns the built-in prompt injection detection rules.
func defaultRules() []Rule {
	return []Rule{
		{
			Name:     "system_prompt_override",
			Category: "S13",
			Pattern:  regexp.MustCompile(`(?i)(ignore\s+(all\s+)?(previous|prior|above|earlier)\s+(instructions?|prompts?|rules?|directives?)|forget\s+(your|all|previous)\s+(instructions?|rules?|programming)|disregard\s+(all\s+)?(previous|prior|above)\s+(instructions?|text|context))`),
			Enabled:  true,
		},
		{
			Name:     "role_hijack",
			Category: "S13",
			Pattern:  regexp.MustCompile(`(?i)(you\s+are\s+now\s+(a|an|the)\s+|from\s+now\s+on\s+you\s+(are|will|must|should)\s+|act\s+as\s+(if\s+you\s+are|a\s+)|pretend\s+(to\s+be|you\s+are)\s+|roleplay\s+as\s+)`),
			Enabled:  true,
		},
		{
			Name:     "system_token_injection",
			Category: "S13",
			Pattern:  regexp.MustCompile(`(\[SYSTEM\]|<<SYS>>|<\|im_start\|>system|<\|system\|>|\[INST\]\s*<<SYS>>)`),
			Enabled:  true,
		},
		{
			Name:     "delimiter_escape",
			Category: "S13",
			Pattern:  regexp.MustCompile(`(?i)(---{3,}|==={3,}|###\s*(END|STOP|IGNORE|BEGIN)\s*(OF\s+)?(SYSTEM|INSTRUCTIONS?|CONTEXT|PROMPT))`),
			Enabled:  true,
		},
		{
			Name:     "instruction_leak",
			Category: "S13",
			Pattern:  regexp.MustCompile(`(?i)(print\s+(your|the)\s+(system\s+)?(prompt|instructions?|rules?|programming)|reveal\s+(your|the)\s+(system\s+)?(prompt|instructions?)|output\s+(your|the)\s+(initial|system|original)\s+(prompt|instructions?))`),
			Enabled:  true,
		},
		{
			Name:     "jailbreak_keywords",
			Category: "S13",
			Pattern:  regexp.MustCompile(`(?i)(DAN\s+mode|do\s+anything\s+now|jailbreak|developer\s+mode\s+(enabled|on)|evil\s+mode|god\s+mode\s+(enabled|on)|bypass\s+(all\s+)?(safety|content)\s+(filter|restrict))`),
			Enabled:  true,
		},
	}
}

// Classify scans all messages for prompt injection patterns.
// Returns unsafe result on first match (short-circuit).
func (r *RuleEngine) Classify(_ context.Context, messages []provider.Message) (*SafetyResult, error) {
	for _, msg := range messages {
		// FlexibleContent.Text is populated by UnmarshalJSON and already
		// contains concatenated text from multi-part (multimodal) messages.
		text := msg.Content.Text
		if text == "" {
			continue
		}

		for _, rule := range r.rules {
			if !rule.Enabled {
				continue
			}
			if rule.Pattern.MatchString(text) {
				return &SafetyResult{
					Safe:     false,
					Category: rule.Category,
					Score:    1.0,
					Reason:   "Matched rule: " + rule.Name,
				}, nil
			}
		}
	}

	return &SafetyResult{Safe: true}, nil
}

