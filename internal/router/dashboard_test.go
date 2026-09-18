package router

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/handler"
)

func TestDashboardRoutesAreRegistered(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		APIPrefix:          "/api/rtu/v1",
		AuthEnabled:        true,
		AuthJWTSecret:      "01234567890123456789012345678901",
		CORSAllowedOrigins: []string{"*"},
		CORSAllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxBodyBytes:       1 << 20,
		RequestTimeout:     5 * time.Second,
		RateLimitEnabled:   false,
	}
	h := New(Deps{
		Config: cfg,
		Logger: slog.New(slog.DiscardHandler),
		Handlers: &handler.Handlers{
			Health:    &handler.HealthHandler{},
			Dashboard: &handler.DashboardHandler{},
		},
	})

	for _, path := range []string{
		"/api/rtu/v1/dashboard",
		"/api/rtu/v1/dashboard?period=TODAY",
		"/api/rtu/v1//dashboard?period=TODAY",
		"/api/rtu/v1/dashboard/map",
		"/api/rtu/v1/dashboard/stations-at-risk",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: body=%s err=%v", path, rec.Body.String(), err)
		}
		if rec.Code == http.StatusNotFound || body.Code == "E500_002" {
			t.Fatalf("%s: route not registered status=%d code=%s body=%s", path, rec.Code, body.Code, rec.Body.String())
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status=%d code=%s want 401 (route exists, auth required)", path, rec.Code, body.Code)
		}
	}
}
