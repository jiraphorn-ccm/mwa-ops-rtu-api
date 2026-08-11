// Package middleware holds the cross-cutting HTTP concerns of the service.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/httpx"
)

// HeaderRequestID is the correlation id echoed on every response.
const HeaderRequestID = "X-Request-Id"

// NormalizePath collapses duplicate slashes in the request path so clients
// that join base URL + prefix incorrectly (e.g. //api/rtu/v1/panels) still
// reach the registered routes.
func NormalizePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean(r.URL.Path)
		if cleaned != r.URL.Path {
			r.URL.Path = cleaned
			r.URL.RawPath = cleaned
		}
		next.ServeHTTP(w, r)
	})
}

// RequestID reuses an inbound correlation id or mints a new one, and publishes
// it on the context, the response header and the chi log entry.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}

		ctx := httpx.WithRequestID(r.Context(), id)
		ctx = context.WithValue(ctx, middleware.RequestIDKey, id)
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AppEnv advertises the deployment environment on every response, matching the
// behaviour of the other MWA services behind the gateway.
func AppEnv(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-App-Env", cfg.AppEnv)
			if cfg.IsStaging() {
				w.Header().Set("X-Staging-Mode", "1")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Logger emits one structured line per request.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				attrs := []any{
					"request_id", httpx.RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration_ms", float64(time.Since(start).Microseconds()) / 1000,
					"remote_ip", r.RemoteAddr,
				}
				if q := r.URL.RawQuery; q != "" {
					attrs = append(attrs, "query", q)
				}
				if auth, ok := httpx.AuthFromContext(r.Context()); ok && auth.UserID != "" {
					attrs = append(attrs, "user_id", auth.UserID)
				}

				switch {
				case ww.Status() >= http.StatusInternalServerError:
					logger.ErrorContext(r.Context(), "http request", attrs...)
				case ww.Status() >= http.StatusBadRequest:
					logger.WarnContext(r.Context(), "http request", attrs...)
				default:
					logger.InfoContext(r.Context(), "http request", attrs...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// Recoverer turns a panic into an E500_001 response instead of a dropped
// connection.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				logger.ErrorContext(r.Context(), "panic recovered",
					"request_id", httpx.RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				httpx.Error(w, r, httpx.Err(httpx.ErrInternal))
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit caps the request body so a malicious client cannot exhaust memory.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// StagingGuard blocks mutating requests on staging with E600_001.
func StagingGuard(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.IsStaging() {
				next.ServeHTTP(w, r)
				return
			}
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				httpx.Error(w, r, httpx.Err(httpx.ErrStagingGuard))
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

// NotFound answers unknown routes with E500_002.
func NotFound(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, r, httpx.Err(httpx.ErrEndpointNotFound))
}

// MethodNotAllowed answers unsupported verbs with E500_005.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, r, httpx.Err(httpx.ErrMethodNotAllowed))
}
