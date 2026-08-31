//go:build integration

package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/handler"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
	"github.com/rtu-api/internal/router"
	"github.com/rtu-api/internal/service"
)

func integrationRouter(t *testing.T) http.Handler {
	t.Helper()
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
	return router.New(router.Deps{Config: cfg, Logger: logger, Handlers: handlers})
}

func TestIntegrationCmDuplicateCreate(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "SCRUT-" + uuid.NewString()[:8],
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, prefix+"/panels", bytes.NewReader(panelBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create panel: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var panelResp struct {
		Data struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &panelResp); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/problem-topics?active=true", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list topics: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var topicsResp struct {
		Data []struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data) == 0 {
		t.Skip("no problem topics seeded")
	}
	topicID := topicsResp.Data[0].ID

	createCM := func() int {
		body, _ := json.Marshal(map[string]any{
			"work_order_type":  "CM",
			"panel_id":         panelResp.Data.ID.String(),
			"problem_topic_id": topicID.String(),
			"requested_by":     actorID.String(),
			"assigned_to":      actorID.String(),
			"assigned_by":      actorID.String(),
			"title":            "CM duplicate test",
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := createCM(); code != http.StatusCreated {
		t.Fatalf("first CM create: status=%d", code)
	}
	rec = httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topicID.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
		"title":            "CM duplicate test 2",
	})
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate CM: want 409 got %d body=%s", rec.Code, rec.Body.String())
	}
	var errResp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatal(err)
	}
	if errResp.Code != httpx.ErrOpenCmDuplicate.Code {
		t.Fatalf("code=%q want %q", errResp.Code, httpx.ErrOpenCmDuplicate.Code)
	}
}
