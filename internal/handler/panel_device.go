package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// PanelDeviceHandler serves /panel-devices and the devices nested under a panel.
type PanelDeviceHandler struct {
	svc *service.PanelDeviceService
}

// List handles GET /panel-devices.
func (h *PanelDeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, nil)
}

// ListByPanel handles GET /panels/{id}/devices.
func (h *PanelDeviceHandler) ListByPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.list(w, r, &panelID)
}

func (h *PanelDeviceHandler) list(w http.ResponseWriter, r *http.Request, panelID *uuid.UUID) {
	page, filter, err := service.ParsePanelDeviceList(r, panelID)
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

// Get handles GET /panel-devices/{id}.
func (h *PanelDeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	device, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, device)
}

// Create handles POST /panel-devices.
func (h *PanelDeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.PanelDeviceCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if in.PanelID == uuid.Nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("panel_id", httpx.IssueRequired, "This field is required."))
		return
	}

	device, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, device)
}

// CreateForPanel handles POST /panels/{id}/devices.
func (h *PanelDeviceHandler) CreateForPanel(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PanelDeviceCreateInput
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

	device, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, device)
}

// Update handles PUT and PATCH /panel-devices/{id}.
func (h *PanelDeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PanelDeviceUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	device, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, device)
}

// RecordStatus handles POST /panel-devices/{id}/status.
func (h *PanelDeviceHandler) RecordStatus(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PanelDeviceStatusInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	device, err := h.svc.RecordStatus(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessStatus, device)
}

// Delete handles DELETE /panel-devices/{id} as a soft delete.
func (h *PanelDeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Restore handles POST /panel-devices/{id}/restore.
func (h *PanelDeviceHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	device, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessRestore, device)
}

// Purge handles DELETE /panel-devices/{id}/permanent.
func (h *PanelDeviceHandler) Purge(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Purge(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: id, Deleted: true})
}
