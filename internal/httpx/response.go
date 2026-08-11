package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// SuccessEnvelope is the body of every 2xx JSON response.
type SuccessEnvelope struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Code      string `json:"code"`
	Context   string `json:"context"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
}

// ErrorEnvelope is the body of every 4xx/5xx JSON response.
type ErrorEnvelope struct {
	Status    string       `json:"status"`
	Timestamp string       `json:"timestamp"`
	Code      string       `json:"code"`
	Context   string       `json:"context"`
	Message   string       `json:"message"`
	Errors    []FieldError `json:"errors"`
	RequestID string       `json:"request_id,omitempty"`
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// Success writes a success envelope using the HTTP status carried by the code.
func Success(w http.ResponseWriter, r *http.Request, code SuccessCode, data any) {
	writeJSON(w, r, code.Status, SuccessEnvelope{
		Status:    "success",
		Timestamp: nowISO(),
		Code:      code.Code,
		Context:   code.Context,
		Message:   code.Message,
		Data:      data,
	})
}

// Error writes an error envelope. Any non-AppError is reported as E500_001 and
// its cause is logged rather than leaked to the client.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	appErr := AsAppError(err)

	fields := appErr.Fields
	if fields == nil {
		fields = []FieldError{}
	}

	if appErr.Code.Status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			"code", appErr.Code.Code,
			"method", r.Method,
			"path", r.URL.Path,
			"error", appErr.Error(),
		)
	} else {
		slog.DebugContext(r.Context(), "request rejected",
			"code", appErr.Code.Code,
			"method", r.Method,
			"path", r.URL.Path,
			"error", appErr.Error(),
		)
	}

	writeJSON(w, r, appErr.Code.Status, ErrorEnvelope{
		Status:    "error",
		Timestamp: nowISO(),
		Code:      appErr.Code.Code,
		Context:   appErr.Code.Context,
		Message:   appErr.ResolvedMessage(),
		Errors:    fields,
		RequestID: RequestIDFromContext(r.Context()),
	})
}

// Raw writes an arbitrary payload that is intentionally outside the envelope
// (health checks, service root, rate limiting).
func Raw(w http.ResponseWriter, r *http.Request, status int, payload any) {
	writeJSON(w, r, status, payload)
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to marshal response", "error", err)
		http.Error(w, `{"status":"error","code":"E500_001","message":"Internal server error."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}
