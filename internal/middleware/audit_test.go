package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
)

type memSink struct {
	mu      sync.Mutex
	entries []AuditEntry
}

func (m *memSink) WriteAudit(_ context.Context, e AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

func TestDeriveAction(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"POST /api/rtu/v1/panels":                 "CREATE",
		"PATCH /api/rtu/v1/panels/x":              "UPDATE",
		"DELETE /api/rtu/v1/users/x":              "DELETE",
		"POST /api/rtu/v1/users/x/restore":        "RESTORE",
		"DELETE /api/rtu/v1/users/x/permanent":    "PURGE",
		"POST /api/rtu/v1/work-orders/x/check-in": "CHECK_IN",
		"POST /api/rtu/v1/auth/change-password":   "CHANGE_PASSWORD",
	}
	for in, want := range cases {
		method, path, _ := splitMethodPath(in)
		if got := deriveAction(method, path); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
}

func splitMethodPath(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return s[:i], s[i+1:], true
		}
	}
	return "", s, false
}

func TestAuditRecordsMutationWithActor(t *testing.T) {
	sink := &memSink{}
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	h := Audit(nil, sink)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WithAuth(r.Context(), httpx.AuthInfo{UserID: uid.String(), Subject: uid.String()})
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/rtu/v1/panels", nil)
	req = req.WithContext(httpx.EnsureAuthSlot(req.Context()))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sink.mu.Lock()
		n := len(sink.entries)
		sink.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.entries) != 1 {
		t.Fatalf("entries=%d", len(sink.entries))
	}
	e := sink.entries[0]
	if e.Action != "CREATE" || e.StatusCode != http.StatusCreated {
		t.Fatalf("entry=%+v", e)
	}
	if e.UserID == nil || *e.UserID != uid {
		t.Fatalf("user_id=%v", e.UserID)
	}
}

func TestAuditSkipsGET(t *testing.T) {
	sink := &memSink{}
	h := Audit(nil, sink)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/rtu/v1/panels", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	time.Sleep(50 * time.Millisecond)
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.entries) != 0 {
		t.Fatalf("GET should not be audited: %+v", sink.entries)
	}
}
