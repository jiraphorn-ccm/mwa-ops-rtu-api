package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// CalibrationHandler serves /calibrations and /calibration-readings.
type CalibrationHandler struct {
	svc *service.CalibrationService
}

// List handles GET /calibrations.
func (h *CalibrationHandler) List(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, nil)
}

// ListByDevice handles GET /panel-devices/{id}/calibrations.
func (h *CalibrationHandler) ListByDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.list(w, r, &deviceID)
}

func (h *CalibrationHandler) list(w http.ResponseWriter, r *http.Request, deviceID *uuid.UUID) {
	page, filter, err := service.ParseCalibrationList(r, deviceID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if filter.PerformedFrom != nil && filter.PerformedTo != nil && filter.PerformedTo.Before(*filter.PerformedFrom) {
		httpx.Error(w, r, httpx.Err(httpx.ErrDateRangeInvalid).
			WithField("performed_to", httpx.IssueInvalid, "Must be later than performed_from."))
		return
	}

	items, total, err := h.svc.List(r.Context(), page, filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}

// Get handles GET /calibrations/{id} and includes the measurement sheet.
func (h *CalibrationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	detail, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, detail)
}

// Create handles POST /calibrations.
func (h *CalibrationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.CalibrationCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if in.PanelDeviceID == uuid.Nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("panel_device_id", httpx.IssueRequired, "This field is required."))
		return
	}

	detail, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, detail)
}

// CreateForDevice handles POST /panel-devices/{id}/calibrations.
func (h *CalibrationHandler) CreateForDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CalibrationCreateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if fields.Has("panel_device_id") && in.PanelDeviceID != deviceID {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("panel_device_id", httpx.IssueInvalid, "Must match the device in the URL."))
		return
	}
	in.PanelDeviceID = deviceID

	detail, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, detail)
}

// Update handles PUT and PATCH /calibrations/{id}.
func (h *CalibrationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CalibrationUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	detail, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, detail)
}

// Delete handles DELETE /calibrations/{id}. Readings cascade with it.
func (h *CalibrationHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Summary handles GET /calibrations/summary.
func (h *CalibrationHandler) Summary(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	deviceID := q.UUID("panel_device_id")
	from := q.Time("performed_from")
	to := q.Time("performed_to")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}

	summary, err := h.svc.ResultSummary(r.Context(), deviceID, from, to)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessSummary, map[string]any{"by_result": summary})
}

// ListReadings handles GET /calibrations/{id}/readings.
func (h *CalibrationHandler) ListReadings(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	readings, err := h.svc.ListReadings(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(readings))
}

// AddReading handles POST /calibrations/{id}/readings.
func (h *CalibrationHandler) AddReading(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.CalibrationReadingInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	reading, err := h.svc.AddReading(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, reading)
}

// ReplaceReadings handles PUT /calibrations/{id}/readings.
func (h *CalibrationHandler) ReplaceReadings(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.ReadingSheetInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	readings, err := h.svc.ReplaceReadings(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, httpx.NewCollection(readings))
}

// GetReading handles GET /calibration-readings/{id}.
func (h *CalibrationHandler) GetReading(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	reading, err := h.svc.GetReading(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, reading)
}

// GetReadingByCalibration handles GET /calibrations/{id}/readings/{readingId}.
func (h *CalibrationHandler) GetReadingByCalibration(w http.ResponseWriter, r *http.Request) {
	calibrationID, readingID, err := h.calibrationReadingIDs(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	reading, err := h.svc.GetReadingForCalibration(r.Context(), calibrationID, readingID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, reading)
}

// UpdateReading handles PUT and PATCH /calibration-readings/{id}.
func (h *CalibrationHandler) UpdateReading(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.ReadingUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	reading, err := h.svc.UpdateReading(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, reading)
}

// UpdateReadingByCalibration handles PUT and PATCH /calibrations/{id}/readings/{readingId}.
func (h *CalibrationHandler) UpdateReadingByCalibration(w http.ResponseWriter, r *http.Request) {
	calibrationID, readingID, err := h.calibrationReadingIDs(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.ReadingUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	reading, err := h.svc.UpdateReadingForCalibration(r.Context(), calibrationID, readingID, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, reading)
}

// DeleteReading handles DELETE /calibration-readings/{id}.
func (h *CalibrationHandler) DeleteReading(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.DeleteReading(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: id, Deleted: true})
}

// DeleteReadingByCalibration handles DELETE /calibrations/{id}/readings/{readingId}.
func (h *CalibrationHandler) DeleteReadingByCalibration(w http.ResponseWriter, r *http.Request) {
	calibrationID, readingID, err := h.calibrationReadingIDs(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.DeleteReadingForCalibration(r.Context(), calibrationID, readingID); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: readingID, Deleted: true})
}

func (h *CalibrationHandler) calibrationReadingIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	calibrationID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	readingID, err := httpx.UUIDParam(r, "readingId")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return calibrationID, readingID, nil
}
