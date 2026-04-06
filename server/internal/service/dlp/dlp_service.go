package dlp

import (
	"context"
	"log"
	"regexp"
	"sync"
	"time"

	"llm-router-platform/internal/models"
)

const (
	StrategyRedact = "REDACT"
	StrategyBlock  = "BLOCK"

	maxCustomPatternLength = 200
	customRegexTimeout     = 100 * time.Millisecond
)

// Pre-compiled regular expressions for standard PII
var (
	emailRegex      = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRegex      = regexp.MustCompile(`(?:\b|\+)(?:\d{1,3}[\s-]?)?\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{4}\b`)
	creditCardRegex = regexp.MustCompile(`\b(?:\d{4}[\s-]?){3}\d{4}\b`)
	ssnRegex        = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	apiKeyRegex     = regexp.MustCompile(`\b(sk-[a-zA-Z0-9]{32,}|sk_live_[a-zA-Z0-9]{24,})\b`)
)

// regexCache stores pre-compiled custom regex patterns keyed by their source string.
var regexCache sync.Map // map[string]*regexp.Regexp

// getOrCompileRegex returns a compiled regex from cache, or compiles and caches it.
// Returns nil if the pattern is invalid or exceeds the length limit.
func getOrCompileRegex(pattern string) *regexp.Regexp {
	if len(pattern) > maxCustomPatternLength {
		log.Printf("[DLP] skipping custom regex: pattern length %d exceeds limit %d", len(pattern), maxCustomPatternLength)
		return nil
	}

	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("[DLP] skipping invalid custom regex %q: %v", pattern, err)
		return nil
	}

	regexCache.Store(pattern, re)
	return re
}

// runWithTimeout executes fn in a goroutine and returns its result, or the
// zero value if the per-pattern timeout is exceeded.
func runWithTimeout[T any](fn func() T) (T, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), customRegexTimeout)
	defer cancel()

	type result struct{ v T }
	ch := make(chan result, 1)
	go func() {
		ch <- result{fn()}
	}()

	select {
	case r := <-ch:
		return r.v, true
	case <-ctx.Done():
		var zero T
		return zero, false
	}
}

// ScrubText replaces all configured PII patterns with redacted placeholders.
// It also checks for custom regex defined in the configuration.
func ScrubText(content string, config *models.DlpConfig) string {
	if config == nil || !config.IsEnabled {
		return content
	}

	result := scrubBuiltinPII(content, config)
	result = scrubCustomPII(result, config.CustomRegex)
	return result
}

// HasPII returns true if any of the enabled PII patterns are found in the content.
func HasPII(content string, config *models.DlpConfig) bool {
	if config == nil || !config.IsEnabled {
		return false
	}

	if matchesBuiltinPII(content, config) {
		return true
	}
	return matchesCustomPII(content, config.CustomRegex)
}

// scrubBuiltinPII replaces built-in PII patterns in the content.
func scrubBuiltinPII(content string, config *models.DlpConfig) string {
	result := content
	if config.MaskCreditCards {
		result = creditCardRegex.ReplaceAllString(result, "[CREDIT_CARD_REDACTED]")
	}
	if config.MaskSSN {
		result = ssnRegex.ReplaceAllString(result, "[SSN_REDACTED]")
	}
	if config.MaskApiKeys {
		result = apiKeyRegex.ReplaceAllString(result, "[API_KEY_REDACTED]")
	}
	if config.MaskEmails {
		result = emailRegex.ReplaceAllString(result, "[EMAIL_REDACTED]")
	}
	if config.MaskPhones {
		result = phoneRegex.ReplaceAllString(result, "[PHONE_REDACTED]")
	}
	return result
}

// scrubCustomPII replaces custom regex patterns in the content.
// Patterns are pre-compiled and cached; matching runs with a per-pattern timeout.
func scrubCustomPII(content string, patterns []string) string {
	result := content
	for _, customPattern := range patterns {
		if customPattern == "" {
			continue
		}
		re := getOrCompileRegex(customPattern)
		if re == nil {
			continue
		}
		replaced, ok := runWithTimeout(func() string {
			return re.ReplaceAllString(result, "[CUSTOM_REDACTED]")
		})
		if !ok {
			log.Printf("[DLP] custom regex timed out after %v: %q", customRegexTimeout, customPattern)
			continue
		}
		result = replaced
	}
	return result
}

// matchesBuiltinPII returns true if any enabled built-in PII pattern matches.
func matchesBuiltinPII(content string, config *models.DlpConfig) bool {
	if config.MaskCreditCards && creditCardRegex.MatchString(content) {
		return true
	}
	if config.MaskSSN && ssnRegex.MatchString(content) {
		return true
	}
	if config.MaskApiKeys && apiKeyRegex.MatchString(content) {
		return true
	}
	if config.MaskEmails && emailRegex.MatchString(content) {
		return true
	}
	if config.MaskPhones && phoneRegex.MatchString(content) {
		return true
	}
	return false
}

// matchesCustomPII returns true if any custom regex pattern matches.
// Patterns are pre-compiled and cached; matching runs with a per-pattern timeout.
func matchesCustomPII(content string, patterns []string) bool {
	for _, customPattern := range patterns {
		if customPattern == "" {
			continue
		}
		re := getOrCompileRegex(customPattern)
		if re == nil {
			continue
		}
		matched, ok := runWithTimeout(func() bool {
			return re.MatchString(content)
		})
		if !ok {
			log.Printf("[DLP] custom regex timed out after %v: %q", customRegexTimeout, customPattern)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
