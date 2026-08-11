package httpx

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// dateLayout is the wire format for SQL DATE columns. Using RFC3339 for a
// date-only value forces every client to invent a fake time-of-day, so DATE
// fields get their own type instead of sharing time.Time's JSON codec.
const dateLayout = "2006-01-02"

var dateGoType = reflect.TypeOf(Date{})

// Date is the JSON wire type for SQL DATE columns (as opposed to
// TIMESTAMPTZ, which uses time.Time directly). It marshals as "YYYY-MM-DD"
// and only accepts that format on the way in.
type Date struct {
	time.Time
}

// NewDate truncates t to a calendar date in UTC.
func NewDate(t time.Time) Date {
	return Date{time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(dateLayout))
}

// UnmarshalJSON returns *json.UnmarshalTypeError on any format mismatch so
// the request-binding layer (decodeError) can report the offending field
// instead of failing the whole body with an opaque "invalid body" error.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return &json.UnmarshalTypeError{Value: string(data), Type: dateGoType}
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return &json.UnmarshalTypeError{Value: "string " + s, Type: dateGoType}
	}
	d.Time = t
	return nil
}

// AsTime converts an optional Date into the *time.Time the repository layer
// expects for a DATE column, preserving nil.
func (d *Date) AsTime() *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

// Scan implements pgx/database/sql scanning for PostgreSQL DATE columns.
func (d *Date) Scan(src any) error {
	if src == nil {
		d.Time = time.Time{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		d.Time = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
		return nil
	case string:
		t, err := time.Parse(dateLayout, v)
		if err != nil {
			return fmt.Errorf("httpx.Date.Scan: %w", err)
		}
		d.Time = t
		return nil
	case []byte:
		t, err := time.Parse(dateLayout, string(v))
		if err != nil {
			return fmt.Errorf("httpx.Date.Scan: %w", err)
		}
		d.Time = t
		return nil
	default:
		return fmt.Errorf("httpx.Date.Scan: unsupported type %T", src)
	}
}

// Value implements driver.Valuer for PostgreSQL DATE columns.
func (d Date) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Format(dateLayout), nil
}
