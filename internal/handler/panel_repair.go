package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// PanelRepairHandler serves App-facing panel repair endpoints.
type PanelRepairHandler struct {
	cm *service.CmReportService
}

// OpenOnsite handles POST /panels/{id}/repairs/onsite.
func (h *PanelRepairHandler) OpenOnsite(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PanelRepairOnsiteInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	out, err := h.cm.OpenOnsiteRepairFromPanel(r.Context(), panelID, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, out)
}

// Escalate handles POST /panels/{id}/repairs/escalate.
func (h *PanelRepairHandler) Escalate(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PanelRepairEscalateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	out, err := h.cm.EscalateRepairFromPanel(r.Context(), panelID, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, out)
}

// ListHistory handles GET /panels/{id}/repair-history.
func (h *PanelRepairHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, service.CmReportHistorySortable(), "created_at")
	filter, err := service.ParseRepairHistoryFilter(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.cm.ListHistoryByPanel(r.Context(), panelID, page, filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}

// ListActivity handles GET /panels/{id}/repair-activity.
func (h *PanelRepairHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, httpx.Sortable{"created_at": "wal.created_at"}, "created_at")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.cm.ListRepairActivityByPanel(r.Context(), panelID, page)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}
