package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// CmReportHandler serves the CM report aggregate, nested under work orders,
// PM reports, panels and panel devices.
type CmReportHandler struct {
	svc *service.CmReportService
}

// Save handles PUT /work-orders/{id}/cm-report.
func (h *CmReportHandler) Save(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CmReportSaveInput
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

// GetForWorkOrder handles GET /work-orders/{id}/cm-report.
func (h *CmReportHandler) GetForWorkOrder(w http.ResponseWriter, r *http.Request) {
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

// Submit handles POST /work-orders/{id}/cm-report/submit.
func (h *CmReportHandler) Submit(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CmReportSubmitInput
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

// ListHistoryByWorkOrder handles GET /work-orders/{id}/cm-reports.
func (h *CmReportHandler) ListHistoryByWorkOrder(w http.ResponseWriter, r *http.Request) {
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

// CreateOnsiteFix handles POST /pm-reports/{id}/onsite-fixes.
func (h *CmReportHandler) CreateOnsiteFix(w http.ResponseWriter, r *http.Request) {
	pmReportID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CmReportOnsiteInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.CreateOnsiteFix(r.Context(), pmReportID, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, report)
}

// Escalate handles POST /pm-reports/{id}/escalate — "Report an issue" during
// a PM visit when the repair cannot finish on site.
func (h *CmReportHandler) Escalate(w http.ResponseWriter, r *http.Request) {
	pmReportID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CmReportEscalateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.EscalateFromPm(r.Context(), pmReportID, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, report)
}

// ListByPmReport handles GET /pm-reports/{id}/onsite-fixes.
func (h *CmReportHandler) ListByPmReport(w http.ResponseWriter, r *http.Request) {
	pmReportID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, err := h.svc.ListByPmReport(r.Context(), pmReportID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}

// ListHistoryByPanel handles GET /panels/{id}/cm-reports — the repair
// history of a panel.
func (h *CmReportHandler) ListHistoryByPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, service.CmReportHistorySortable(), "created_at")
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

// ListHistoryByPanelDevice handles GET /panel-devices/{id}/cm-reports — the
// repair history of a single device.
func (h *CmReportHandler) ListHistoryByPanelDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, service.CmReportHistorySortable(), "created_at")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.svc.ListHistoryByPanelDevice(r.Context(), deviceID, page)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}

// Get handles GET /cm-reports/{id}.
func (h *CmReportHandler) Get(w http.ResponseWriter, r *http.Request) {
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

// Update handles PUT and PATCH /cm-reports/{id}.
func (h *CmReportHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CmReportUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	report, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, report)
}

// Delete handles DELETE /cm-reports/{id}.
func (h *CmReportHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
