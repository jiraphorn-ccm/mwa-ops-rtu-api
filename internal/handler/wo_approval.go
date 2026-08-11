package handler

import (
	"net/http"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// ApprovalHandler serves the review decisions nested under a work order.
type ApprovalHandler struct {
	svc *service.ApprovalService
}

// Create handles POST /work-orders/{id}/approvals — reviews the work order's
// current round and drives the resulting workflow (complete, conditional,
// rework or escalate to CM).
func (h *ApprovalHandler) Create(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in service.ApprovalDecisionInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	wo, err := h.svc.Decide(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, wo)
}

// List handles GET /work-orders/{id}/approvals — every decision made across
// all rounds of a work order.
func (h *ApprovalHandler) List(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	approvals, err := h.svc.ListByWorkOrder(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(approvals))
}
