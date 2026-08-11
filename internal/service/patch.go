package service

import (
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/httpx"
)

// patchRequired resolves a PATCH field that maps to a NOT NULL column. It
// reports whether the column should be written and rejects an explicit null.
func patchRequired[T any](fields httpx.FieldSet, name string, value *T) (T, bool, error) {
	var zero T
	if !fields.Has(name) {
		return zero, false, nil
	}
	if value == nil {
		return zero, false, httpx.Err(httpx.ErrValidationFailed).
			WithField(name, httpx.IssueRequired, "This field cannot be null.")
	}
	return *value, true, nil
}

// patchNullable resolves a PATCH field that maps to a nullable column. An
// explicit null clears the column; an absent key leaves it untouched.
func patchNullable[T any](fields httpx.FieldSet, name string, value *T) (*T, bool) {
	if !fields.Has(name) {
		return nil, false
	}
	return value, true
}

// checkDecimalRange validates an optional fixed-point field against inclusive
// bounds and appends a field error when it is out of range.
func checkDecimalRange(appErr *httpx.AppError, field string, value *decimal.Decimal, min, max float64) *httpx.AppError {
	if value == nil {
		return appErr
	}
	lower := decimal.NewFromFloat(min)
	upper := decimal.NewFromFloat(max)
	if value.LessThan(lower) || value.GreaterThan(upper) {
		if appErr == nil {
			appErr = httpx.Err(httpx.ErrValidationFailed)
		}
		appErr.WithField(field, httpx.IssueOutOfRange,
			"Must be between "+decimal.NewFromFloat(min).String()+" and "+decimal.NewFromFloat(max).String()+".")
	}
	return appErr
}

// errOrNil converts an accumulated validation error back into a plain error.
func errOrNil(appErr *httpx.AppError) error {
	if appErr == nil {
		return nil
	}
	return appErr
}
