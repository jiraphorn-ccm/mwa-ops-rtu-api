package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
)

// AuditEntry is one recorded HTTP action.
type AuditEntry struct {
	UserID     *uuid.UUID
	Action     string
	Method     string
	Path       string
	Resource   *string
	ResourceID *uuid.UUID
	StatusCode int
	IP         string
	UserAgent  string
	RequestID  string
}

// AuditSink persists audit rows. Implementations must be safe for concurrent use.
type AuditSink interface {
	WriteAudit(ctx context.Context, e AuditEntry) error
}

var skipAuditExact = map[string]struct{}{
	"/health":       {},
	"/health/live":  {},
	"/health/ready": {},
	"/metrics":      {},
}

// Audit records mutating HTTP calls (who did what) after the handler returns.
// Failures are logged and never fail the request.
func Audit(logger *slog.Logger, sink AuditSink) func(http.Handler) http.Handler {
	if sink == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := httpx.EnsureAuthSlot(r.Context())
			r = r.WithContext(ctx)
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			if !shouldAudit(r) {
				return
			}

			entry := AuditEntry{
				UserID:     actorUUID(ctx),
				Action:     deriveAction(r.Method, r.URL.Path),
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: ww.Status(),
				IP:         clientIP(r),
				UserAgent:  r.UserAgent(),
				RequestID:  httpx.RequestIDFromContext(ctx),
			}
			entry.Resource, entry.ResourceID = resourceFromPath(r.URL.Path)

			go func(e AuditEntry) {
				writeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := sink.WriteAudit(writeCtx, e); err != nil && logger != nil {
					logger.Error("audit write failed", "error", err, "path", e.Path, "action", e.Action)
				}
			}(entry)
		})
	}
}

func shouldAudit(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	path := r.URL.Path
	if _, skip := skipAuditExact[path]; skip {
		return false
	}
	// Auth events are written by AuthService with the resolved user_id.
	if strings.Contains(path, "/auth/login") ||
		strings.Contains(path, "/auth/refresh") ||
		strings.Contains(path, "/auth/logout") ||
		strings.Contains(path, "/auth/register") {
		return false
	}
	return true
}

func deriveAction(method, path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, "/restore"):
		return "RESTORE"
	case strings.HasSuffix(lower, "/permanent"):
		return "PURGE"
	case strings.HasSuffix(lower, "/check-in"):
		return "CHECK_IN"
	case strings.HasSuffix(lower, "/check-out"):
		return "CHECK_OUT"
	case strings.HasSuffix(lower, "/submit"):
		return "SUBMIT"
	case strings.HasSuffix(lower, "/reassign"):
		return "REASSIGN"
	case strings.HasSuffix(lower, "/change-password"):
		return "CHANGE_PASSWORD"
	case method == http.MethodDelete:
		return "DELETE"
	case method == http.MethodPut || method == http.MethodPatch:
		return "UPDATE"
	default:
		return "CREATE"
	}
}

func resourceFromPath(path string) (*string, *uuid.UUID) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Drop API prefix: api/rtu/v1/...
	if len(parts) >= 3 && parts[0] == "api" {
		parts = parts[3:]
	}
	if len(parts) == 0 {
		return nil, nil
	}

	var resourceID *uuid.UUID
	resource := parts[0]
	for _, p := range parts[1:] {
		if id, err := uuid.Parse(p); err == nil {
			resourceID = &id
			break
		}
	}
	if len(parts) >= 3 && resourceID != nil {
		// /panels/{id}/images → panel_images
		for _, p := range parts[2:] {
			if _, err := uuid.Parse(p); err != nil && p != "permanent" && p != "restore" {
				resource = strings.TrimSuffix(resource, "s") + "_" + strings.ReplaceAll(p, "-", "_")
				break
			}
		}
	}
	return &resource, resourceID
}

func actorUUID(ctx context.Context) *uuid.UUID {
	auth, ok := httpx.AuthFromContext(ctx)
	if !ok {
		return nil
	}
	for _, raw := range []string{auth.UserID, auth.Subject} {
		if raw == "" {
			continue
		}
		if id, err := uuid.Parse(raw); err == nil {
			return &id
		}
	}
	return nil
}
