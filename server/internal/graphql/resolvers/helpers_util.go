package resolvers

// Domain helpers: helpers_util

import (
	"time"
)

// ── Utility helpers ─────────────────────────────────────────────────

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefStrDefault(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

func derefBool(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func valInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func monthStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}
