package dlp

import (
	"regexp"

	"llm-router-platform/internal/models"
)

const (
	StrategyRedact = "REDACT"
	StrategyBlock  = "BLOCK"
)

// Pre-compiled regular expressions for standard PII
var (
	emailRegex      = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRegex      = regexp.MustCompile(`(?:\b|\+)(?:\d{1,3}[\s-]?)?\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{4}\b`)
	creditCardRegex = regexp.MustCompile(`\b(?:\d{4}[\s-]?){3}\d{4}\b`)
	ssnRegex        = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	apiKeyRegex     = regexp.MustCompile(`\b(sk-[a-zA-Z0-9]{32,}|sk_live_[a-zA-Z0-9]{24,})\b`)
)

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
func scrubCustomPII(content string, patterns []string) string {
	result := content
	for _, customPattern := range patterns {
		if customPattern == "" {
			continue
		}
		if re, err := regexp.Compile(customPattern); err == nil {
			result = re.ReplaceAllString(result, "[CUSTOM_REDACTED]")
		}
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
func matchesCustomPII(content string, patterns []string) bool {
	for _, customPattern := range patterns {
		if customPattern == "" {
			continue
		}
		if re, err := regexp.Compile(customPattern); err == nil {
			if re.MatchString(content) {
				return true
			}
		}
	}
	return false
}
