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
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) == 0 {
		t.Skip("no problem topics seeded")
	}
	topicID := topicsResp.Data.Items[0].ID

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
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders?work_order_type=CM&panel_id="+panelResp.Data.ID.String(), nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list CM: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Data struct {
			Items []struct {
				ProblemTopics []struct {
					ID uuid.UUID `json:"id"`
				} `json:"problem_topics"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data.Items) == 0 || len(listResp.Data.Items[0].ProblemTopics) == 0 || listResp.Data.Items[0].ProblemTopics[0].ID != topicID {
		t.Fatalf("list CM missing problem_topics: %+v", listResp.Data.Items)
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

func TestIntegrationCmMultiTopicCreate(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "MULTI-" + uuid.NewString()[:8],
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
		t.Fatalf("list topics: status=%d", rec.Code)
	}
	var topicsResp struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) < 2 {
		t.Skip("need at least 2 problem topics seeded")
	}
	topic1 := topicsResp.Data.Items[0].ID
	topic2 := topicsResp.Data.Items[1].ID

	body, _ := json.Marshal(map[string]any{
		"work_order_type":   "CM",
		"panel_id":          panelResp.Data.ID.String(),
		"problem_topic_ids": []string{topic1.String(), topic2.String()},
		"requested_by":      actorID.String(),
		"assigned_to":       actorID.String(),
		"assigned_by":       actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("multi-topic CM: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders?work_order_type=CM&panel_id="+panelResp.Data.ID.String(), nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list CM: status=%d", rec.Code)
	}
	var listResp struct {
		Data struct {
			Items []struct {
				ProblemTopics []struct {
					ID uuid.UUID `json:"id"`
				} `json:"problem_topics"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data.Items) == 0 || len(listResp.Data.Items[0].ProblemTopics) != 2 {
		t.Fatalf("want 2 problem_topics, got %+v", listResp.Data.Items)
	}

	dupBody, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topic2.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(dupBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate on second topic: want 409 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIntegrationOpenCmWorkOrdersList(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "OPENCM-" + uuid.NewString()[:8],
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
		t.Fatalf("list topics: status=%d", rec.Code)
	}
	var topicsResp struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) == 0 {
		t.Skip("no problem topics seeded")
	}
	topicID := topicsResp.Data.Items[0].ID

	cmBody, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topicID.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(cmBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create CM: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/panels/"+panelResp.Data.ID.String()+"/open-cm-work-orders", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("open-cm list: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var openResp struct {
		Data struct {
			Items []struct {
				WorkOrderNo   string `json:"work_order_no"`
				ProblemTopics []struct {
					ID uuid.UUID `json:"id"`
				} `json:"problem_topics"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &openResp); err != nil {
		t.Fatal(err)
	}
	if len(openResp.Data.Items) == 0 {
		t.Fatalf("expected open CM items, got %+v", openResp.Data)
	}
	if len(openResp.Data.Items[0].ProblemTopics) == 0 || openResp.Data.Items[0].ProblemTopics[0].ID != topicID {
		t.Fatalf("open-cm missing problem_topics: %+v", openResp.Data.Items[0])
	}
}

func TestIntegrationCmReportTopicSyncAddOnly(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "SYNC-" + uuid.NewString()[:8],
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
		t.Fatalf("list topics: status=%d", rec.Code)
	}
	var topicsResp struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) < 2 {
		t.Skip("need at least 2 problem topics seeded")
	}
	topic1 := topicsResp.Data.Items[0].ID
	topic2 := topicsResp.Data.Items[1].ID

	cmBody, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topic1.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(cmBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create CM: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var woResp struct {
		Data struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &woResp); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders/"+woResp.Data.ID.String()+"/cm-report", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get cm-report: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var reportResp struct {
		Data struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reportResp); err != nil {
		t.Fatal(err)
	}

	patchBody, _ := json.Marshal(map[string]any{
		"problem_topic_id": topic2.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, prefix+"/cm-reports/"+reportResp.Data.ID.String(), bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch cm-report topic: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders/"+woResp.Data.ID.String(), nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get work order: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var getResp struct {
		Data struct {
			ProblemTopics []struct {
				ID uuid.UUID `json:"id"`
			} `json:"problem_topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatal(err)
	}
	if len(getResp.Data.ProblemTopics) != 2 {
		t.Fatalf("want 2 problem_topics after sync, got %+v", getResp.Data.ProblemTopics)
	}
	seen := map[uuid.UUID]bool{}
	for _, pt := range getResp.Data.ProblemTopics {
		seen[pt.ID] = true
	}
	if !seen[topic1] || !seen[topic2] {
		t.Fatalf("want topics %s and %s, got %+v", topic1, topic2, getResp.Data.ProblemTopics)
	}
}

func TestIntegrationCmPatchProblemTopics(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "PATCH-" + uuid.NewString()[:8],
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
		t.Fatalf("list topics: status=%d", rec.Code)
	}
	var topicsResp struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) < 2 {
		t.Skip("need at least 2 problem topics seeded")
	}
	topic1 := topicsResp.Data.Items[0].ID
	topic2 := topicsResp.Data.Items[1].ID

	cmBody, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topic1.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(cmBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create CM: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var woResp struct {
		Data struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &woResp); err != nil {
		t.Fatal(err)
	}

	patchBody, _ := json.Marshal(map[string]any{
		"problem_topic_ids": []string{topic1.String(), topic2.String()},
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, prefix+"/work-orders/"+woResp.Data.ID.String(), bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch topics: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders/"+woResp.Data.ID.String(), nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get work order: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var getResp struct {
		Data struct {
			ProblemTopics []struct {
				ID uuid.UUID `json:"id"`
			} `json:"problem_topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatal(err)
	}
	if len(getResp.Data.ProblemTopics) != 2 {
		t.Fatalf("want 2 topics after patch, got %+v", getResp.Data.ProblemTopics)
	}
}

func TestIntegrationCmReportPutMultiTopic(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "PUTCM-" + uuid.NewString()[:8],
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, prefix+"/panels", bytes.NewReader(panelBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create panel: status=%d", rec.Code)
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
	var topicsResp struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) < 2 {
		t.Skip("need at least 2 problem topics")
	}
	topic1 := topicsResp.Data.Items[0].ID
	topic2 := topicsResp.Data.Items[1].ID

	cmBody, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topic1.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(cmBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create CM: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var woResp struct {
		Data struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &woResp); err != nil {
		t.Fatal(err)
	}

	putBody, _ := json.Marshal(map[string]any{
		"problem_topic_ids": []string{topic1.String(), topic2.String()},
		"problem_detail":    "multi topic report",
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, prefix+"/work-orders/"+woResp.Data.ID.String()+"/cm-report", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put cm-report: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders/"+woResp.Data.ID.String(), nil)
	h.ServeHTTP(rec, req)
	var getResp struct {
		Data struct {
			ProblemTopics []struct {
				ID uuid.UUID `json:"id"`
			} `json:"problem_topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatal(err)
	}
	if len(getResp.Data.ProblemTopics) != 2 {
		t.Fatalf("want 2 topics on WO after PUT cm-report, got %+v", getResp.Data.ProblemTopics)
	}
}

func TestIntegrationWorkOrderListMultiStatus(t *testing.T) {
	h := integrationRouter(t)
	prefix := "/api/rtu/v1"
	actorID := uuid.New()

	panelBody, _ := json.Marshal(map[string]any{
		"code": "STFL-" + uuid.NewString()[:8],
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, prefix+"/panels", bytes.NewReader(panelBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create panel: %d", rec.Code)
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
	var topicsResp struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &topicsResp); err != nil {
		t.Fatal(err)
	}
	if len(topicsResp.Data.Items) == 0 {
		t.Skip("no topics")
	}
	topicID := topicsResp.Data.Items[0].ID

	cmBody, _ := json.Marshal(map[string]any{
		"work_order_type":  "CM",
		"panel_id":         panelResp.Data.ID.String(),
		"problem_topic_id": topicID.String(),
		"requested_by":     actorID.String(),
		"assigned_to":      actorID.String(),
		"assigned_by":      actorID.String(),
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, prefix+"/work-orders", bytes.NewReader(cmBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create CM: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, prefix+"/work-orders?panel_id="+panelResp.Data.ID.String()+"&status=ASSIGNED&status=IN_PROGRESS", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list multi status: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Data struct {
			Items []struct {
				Status string `json:"status"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data.Items) == 0 {
		t.Fatalf("expected at least one ASSIGNED CM")
	}
}
