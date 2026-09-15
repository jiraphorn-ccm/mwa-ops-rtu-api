package handler

import (
	"net/http"
	"strings"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// AuditHandler serves GET /audit-logs.
type AuditHandler struct {
	svc *service.AuditService
}

// List handles GET /audit-logs.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, service.AuditLogSortable(), "created_at")
	filter := service.AuditLogListFilter{
		UserID: q.UUID("user_id"),
		From:   q.Time("from"),
		To:     q.Time("to"),
	}
	if action := q.String("action"); action != nil {
		a := strings.ToUpper(strings.TrimSpace(*action))
		filter.Action = &a
	}
	if resource := q.String("resource"); resource != nil {
		filter.Resource = resource
	}
	if err := q.Err(); err != nil {
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
