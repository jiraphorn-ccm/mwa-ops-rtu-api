package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/rtu-api/internal/handler"
)

// mountLocalImages is a standalone disk-upload router. Files land in
// server/images and never go through S3.
func mountLocalImages(r chi.Router, h *handler.Handlers) {
	r.Route("/local-images", func(r chi.Router) {
		r.Post("/", h.LocalImages.Upload)
		r.Get("/{filename}", h.LocalImages.Get)
	})
}
