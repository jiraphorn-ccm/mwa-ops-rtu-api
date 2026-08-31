package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// WorkOrderHandler serves /work-orders and the work orders nested under a
// panel or panel device.
type WorkOrderHandler struct {
	svc *service.WorkOrderService
}

// List handles GET /work-orders.
func (h *WorkOrderHandler) List(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, nil, nil)
}

// ListByPanel handles GET /panels/{id}/work-orders.
func (h *WorkOrderHandler) ListByPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.list(w, r, &panelID, nil)
}

// ListByPanelDevice handles GET /panel-devices/{id}/work-orders.
func (h *WorkOrderHandler) ListByPanelDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.list(w, r, nil, &deviceID)
}

func (h *WorkOrderHandler) list(w http.ResponseWriter, r *http.Request, panelID, panelDeviceID *uuid.UUID) {
	page, filter, err := service.ParseWorkOrderList(r, panelID, panelDeviceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.svc.List(r.Context(), page, filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}

// Get handles GET /work-orders/{id}.
func (h *WorkOrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, wo)
}

// Create handles POST /work-orders.
func (h *WorkOrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.WorkOrderCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if in.PanelID == uuid.Nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("panel_id", httpx.IssueRequired, "This field is required."))
		return
	}

	wo, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, wo)
}

// CreateForPanel handles POST /panels/{id}/work-orders.
func (h *WorkOrderHandler) CreateForPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.WorkOrderCreateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if fields.Has("panel_id") && in.PanelID != panelID {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("panel_id", httpx.IssueInvalid, "Must match the panel in the URL."))
		return
	}
	in.PanelID = panelID

	wo, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, wo)
}

// Update handles PUT and PATCH /work-orders/{id}.
func (h *WorkOrderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.WorkOrderUpdateInput
	fields, err := httpx.BindLenient(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, wo)
}

// Delete handles DELETE /work-orders/{id} as a cancel (soft delete).
func (h *WorkOrderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if _, err := h.svc.SoftDelete(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Deleted{ID: id, SoftDeleted: true})
}

// Restore handles POST /work-orders/{id}/restore.
func (h *WorkOrderHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessRestore, wo)
}

// Reassign handles POST /work-orders/{id}/reassign.
func (h *WorkOrderHandler) Reassign(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.ReassignInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.Reassign(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessStatus, wo)
}

// CheckIn handles POST /work-orders/{id}/check-in.
func (h *WorkOrderHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CheckInInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.CheckIn(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessStatus, wo)
}

// CheckOut handles POST /work-orders/{id}/check-out.
func (h *WorkOrderHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CheckOutInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.CheckOut(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessStatus, wo)
}

// ListRounds handles GET /work-orders/{id}/rounds — the full history of
// every visit/rework attempt on this work order.
func (h *WorkOrderHandler) ListRounds(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	rounds, err := h.svc.ListRounds(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(rounds))
}

// ListActivity handles GET /work-orders/{id}/activity — the status/
// assignment timeline (opened/assigned/started/pending approval/rejected).
func (h *WorkOrderHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	logs, err := h.svc.ListActivity(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(logs))
}

// ListOpenCmByPanel handles GET /panels/{id}/open-cm-work-orders.
func (h *WorkOrderHandler) ListOpenCmByPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	filter, err := service.ParseOpenCmWorkOrderFilter(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, err := h.svc.ListOpenCmForPanel(r.Context(), panelID, filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}

// ListOpenCmByWorkOrder handles GET /work-orders/{id}/open-cm-work-orders.
func (h *WorkOrderHandler) ListOpenCmByWorkOrder(w http.ResponseWriter, r *http.Request) {
	workOrderID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	filter, err := service.ParseOpenCmWorkOrderFilter(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, err := h.svc.ListOpenCmForWorkOrder(r.Context(), workOrderID, filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}
