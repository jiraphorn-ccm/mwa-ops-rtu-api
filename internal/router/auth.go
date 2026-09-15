package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/handler"
	"github.com/rtu-api/internal/middleware"
)

func mountAuth(api chi.Router, h *handler.Handlers, cfg *config.Config) {
	api.Route("/auth", func(r chi.Router) {
		r.Get("/register/status", h.Auth.RegisterStatus)
		r.Post("/register", h.Auth.Register)
		r.Post("/login", h.Auth.Login)
		r.Post("/refresh", h.Auth.Refresh)
		r.Post("/logout", h.Auth.Logout)

		r.Group(func(priv chi.Router) {
			priv.Use(middleware.RequireAccessToken(cfg))
			priv.Get("/me", h.Auth.Me)
			priv.Post("/change-password", h.Auth.ChangePassword)
		})
	})
}

func mountUsers(api chi.Router, h *handler.Handlers) {
	api.Route("/users", func(r chi.Router) {
		r.Get("/", h.Users.List)
		r.Post("/", h.Users.Create)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Users.Get)
			r.Put("/", h.Users.Update)
			r.Patch("/", h.Users.Update)
			r.Delete("/", h.Users.Delete)
			r.Delete("/permanent", h.Users.Purge)
			r.Post("/restore", h.Users.Restore)
		})
	})
}

func mountAuditLogs(api chi.Router, h *handler.Handlers) {
	api.Get("/audit-logs", h.AuditLogs.List)
}
