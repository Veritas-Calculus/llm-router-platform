# Money: float64 → shopspring/decimal

The audit (DB-C2) flagged that every money column used `float64`/`DOUBLE PRECISION` and that the various `roundCost` helpers don't actually fix the underlying representation error — they just hide it for one operation. Migration 000022 already converted every money column to `NUMERIC(20,8)` at the database level. This document covers the Go-side migration that follows.

## Status

- **Done:** every money column is `NUMERIC(20,8)` in Postgres. Existing rows up-cast losslessly; new rows store full precision.
- **Done:** core balance/ledger, model pricing, plan-price, usage-cost, budget, coupon/redeem credit, and org/project/user limit fields (`User.Balance`, `User.MonthlyBudgetUSD`, `Model.*Price*` / `Model.*Cost*`, `Plan.PriceMonth`, `UsageLog.Cost` / `UsageLog.CustomerCharge` / `UsageLog.ProviderCost`, `Order.Amount`, `Transaction.Amount` / `Transaction.Balance`, `Budget.MonthlyLimitUSD`, `Coupon.DiscountValue` / `Coupon.MinAmount`, `RedeemCode.CreditAmount`, `Organization.BillingLimit`, `Project.QuotaLimit`) are `shopspring/decimal.Decimal` in Go and persist at the same 8-digit DB scale.
- **Done:** billing calculations, usage summary DTOs, current-month Redis usage caches, core balance mutations, budget checks, payment checkout inputs, payment webhook amount checks, balance fulfillment repository methods, and SQL financial/usage aggregate rows now use `shopspring/decimal` internally and round to the DB/provider scale before writing.
- **Done:** GraphQL now exposes primary persisted money fields and aggregate/reporting money fields through a `Money` scalar serialized as a decimal string: user balance/budgets, org/project limits, model pricing/costs, plans, orders, budgets, coupons, redeem credits, usage summaries, finance dashboards, revenue/cost rollups, and chart points. The web app accepts `Money` inputs as strings or numbers, formats string responses safely, and normalizes chart data to numbers only at visualization boundaries.
- **Done:** admin revenue stats and the financial dashboard service DTOs now carry `decimal.Decimal` through to GraphQL instead of converting to `float64` internally.
- **Remaining:** no known money write/enforcement path depends on `float64`. The only intentional floating-point values left are non-money API metrics such as percentages and standard-deviation scores. Deprecated REST payment helpers still emit JSON numbers for backward compatibility, but they serialize from decimal rather than float.

## Migration plan

### Phase 1 (this commit, done)

Database columns are `NUMERIC(20,8)`. Sum and aggregation queries in SQL are now exact regardless of how many rows you sum.

### Phase 2 (in progress): Go models adopt `shopspring/decimal.Decimal`

The first slices are landed: cost calculation no longer does token/price math with raw `float64`, the old `roundCost` transition helper has been removed, the balance mutation paths for usage deduction, recharge fulfillment, subscription balance payment, onboarding credit, and redeem-code credit now do decimal add/subtract against `User.Balance`, and user/model/plan/usage/order/transaction/budget/coupon/redeem/limit persisted amount fields are decimal-native. GraphQL money responses use the public `Money` scalar; deprecated REST payment DTOs still expose JSON numbers for backward compatibility, generated from decimal values.

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

Continue through the remaining model/API fields in this order:

1. Public API boundary: `Money` scalar/DTO string responses are now in place for primary persisted money fields.
2. Internal DTO/cache fields: billing usage summary structs are decimal-native, Redis cost counters use `cost_units` / `total_cost_units` fixed at `MoneyScale`, and latency cache sums use integer milliseconds. Old `cost_usd` Redis values are read as a fallback but are not the canonical counter.
3. Remaining cleanup is API-design work: decide whether deprecated REST payment endpoints should eventually move from JSON numbers to string money responses. This is intentionally separate because it changes response shape for old clients.

### Phase 3: GraphQL Money scalar

`scalar Money` is now wired through gqlgen as a decimal-string scalar. Frontend codegen maps `Money` outputs to `string` and inputs to `string | number`, with formatting helpers normalizing display-time values.

Next, treat deprecated REST payment endpoint response shape as API-design cleanup. The primary management surface already speaks decimal/`Money`, and the internal billing/quota/anomaly paths no longer depend on float arithmetic for money.

## Why not flag-day everything

The Phase 1 SQL migration is safe to land in isolation because PostgreSQL accepts NUMERIC values from a Go `float64` writer (the GORM driver converts) and returns NUMERIC values to a Go `float64` reader (lossy, but no worse than before). The Phase 2 + 3 work is much wider — every billing-touching test must update — and benefits from a separate review cycle.

## Why decimal.js / bignumber.js on the frontend

JavaScript's `Number` is float64 with the same representation pitfalls. If we return a JSON number from the backend, the frontend's `0.1 + 0.2 !== 0.3` problem is back at the UI layer. Returning Money as a string and parsing into `decimal.js` keeps the precision intact.
