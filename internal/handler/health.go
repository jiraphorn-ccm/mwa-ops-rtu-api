package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/httpx"
)

// HealthHandler serves the liveness, readiness and service description
// endpoints. They sit outside the standard envelope so infrastructure probes
// can read them without knowing the MWA conventions.
type HealthHandler struct {
	cfg       *config.Config
	pool      *pgxpool.Pool
	version   string
	startedAt time.Time
}

// NewHealthHandler builds the health handler.
func NewHealthHandler(cfg *config.Config, pool *pgxpool.Pool, version string) *HealthHandler {
	return &HealthHandler{cfg: cfg, pool: pool, version: version, startedAt: time.Now()}
}

type componentStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	Latency string `json:"latency,omitempty"`
}

type healthResponse struct {
	Status     string            `json:"status"`
	Env        string            `json:"env"`
	Version    string            `json:"version"`
	UptimeSec  int64             `json:"uptime_sec"`
	Components []componentStatus `json:"components"`
}

// Live handles GET /health/live and only reports that the process is running.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	httpx.Raw(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready handles GET /health and GET /health/ready. It reports 503 when the
// database is unreachable or migrations are pending.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	components := make([]componentStatus, 0, 2)
	healthy := true

	start := time.Now()
	if err := h.pool.Ping(ctx); err != nil {
		healthy = false
		components = append(components, componentStatus{Name: "postgres", Status: "down", Detail: err.Error()})
	} else {
		components = append(components, componentStatus{
			Name:    "postgres",
			Status:  "ok",
			Latency: time.Since(start).Round(time.Millisecond).String(),
		})

		status, err := db.CheckSchema(ctx, h.pool)
		switch {
		case err != nil:
			healthy = false
			components = append(components, componentStatus{Name: "schema", Status: "unknown", Detail: err.Error()})
		case !status.UpToDate():
			healthy = false
			components = append(components, componentStatus{Name: "schema", Status: "outdated", Detail: status.Describe()})
		default:
			components = append(components, componentStatus{Name: "schema", Status: "ok", Detail: status.Describe()})
		}
	}

	body := healthResponse{
		Status:     "ok",
		Env:        h.cfg.AppEnv,
		Version:    h.version,
		UptimeSec:  int64(time.Since(h.startedAt).Seconds()),
		Components: components,
	}

	code := http.StatusOK
	if !healthy {
		body.Status = "degraded"
		code = http.StatusServiceUnavailable
	}

	httpx.Raw(w, r, code, body)
}

// Root handles GET / and describes the service to a human or a gateway.
func (h *HealthHandler) Root(w http.ResponseWriter, r *http.Request) {
	httpx.Raw(w, r, http.StatusOK, map[string]any{
		"name":       h.cfg.AppName,
		"env":        h.cfg.AppEnv,
		"version":    h.version,
		"api_prefix": h.cfg.APIPrefix,
		"resources": []string{
			"panels",
			"device-models",
			"panel-devices",
			"calibration-instruments",
			"calibrations",
			"calibration-readings",
		},
	})
}
