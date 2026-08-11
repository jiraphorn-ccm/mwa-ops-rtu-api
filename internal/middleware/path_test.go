package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePath_collapsesDuplicateSlashes(t *testing.T) {
	t.Parallel()

	var got string
	h := NormalizePath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "//api/rtu/v1/panels", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got != "/api/rtu/v1/panels" {
		t.Fatalf("path = %q, want /api/rtu/v1/panels", got)
	}
}
