// Package sanitize provides input sanitization utilities for logging and display.
package sanitize

import "strings"

// logReplacer strips control characters that could enable log injection attacks.
var logReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\r", "\\r",
	"\t", "\\t",
	"\x00", "",
	"\x1b", "", // ESC
)

// LogValue removes newlines and control characters from a string
// to prevent log injection / log forging attacks.
// See: CWE-117 (Improper Output Neutralization for Logs)
func LogValue(s string) string {
	return logReplacer.Replace(s)
}
