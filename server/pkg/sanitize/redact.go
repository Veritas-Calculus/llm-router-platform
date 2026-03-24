// Package sanitize provides utilities for sanitizing and redacting sensitive data.
// This file contains functions for redacting sensitive patterns (API keys, credentials)
// from error messages and HTTP headers before they are persisted to the database.
package sanitize

import (
	"encoding/json"
	"regexp"
	"strings"
)

// maxErrorMessageLen is the maximum length for error messages stored in the database.
// This prevents excessive data storage and limits exposure of upstream response bodies.
const maxErrorMessageLen = 500

// maxResponseBodyLen is the maximum length for upstream response bodies stored in error logs.
const maxResponseBodyLen = 2000

// sensitiveKeyPattern matches common API key prefixes from LLM providers.
// Covers OpenAI (sk-), Anthropic (sk-ant-), Google (AIza), and generic patterns.
var sensitiveKeyPattern = regexp.MustCompile(`(?i)(sk-|pk-|key-|api[_-]?key[=: "]+|bearer )[A-Za-z0-9_\-]{8,}`)

// sensitiveHeaderKeys are HTTP header names that may contain authentication credentials.
// These are removed from headers before persistence.
var sensitiveHeaderKeys = map[string]bool{
	"authorization":       true,
	"x-api-key":           true,
	"api-key":             true,
	"x-auth-token":        true,
	"proxy-authorization": true,
	"x-goog-api-key":      true,
	"anthropic-api-key":   true,
}

// RedactSecrets replaces sensitive credential patterns in a string with [REDACTED].
// This is used to sanitize error messages and response bodies before database storage.
func RedactSecrets(s string) string {
	return sensitiveKeyPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Preserve the prefix for context, redact the key value
		for _, prefix := range []string{"sk-", "pk-", "key-", "bearer ", "Bearer "} {
			if strings.HasPrefix(match, prefix) || strings.HasPrefix(strings.ToLower(match), strings.ToLower(prefix)) {
				return prefix + "[REDACTED]"
			}
		}
		return "[REDACTED]"
	})
}

// TruncateErrorMessage sanitizes and truncates an error message for database storage.
// It redacts sensitive patterns and limits length to prevent data leakage.
func TruncateErrorMessage(msg string) string {
	msg = RedactSecrets(msg)
	if len(msg) > maxErrorMessageLen {
		return msg[:maxErrorMessageLen] + "...[truncated]"
	}
	return msg
}

// TruncateResponseBody sanitizes and truncates a response body for database storage.
func TruncateResponseBody(body []byte) []byte {
	s := RedactSecrets(string(body))
	if len(s) > maxResponseBodyLen {
		s = s[:maxResponseBodyLen] + "...[truncated]"
	}
	return []byte(s)
}

// RedactHeaders removes sensitive authentication headers from a header map
// and returns the sanitized JSON bytes suitable for database storage.
func RedactHeaders(headers map[string][]string) []byte {
	if len(headers) == 0 {
		return nil
	}
	cleaned := make(map[string][]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaderKeys[strings.ToLower(k)] {
			cleaned[k] = []string{"[REDACTED]"}
		} else {
			cleaned[k] = v
		}
	}
	b, _ := json.Marshal(cleaned)
	return b
}
