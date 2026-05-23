# Money: float64 → shopspring/decimal

The audit (DB-C2) flagged that every money column used `float64`/`DOUBLE PRECISION` and that the various `roundCost` helpers don't actually fix the underlying representation error — they just hide it for one operation. Migration 000022 already converted every money column to `NUMERIC(20,8)` at the database level. This document covers the Go-side migration that follows.

## Status

- **Done:** every money column is `NUMERIC(20,8)` in Postgres. Existing rows up-cast losslessly; new rows store full precision.
- **In progress:** Go models still carry `float64`. The DB→Go cast is lossy (NUMERIC → float64) but no worse than the previous state.
- **Not started:** GraphQL API still returns money fields as JSON floats. Clients can't see fractional cents beyond float precision.

## Migration plan

### Phase 1 (this commit, done)

Database columns are `NUMERIC(20,8)`. Sum and aggregation queries in SQL are now exact regardless of how many rows you sum.

### Phase 2 (next): Go models adopt `shopspring/decimal.Decimal`

For each money field on a model, switch the Go type:

```go
// before
type Transaction struct {
    Amount  float64 `gorm:"not null" json:"amount"`
    Balance float64 `json:"balance"`
}

// after
import "github.com/shopspring/decimal"

type Transaction struct {
    Amount  decimal.Decimal `gorm:"type:numeric(20,8);not null" json:"amount"`
    Balance decimal.Decimal `gorm:"type:numeric(20,8)" json:"balance"`
}
```

`decimal.Decimal` implements `sql.Scanner` / `driver.Valuer` / `encoding/json.Marshaler` out of the box, so the only mechanical churn is at the boundaries where arithmetic happens (`a + b` → `a.Add(b)`, `a * b` → `a.Mul(b)`). The compiler errors guide you through every site.

Touch the following packages roughly in this order:

1. `internal/models/billing.go` and `internal/models/identity.go` (User.Balance, Transaction.Amount/Balance, Order.Amount).
2. `internal/repository/billing_extra_repository.go` — `UpdateUserBalance` / `FulfillRechargeOrder` write paths.
3. `internal/service/billing/balance.go` / `billing.go` / `payment.go` — comparison + math.
4. `internal/service/billing/cost.go` — pricing calculation. Keep the inputs as `decimal.Decimal` end-to-end so you don't reintroduce float64 in the middle.
5. `internal/api/handlers/payment_handler.go` and the Stripe/WeChat/Alipay services for input parsing.
6. GraphQL resolvers (`billing.resolvers.go`, plan/usage queries) — serialize via the JSON marshaler (default `decimal.Decimal` marshals to a quoted string, which most JS clients handle correctly).

### Phase 3: GraphQL Money scalar

Add a `scalar Money` to `internal/graphql/schema/schema.graphqls` and a custom scalar definition in `internal/graphql/handler/handler.go` that wraps `decimal.Decimal`. Frontend then deserializes via the same string form — pick a JS decimal library on the client (recommended: `decimal.js` or `bignumber.js`).

Once Phase 3 lands, remove every remaining `float64` money field in Go and the `roundCost` helpers.

## Why not flag-day everything

The Phase 1 SQL migration is safe to land in isolation because PostgreSQL accepts NUMERIC values from a Go `float64` writer (the GORM driver converts) and returns NUMERIC values to a Go `float64` reader (lossy, but no worse than before). The Phase 2 + 3 work is much wider — every billing-touching test must update — and benefits from a separate review cycle.

## Why decimal.js / bignumber.js on the frontend

JavaScript's `Number` is float64 with the same representation pitfalls. If we return a JSON number from the backend, the frontend's `0.1 + 0.2 !== 0.3` problem is back at the UI layer. Returning Money as a string and parsing into `decimal.js` keeps the precision intact.
