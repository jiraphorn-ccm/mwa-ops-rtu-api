package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// ChecklistItemHandler serves /checklist-items.
type ChecklistItemHandler struct {
	svc *service.ChecklistItemService
}

// List handles GET /checklist-items.
func (h *ChecklistItemHandler) List(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	active := q.Bool("active")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, err := h.svc.List(r.Context(), active)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}

// Get handles GET /checklist-items/{id}.
func (h *ChecklistItemHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	item, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, item)
}

// Create handles POST /checklist-items.
func (h *ChecklistItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.ChecklistItemCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	item, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, item)
}

// Update handles PUT and PATCH /checklist-items/{id}.
func (h *ChecklistItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.ChecklistItemUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	item, err := h.svc.Update(r.Context(), id, fields, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, item)
}

// Delete handles DELETE /checklist-items/{id} as a soft delete.
func (h *ChecklistItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Restore handles POST /checklist-items/{id}/restore.
func (h *ChecklistItemHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	item, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessRestore, item)
}

// Purge handles DELETE /checklist-items/{id}/permanent.
func (h *ChecklistItemHandler) Purge(w http.ResponseWriter, r *http.Request) {
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
