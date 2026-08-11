package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// NotificationHandler serves /notifications (System Design Screen 06).
// Every route is scoped to the caller's own recipient_id via the
// recipient_id query/body field, matching the actor_id convention used
// elsewhere in this API (no auth-derived identity).
type NotificationHandler struct {
	svc *service.NotificationService
}

// List handles GET /notifications?recipient_id=....
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	recipientID := q.UUID("recipient_id")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if recipientID == nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("recipient_id", httpx.IssueRequired, "This field is required."))
		return
	}

	page, filter, err := service.ParseNotificationList(r, *recipientID)
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

// Get handles GET /notifications/{id}.
func (h *NotificationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	n, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, n)
}

// Create handles POST /notifications. Meant for internal/system use once a
// work order event happens (assignment, submission, approval, ...).
func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.NotificationCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	n, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, n)
}

// recipientBody is the body of the mark-read endpoints.
type recipientBody struct {
	RecipientID uuid.UUID `json:"recipient_id" validate:"required"`
}

func recipientIDFromBody(r *http.Request) (uuid.UUID, error) {
	var in recipientBody
	if _, err := httpx.Bind(r, &in); err != nil {
		return uuid.Nil, err
	}
	return in.RecipientID, nil
}

// MarkRead handles POST /notifications/{id}/read.
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	recipientID, err := recipientIDFromBody(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	n, err := h.svc.MarkRead(r.Context(), id, recipientID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, n)
}

// MarkAllRead handles POST /notifications/read-all.
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	recipientID, err := recipientIDFromBody(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	count, err := h.svc.MarkAllRead(r.Context(), recipientID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, map[string]int64{"updated": count})
}

// CountUnread handles GET /notifications/unread-count?recipient_id=....
func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	recipientID := q.UUID("recipient_id")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if recipientID == nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("recipient_id", httpx.IssueRequired, "This field is required."))
		return
	}

	count, err := h.svc.CountUnread(r.Context(), *recipientID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, map[string]int64{"unread": count})
}

// Delete handles DELETE /notifications/{id}?recipient_id=....
func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	q := httpx.NewQuery(r)
	recipientID := q.UUID("recipient_id")
	if err := q.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if recipientID == nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("recipient_id", httpx.IssueRequired, "This field is required."))
		return
	}

	if err := h.svc.Delete(r.Context(), id, *recipientID); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: id, Deleted: true})
}
