package httpx_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rtu-api/internal/httpx"
)

func TestDateJSONRoundTrip(t *testing.T) {
	raw := `"2026-01-10"`
	var d httpx.Date
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != raw {
		t.Fatalf("got %s want %s", out, raw)
	}
}

func TestDateRejectsTimestampInput(t *testing.T) {
	var d httpx.Date
	err := json.Unmarshal([]byte(`"2026-01-10T00:00:00Z"`), &d)
	if err == nil {
		t.Fatal("expected RFC3339 timestamp to be rejected")
	}
}

func TestDateScanFromTime(t *testing.T) {
	var d httpx.Date
	src := time.Date(2026, 1, 10, 15, 30, 0, 0, time.UTC)
	if err := d.Scan(src); err != nil {
		t.Fatalf("scan: %v", err)
	}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `"2026-01-10"` {
		t.Fatalf("got %s", out)
	}
}

func TestDateAsTimeNilPointer(t *testing.T) {
	var d *httpx.Date
	if d.AsTime() != nil {
		t.Fatal("nil Date AsTime should be nil")
	}
}
