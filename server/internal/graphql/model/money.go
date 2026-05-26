package model

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"

	appmodels "llm-router-platform/internal/models"
)

// Money is a GraphQL scalar serialized as a decimal string.
type Money string

// NewMoney converts a persisted decimal money value into the GraphQL scalar.
func NewMoney(v decimal.Decimal) Money {
	return Money(v.Round(appmodels.MoneyScale).StringFixed(appmodels.MoneyScale))
}

// NewMoneyPtr converts a decimal value to a pointer for nullable fields.
func NewMoneyPtr(v decimal.Decimal) *Money {
	out := NewMoney(v)
	return &out
}

// Decimal parses the scalar back into a decimal value.
func (m Money) Decimal() (decimal.Decimal, error) {
	return decimal.NewFromString(strings.TrimSpace(string(m)))
}

// MarshalGQL writes Money as a JSON string so clients never receive a lossy
// JSON number for currency values.
func (m Money) MarshalGQL(w io.Writer) {
	graphql.MarshalString(string(m)).MarshalGQL(w)
}

// UnmarshalGQL accepts strings and JSON numbers for compatibility with
// existing clients while normalizing the internal representation.
func (m *Money) UnmarshalGQL(v any) error {
	var d decimal.Decimal
	var err error
	switch value := v.(type) {
	case string:
		d, err = decimal.NewFromString(strings.TrimSpace(value))
	case int:
		d = decimal.NewFromInt(int64(value))
	case int64:
		d = decimal.NewFromInt(value)
	case float64:
		d = decimal.NewFromFloat(value)
	case json.Number:
		d, err = decimal.NewFromString(value.String())
	default:
		return fmt.Errorf("money must be a decimal string or number")
	}
	if err != nil {
		return fmt.Errorf("invalid money value: %w", err)
	}
	*m = NewMoney(d)
	return nil
}
