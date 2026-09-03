package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestQueryEnums(t *testing.T) {
	t.Run("repeated keys", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?status=ASSIGNED&status=IN_PROGRESS", nil)
		q := NewQuery(req)
		got := q.Enums("status", "ASSIGNED", "IN_PROGRESS", "PENDING")
		if err := q.Err(); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0] != "ASSIGNED" || got[1] != "IN_PROGRESS" {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("comma separated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?status=ASSIGNED,PENDING", nil)
		q := NewQuery(req)
		got := q.Enums("status", "ASSIGNED", "IN_PROGRESS", "PENDING")
		if err := q.Err(); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("invalid value", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?status=NOPE", nil)
		q := NewQuery(req)
		_ = q.Enums("status", "ASSIGNED")
		if q.Err() == nil {
			t.Fatal("expected error")
		}
	})
}
