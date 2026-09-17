package handler

import (
	"net/http"
	"time"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// DashboardHandler serves GET /dashboard and related executive widgets.
type DashboardHandler struct {
	svc *service.DashboardService
}

// Overview handles GET /dashboard?period=TODAY|7D|MONTH|YEAR.
func (h *DashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	period, err := service.ParseDashboardPeriod(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	data, err := h.svc.Overview(r.Context(), period, time.Now())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessSummary, data)
}

// StationsAtRisk handles GET /dashboard/stations-at-risk.
func (h *DashboardHandler) StationsAtRisk(w http.ResponseWriter, r *http.Request) {
	limit, err := service.ParseDashboardRiskLimit(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, err := h.svc.StationsAtRisk(r.Context(), limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}

// Map handles GET /dashboard/map.
func (h *DashboardHandler) Map(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.MapStations(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
}
