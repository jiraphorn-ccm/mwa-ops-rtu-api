package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// PmReportHandler serves the PM report aggregate, nested under work orders
// and panels.
type PmReportHandler struct {
	svc *service.PmReportService
}

// Save handles PUT /work-orders/{id}/pm-report.
func (h *PmReportHandler) Save(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PmReportSaveInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.SaveForWorkOrder(r.Context(), workOrderID, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, report)
}

// GetForWorkOrder handles GET /work-orders/{id}/pm-report.
func (h *PmReportHandler) GetForWorkOrder(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.GetForWorkOrder(r.Context(), workOrderID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, report)
}

// Submit handles POST /work-orders/{id}/pm-report/submit.
func (h *PmReportHandler) Submit(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PmReportSubmitInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.Submit(r.Context(), workOrderID, in.ActorID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, report)
}

// ListHistoryByWorkOrder handles GET /work-orders/{id}/pm-reports — the
// report of every round of this work order (i.e. rework history).
func (h *PmReportHandler) ListHistoryByWorkOrder(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, err := h.svc.ListHistoryByWorkOrder(r.Context(), workOrderID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}

// ListHistoryByPanel handles GET /panels/{id}/pm-reports — the PM history of
// a panel, across every work order ever opened against it.
func (h *PmReportHandler) ListHistoryByPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, service.PmReportHistorySortable(), "report_date")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.svc.ListHistoryByPanel(r.Context(), panelID, page)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}

// Get handles GET /pm-reports/{id}.
func (h *PmReportHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, report)
}

// Delete handles DELETE /pm-reports/{id}. Only a DRAFT report can be removed.
func (h *PmReportHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: id, Deleted: true})
}
