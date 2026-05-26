package models

import "github.com/shopspring/decimal"

// MoneyScale matches the database NUMERIC(20,8) fractional precision.
const MoneyScale int32 = 8

// MoneyUnitsFactor is the fixed-scale integer multiplier for MoneyScale.
const MoneyUnitsFactor int64 = 100_000_000

// MoneyFromFloat converts a legacy float64 boundary value into the canonical
// decimal representation used by persisted money fields.
func MoneyFromFloat(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v).Round(MoneyScale)
}

// MoneyFromString converts a decimal literal into canonical money.
// Use this for source-code constants so they never pass through binary float.
func MoneyFromString(v string) decimal.Decimal {
	return decimal.RequireFromString(v).Round(MoneyScale)
}

// MoneyToFloat converts a decimal money value back to the current GraphQL/API
// float boundary. Remove this once the public API exposes a Money scalar.
func MoneyToFloat(v decimal.Decimal) float64 {
	out, _ := v.Round(MoneyScale).Float64()
	return out
}

// MoneyToCents converts a decimal USD amount to integer cents for payment
// providers that require the smallest currency subunit.
func MoneyToCents(v decimal.Decimal) int64 {
	return v.Round(MoneyScale).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

// MoneyFromCents converts an integer minor-unit amount into canonical money.
func MoneyFromCents(cents int64) decimal.Decimal {
	return decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)).Round(MoneyScale)
}

// MoneyToUnits converts money to a fixed-scale integer with MoneyScale digits.
func MoneyToUnits(v decimal.Decimal) int64 {
	return v.Round(MoneyScale).Mul(decimal.NewFromInt(MoneyUnitsFactor)).Round(0).IntPart()
}

// MoneyFromUnits converts a fixed-scale integer back to canonical money.
func MoneyFromUnits(units int64) decimal.Decimal {
	return decimal.NewFromInt(units).Div(decimal.NewFromInt(MoneyUnitsFactor)).Round(MoneyScale)
}

// MoneyRoundToCents normalizes money to the two-decimal precision supported by
// card and QR-code payment providers.
func MoneyRoundToCents(v decimal.Decimal) decimal.Decimal {
	return MoneyFromCents(MoneyToCents(v))
}

// MoneyAdd adds one persisted decimal money value to another.
func MoneyAdd(left, right decimal.Decimal) decimal.Decimal {
	return left.Add(right).Round(MoneyScale)
}

// MoneySub subtracts one persisted decimal money value from another.
func MoneySub(left, right decimal.Decimal) decimal.Decimal {
	return left.Sub(right).Round(MoneyScale)
}

// MoneyNeg returns the negated decimal money value at the database scale.
func MoneyNeg(v decimal.Decimal) decimal.Decimal {
	return v.Neg().Round(MoneyScale)
}
