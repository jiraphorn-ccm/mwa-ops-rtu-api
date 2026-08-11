package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// PanelHandler serves /panels.
type PanelHandler struct {
	svc *service.PanelService
}

// List handles GET /panels.
func (h *PanelHandler) List(w http.ResponseWriter, r *http.Request) {
	page, filter, err := service.ParsePanelList(r)
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

// Get handles GET /panels/{id}.
func (h *PanelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	panel, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, panel)
}

// GetByCode handles GET /panels/code/{code}.
func (h *PanelHandler) GetByCode(w http.ResponseWriter, r *http.Request) {
	panel, err := h.svc.GetByCode(r.Context(), urlParam(r, "code"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, panel)
}

// Create handles POST /panels.
func (h *PanelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.PanelCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	panel, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, panel)
}

// Update handles PUT and PATCH /panels/{id}.
func (h *PanelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.PanelUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	panel, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, panel)
}

// Delete handles DELETE /panels/{id} as a soft delete.
func (h *PanelHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Restore handles POST /panels/{id}/restore.
func (h *PanelHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	panel, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessRestore, panel)
}

// Purge handles DELETE /panels/{id}/permanent.
func (h *PanelHandler) Purge(w http.ResponseWriter, r *http.Request) {
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
