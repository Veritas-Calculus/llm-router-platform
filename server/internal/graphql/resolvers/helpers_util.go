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

// Pagination bounds used across admin resolvers. Keeping these central makes
// it easy to audit the max row a single GraphQL request can pull.
const (
	defaultPageSize = 20
	maxPageSize     = 200
	maxPage         = 10000
)

// clampPagination normalizes (page, pageSize) pointers into a sane, bounded
// (page, pageSize) pair. Callers that receive nil or hostile values (negative,
// zero, or arbitrarily large) end up with safe defaults rather than issuing
// unbounded DB queries.
func clampPagination(page, pageSize *int) (int, int) {
	p := valInt(page, 1)
	if p < 1 {
		p = 1
	}
	if p > maxPage {
		p = maxPage
	}
	ps := valInt(pageSize, defaultPageSize)
	if ps <= 0 {
		ps = defaultPageSize
	}
	if ps > maxPageSize {
		ps = maxPageSize
	}
	return p, ps
}

// clampLimit normalizes a single limit pointer into a bounded positive int.
// Used by endpoints that take `limit:` without an explicit page.
func clampLimit(limit *int, def, max int) int {
	v := valInt(limit, def)
	if v <= 0 {
		v = def
	}
	if v > max {
		v = max
	}
	return v
}

// clampOffset normalizes an offset pointer to a non-negative bounded int.
func clampOffset(offset *int, max int) int {
	v := valInt(offset, 0)
	if v < 0 {
		v = 0
	}
	if v > max {
		v = max
	}
	return v
}

func monthStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}
