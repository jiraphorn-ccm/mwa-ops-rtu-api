package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// urlParam reads a raw path parameter.
func urlParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

// DeviceModelHandler serves /device-models.
type DeviceModelHandler struct {
	svc *service.DeviceModelService
}

// List handles GET /device-models.
func (h *DeviceModelHandler) List(w http.ResponseWriter, r *http.Request) {
	page, filter, err := service.ParseDeviceModelList(r)
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

// Get handles GET /device-models/{id}.
func (h *DeviceModelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	model, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, model)
}

// GetByCode handles GET /device-models/code/{code}.
func (h *DeviceModelHandler) GetByCode(w http.ResponseWriter, r *http.Request) {
	model, err := h.svc.GetByCode(r.Context(), urlParam(r, "code"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, model)
}

// Create handles POST /device-models.
func (h *DeviceModelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.DeviceModelCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	model, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, model)
}

// Update handles PUT and PATCH /device-models/{id}.
func (h *DeviceModelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.DeviceModelUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	model, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, model)
}

// Delete handles DELETE /device-models/{id} as a soft delete.
func (h *DeviceModelHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Restore handles POST /device-models/{id}/restore.
func (h *DeviceModelHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	model, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessRestore, model)
}

// Purge handles DELETE /device-models/{id}/permanent.
func (h *DeviceModelHandler) Purge(w http.ResponseWriter, r *http.Request) {
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
