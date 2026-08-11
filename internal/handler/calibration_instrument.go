package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// CalibrationInstrumentHandler serves /calibration-instruments.
type CalibrationInstrumentHandler struct {
	svc *service.CalibrationInstrumentService
}

// List handles GET /calibration-instruments.
func (h *CalibrationInstrumentHandler) List(w http.ResponseWriter, r *http.Request) {
	page, filter, err := service.ParseCalibrationInstrumentList(r)
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

// Get handles GET /calibration-instruments/{id}.
func (h *CalibrationInstrumentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	instrument, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, instrument)
}

// Create handles POST /calibration-instruments.
func (h *CalibrationInstrumentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.CalibrationInstrumentCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	instrument, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, instrument)
}

// Update handles PUT and PATCH /calibration-instruments/{id}.
func (h *CalibrationInstrumentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CalibrationInstrumentUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	instrument, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, instrument)
}

// Delete handles DELETE /calibration-instruments/{id} as a soft delete.
func (h *CalibrationInstrumentHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Restore handles POST /calibration-instruments/{id}/restore.
func (h *CalibrationInstrumentHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	instrument, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessRestore, instrument)
}

// Purge handles DELETE /calibration-instruments/{id}/permanent.
func (h *CalibrationInstrumentHandler) Purge(w http.ResponseWriter, r *http.Request) {
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
