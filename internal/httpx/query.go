package httpx

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Query reads and validates URL query parameters, collecting every problem so
// the client gets all of them in one E100_004 response.
type Query struct {
	values url.Values
	errs   []FieldError
}

// NewQuery starts reading the query string of a request.
func NewQuery(r *http.Request) *Query {
	return &Query{values: r.URL.Query()}
}

// Has reports whether the parameter was supplied with a non-empty value.
func (q *Query) Has(name string) bool {
	return strings.TrimSpace(q.values.Get(name)) != ""
}

func (q *Query) raw(name string) (string, bool) {
	v := strings.TrimSpace(q.values.Get(name))
	if v == "" {
		return "", false
	}
	return v, true
}

func (q *Query) fail(name, message string) {
	q.errs = append(q.errs, FieldError{Field: name, Issue: IssueInvalid, Message: message})
}

// String returns an optional free-text parameter.
func (q *Query) String(name string) *string {
	v, ok := q.raw(name)
	if !ok {
		return nil
	}
	return &v
}

// Enum returns an optional parameter restricted to a fixed set of values.
func (q *Query) Enum(name string, allowed ...string) *string {
	v, ok := q.raw(name)
	if !ok {
		return nil
	}
	upper := strings.ToUpper(v)
	for _, a := range allowed {
		if upper == a {
			return &upper
		}
	}
	q.fail(name, fmt.Sprintf("Must be one of: %s.", strings.Join(allowed, ", ")))
	return nil
}

// Enums returns zero or more values for a query param. Supports repeated keys
// (?status=A&status=B) and comma-separated values (?status=A,B).
func (q *Query) Enums(name string, allowed ...string) []string {
	rawValues, ok := q.values[name]
	if !ok || len(rawValues) == 0 {
		return nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}
	seen := make(map[string]struct{})
	var out []string
	for _, raw := range rawValues {
		for part := range strings.SplitSeq(raw, ",") {
			v := strings.TrimSpace(part)
			if v == "" {
				continue
			}
			upper := strings.ToUpper(v)
			if _, ok := allowedSet[upper]; !ok {
				q.fail(name, fmt.Sprintf("Must be one of: %s.", strings.Join(allowed, ", ")))
				continue
			}
			if _, ok := seen[upper]; ok {
				continue
			}
			seen[upper] = struct{}{}
			out = append(out, upper)
		}
	}
	return out
}

// Bool returns an optional boolean parameter.
func (q *Query) Bool(name string) *bool {
	v, ok := q.raw(name)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		q.fail(name, "Must be a boolean (true or false).")
		return nil
	}
	return &parsed
}

// UUID returns an optional UUID parameter.
func (q *Query) UUID(name string) *uuid.UUID {
	v, ok := q.raw(name)
	if !ok {
		return nil
	}
	parsed, err := uuid.Parse(v)
	if err != nil {
		q.fail(name, "Must be a valid UUID.")
		return nil
	}
	return &parsed
}

// Time returns an optional timestamp parameter. RFC 3339 and plain dates are
// both accepted; a plain date resolves to midnight UTC.
func (q *Query) Time(name string) *time.Time {
	v, ok := q.raw(name)
	if !ok {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, v); err == nil {
		return &parsed
	}
	if parsed, err := time.Parse(time.DateOnly, v); err == nil {
		return &parsed
	}
	q.fail(name, "Must be an RFC 3339 timestamp or a YYYY-MM-DD date.")
	return nil
}

// Decimal returns an optional fixed-point number parameter.
func (q *Query) Decimal(name string) *decimal.Decimal {
	v, ok := q.raw(name)
	if !ok {
		return nil
	}
	parsed, err := decimal.NewFromString(v)
	if err != nil {
		q.fail(name, "Must be a number.")
		return nil
	}
	return &parsed
}

// Int returns an integer parameter clamped to [min, max], falling back to def.
func (q *Query) Int(name string, def, min, max int) int {
	v, ok := q.raw(name)
	if !ok {
		return def
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		q.fail(name, "Must be an integer.")
		return def
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

// Err returns an E100_004 AppError when any parameter failed to parse.
func (q *Query) Err() error {
	if len(q.errs) == 0 {
		return nil
	}
	return Err(ErrInvalidQuery).WithFields(q.errs...)
}
