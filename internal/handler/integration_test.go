//go:build integration

package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/handler"
	"github.com/rtu-api/internal/repository"
	"github.com/rtu-api/internal/router"
	"github.com/rtu-api/internal/service"
)

func TestIntegrationHealthReady(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skip("config not available:", err)
	}
	if cfg.DatabaseURL == "" {
		t.Skip("database not configured")
	}
	logger := cfg.Logger()

	ctx := t.Context()
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	store := repository.New(pool)
	services := service.New(store, nil, cfg.S3AppPrefix)
	handlers := handler.New(cfg, services, handler.NewHealthHandler(cfg, pool, "test"))
	h := router.New(router.Deps{Config: cfg, Logger: logger, Handlers: handlers})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
