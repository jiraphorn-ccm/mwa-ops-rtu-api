// Package router maps URLs onto handlers and assembles the middleware stack.
package router

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/handler"
	"github.com/rtu-api/internal/middleware"
)

// Deps is everything the router needs to build the HTTP handler.
type Deps struct {
	Config      *config.Config
	Logger      *slog.Logger
	Handlers    *handler.Handlers
	RateLimiter *middleware.RateLimiter
}

// New builds the fully wired HTTP handler of the service.
func New(deps Deps) http.Handler {
	cfg := deps.Config
	r := chi.NewRouter()

	r.NotFound(middleware.NotFound)
	r.MethodNotAllowed(middleware.MethodNotAllowed)

	r.Use(chimw.RealIP)
	r.Use(middleware.NormalizePath)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer(deps.Logger))
	if cfg.MetricsEnabled {
		r.Use(middleware.Metrics)
	}
	r.Use(middleware.Logger(deps.Logger))
	r.Use(middleware.AppEnv(cfg))
	r.Use(middleware.StagingGuard(cfg))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   cfg.CORSAllowedHeaders,
		ExposedHeaders:   []string{middleware.HeaderRequestID, "X-App-Env"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(chimw.Compress(5))
	r.Use(middleware.BodyLimit(cfg.MaxBodyBytes))
	r.Use(chimw.Timeout(cfg.RequestTimeout))
	if cfg.RateLimitEnabled && deps.RateLimiter != nil {
		r.Use(deps.RateLimiter.Handler)
	}

	health := deps.Handlers.Health
	r.Get("/", health.Root)
	r.Get("/health", health.Ready)
	r.Get("/health/live", health.Live)
	r.Get("/health/ready", health.Ready)
	if cfg.MetricsEnabled {
		r.Handle("/metrics", middleware.MetricsHandler())
	}

	r.Route(cfg.APIPrefix, func(api chi.Router) {
		api.Use(middleware.Auth(cfg))
		api.Get("/", health.Root)
		mountPanels(api, deps.Handlers)
		mountDeviceModels(api, deps.Handlers)
		mountPanelDevices(api, deps.Handlers)
		mountCalibrationInstruments(api, deps.Handlers)
		mountCalibrations(api, deps.Handlers)
		mountWorkOrders(api, deps.Handlers)
		mountEngineers(api, deps.Handlers)
		mountChecklistItems(api, deps.Handlers)
		mountAttachments(api, deps.Handlers)
		mountNotifications(api, deps.Handlers)
	})

	return r
}

func mountPanels(api chi.Router, h *handler.Handlers) {
	api.Route("/panels", func(r chi.Router) {
		r.Get("/", h.Panels.List)
		r.Post("/", h.Panels.Create)
		r.Get("/code/{code}", h.Panels.GetByCode)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Panels.Get)
			r.Put("/", h.Panels.Update)
			r.Patch("/", h.Panels.Update)
			r.Delete("/", h.Panels.Delete)
			r.Delete("/permanent", h.Panels.Purge)
			r.Post("/restore", h.Panels.Restore)

			r.Get("/devices", h.PanelDevices.ListByPanel)
			r.Post("/devices", h.PanelDevices.CreateForPanel)

			r.Get("/work-orders", h.WorkOrders.ListByPanel)
			r.Post("/work-orders", h.WorkOrders.CreateForPanel)

			r.Get("/pm-reports", h.PmReports.ListHistoryByPanel)
			r.Get("/cm-reports", h.CmReports.ListHistoryByPanel)

			r.Get("/images", h.PanelImages.List)
			r.Post("/images", h.PanelImages.Create)

			r.Route("/images/{imageId}", func(r chi.Router) {
				r.Get("/", h.PanelImages.Get)
				r.Put("/", h.PanelImages.Update)
				r.Patch("/", h.PanelImages.Update)
				r.Delete("/", h.PanelImages.Delete)
			})
		})
	})
}

func mountDeviceModels(api chi.Router, h *handler.Handlers) {
	api.Route("/device-models", func(r chi.Router) {
		r.Get("/", h.DeviceModels.List)
		r.Post("/", h.DeviceModels.Create)
		r.Get("/code/{code}", h.DeviceModels.GetByCode)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.DeviceModels.Get)
			r.Put("/", h.DeviceModels.Update)
			r.Patch("/", h.DeviceModels.Update)
			r.Delete("/", h.DeviceModels.Delete)
			r.Delete("/permanent", h.DeviceModels.Purge)
			r.Post("/restore", h.DeviceModels.Restore)
		})
	})
}

func mountPanelDevices(api chi.Router, h *handler.Handlers) {
	api.Route("/panel-devices", func(r chi.Router) {
		r.Get("/", h.PanelDevices.List)
		r.Post("/", h.PanelDevices.Create)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.PanelDevices.Get)
			r.Put("/", h.PanelDevices.Update)
			r.Patch("/", h.PanelDevices.Update)
			r.Delete("/", h.PanelDevices.Delete)
			r.Delete("/permanent", h.PanelDevices.Purge)
			r.Post("/restore", h.PanelDevices.Restore)
			r.Post("/status", h.PanelDevices.RecordStatus)

			r.Get("/calibrations", h.Calibrations.ListByDevice)
			r.Post("/calibrations", h.Calibrations.CreateForDevice)

			r.Get("/work-orders", h.WorkOrders.ListByPanelDevice)
			r.Get("/cm-reports", h.CmReports.ListHistoryByPanelDevice)

			r.Get("/attachments", h.Attachments.ListByEntity("PANEL_DEVICE"))
			r.Post("/attachments", h.Attachments.CreateForEntity("PANEL_DEVICE"))
		})
	})
}

func mountCalibrationInstruments(api chi.Router, h *handler.Handlers) {
	api.Route("/calibration-instruments", func(r chi.Router) {
		r.Get("/", h.CalibrationInstruments.List)
		r.Post("/", h.CalibrationInstruments.Create)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.CalibrationInstruments.Get)
			r.Put("/", h.CalibrationInstruments.Update)
			r.Patch("/", h.CalibrationInstruments.Update)
			r.Delete("/", h.CalibrationInstruments.Delete)
			r.Delete("/permanent", h.CalibrationInstruments.Purge)
			r.Post("/restore", h.CalibrationInstruments.Restore)
		})
	})
}

func mountCalibrations(api chi.Router, h *handler.Handlers) {
	api.Route("/calibrations", func(r chi.Router) {
		r.Get("/", h.Calibrations.List)
		r.Post("/", h.Calibrations.Create)
		r.Get("/summary", h.Calibrations.Summary)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Calibrations.Get)
			r.Put("/", h.Calibrations.Update)
			r.Patch("/", h.Calibrations.Update)
			r.Delete("/", h.Calibrations.Delete)

			r.Get("/readings", h.Calibrations.ListReadings)
			r.Post("/readings", h.Calibrations.AddReading)
			r.Put("/readings", h.Calibrations.ReplaceReadings)

			r.Route("/readings/{readingId}", func(r chi.Router) {
				r.Get("/", h.Calibrations.GetReadingByCalibration)
				r.Put("/", h.Calibrations.UpdateReadingByCalibration)
				r.Patch("/", h.Calibrations.UpdateReadingByCalibration)
				r.Delete("/", h.Calibrations.DeleteReadingByCalibration)
			})

			r.Get("/attachments", h.Attachments.ListByEntity("CALIBRATION"))
			r.Post("/attachments", h.Attachments.CreateForEntity("CALIBRATION"))
		})
	})

	api.Route("/calibration-readings/{id}", func(r chi.Router) {
		r.Get("/", h.Calibrations.GetReading)
		r.Put("/", h.Calibrations.UpdateReading)
		r.Patch("/", h.Calibrations.UpdateReading)
		r.Delete("/", h.Calibrations.DeleteReading)
	})
}

func mountWorkOrders(api chi.Router, h *handler.Handlers) {
	api.Route("/work-orders", func(r chi.Router) {
		r.Get("/", h.WorkOrders.List)
		r.Post("/", h.WorkOrders.Create)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.WorkOrders.Get)
			r.Put("/", h.WorkOrders.Update)
			r.Patch("/", h.WorkOrders.Update)
			r.Delete("/", h.WorkOrders.Delete)
			r.Post("/restore", h.WorkOrders.Restore)

			r.Post("/reassign", h.WorkOrders.Reassign)
			r.Post("/check-in", h.WorkOrders.CheckIn)
			r.Post("/check-out", h.WorkOrders.CheckOut)

			r.Get("/rounds", h.WorkOrders.ListRounds)
			r.Get("/activity", h.WorkOrders.ListActivity)

			r.Get("/approvals", h.Approvals.List)
			r.Post("/approvals", h.Approvals.Create)

			r.Get("/pm-report", h.PmReports.GetForWorkOrder)
			r.Put("/pm-report", h.PmReports.Save)
			r.Post("/pm-report/submit", h.PmReports.Submit)
			r.Get("/pm-reports", h.PmReports.ListHistoryByWorkOrder)

			r.Get("/cm-report", h.CmReports.GetForWorkOrder)
			r.Put("/cm-report", h.CmReports.Save)
			r.Post("/cm-report/submit", h.CmReports.Submit)
			r.Get("/cm-reports", h.CmReports.ListHistoryByWorkOrder)

			r.Get("/attachments", h.Attachments.ListByEntity("WORK_ORDER"))
			r.Post("/attachments", h.Attachments.CreateForEntity("WORK_ORDER"))
		})
	})

	api.Route("/pm-reports/{id}", func(r chi.Router) {
		r.Get("/", h.PmReports.Get)
		r.Delete("/", h.PmReports.Delete)

		r.Get("/onsite-fixes", h.CmReports.ListByPmReport)
		r.Post("/onsite-fixes", h.CmReports.CreateOnsiteFix)
		r.Post("/escalate", h.CmReports.Escalate)

		r.Get("/attachments", h.Attachments.ListByEntity("PM_REPORT"))
		r.Post("/attachments", h.Attachments.CreateForEntity("PM_REPORT"))
	})

	api.Route("/cm-reports/{id}", func(r chi.Router) {
		r.Get("/", h.CmReports.Get)
		r.Put("/", h.CmReports.Update)
		r.Patch("/", h.CmReports.Update)
		r.Delete("/", h.CmReports.Delete)

		r.Get("/attachments", h.Attachments.ListByEntity("CM_REPORT"))
		r.Post("/attachments", h.Attachments.CreateForEntity("CM_REPORT"))
	})

	api.Route("/pm-ground-tests/{id}/attachments", func(r chi.Router) {
		r.Get("/", h.Attachments.ListByEntity("PM_GROUND_TEST"))
		r.Post("/", h.Attachments.CreateForEntity("PM_GROUND_TEST"))
	})

	api.Route("/pm-power-test-points/{id}/attachments", func(r chi.Router) {
		r.Get("/", h.Attachments.ListByEntity("PM_POWER_TEST_POINT"))
		r.Post("/", h.Attachments.CreateForEntity("PM_POWER_TEST_POINT"))
	})
}

func mountEngineers(api chi.Router, h *handler.Handlers) {
	api.Route("/engineers", func(r chi.Router) {
		r.Get("/", h.Engineers.List)
		r.Post("/", h.Engineers.Create)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Engineers.Get)
			r.Put("/", h.Engineers.Update)
			r.Patch("/", h.Engineers.Update)
			r.Delete("/", h.Engineers.Delete)
			r.Delete("/permanent", h.Engineers.Purge)
			r.Post("/restore", h.Engineers.Restore)
		})
	})
}

func mountChecklistItems(api chi.Router, h *handler.Handlers) {
	api.Route("/checklist-items", func(r chi.Router) {
		r.Get("/", h.ChecklistItems.List)
		r.Post("/", h.ChecklistItems.Create)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.ChecklistItems.Get)
			r.Put("/", h.ChecklistItems.Update)
			r.Patch("/", h.ChecklistItems.Update)
			r.Delete("/", h.ChecklistItems.Delete)
			r.Delete("/permanent", h.ChecklistItems.Purge)
			r.Post("/restore", h.ChecklistItems.Restore)
		})
	})
}

// mountAttachments serves the standalone /attachments/{id} actions. The list
// and create actions are nested under each entity's own route (see
// mountWorkOrders, mountPanelDevices, mountCalibrations).
func mountAttachments(api chi.Router, h *handler.Handlers) {
	api.Route("/attachments/{id}", func(r chi.Router) {
		r.Get("/", h.Attachments.Get)
		r.Put("/", h.Attachments.Update)
		r.Patch("/", h.Attachments.Update)
		r.Delete("/", h.Attachments.Delete)
	})
}

func mountNotifications(api chi.Router, h *handler.Handlers) {
	api.Route("/notifications", func(r chi.Router) {
		r.Get("/", h.Notifications.List)
		r.Post("/", h.Notifications.Create)
		r.Post("/read-all", h.Notifications.MarkAllRead)
		r.Get("/unread-count", h.Notifications.CountUnread)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Notifications.Get)
			r.Delete("/", h.Notifications.Delete)
			r.Post("/read", h.Notifications.MarkRead)
		})
	})
}
