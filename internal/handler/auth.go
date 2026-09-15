package handler

import (
	"net"
	"net/http"
	"strings"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// AuthHandler serves /auth/*.
type AuthHandler struct {
	svc *service.AuthService
}

func requestMeta(r *http.Request) service.RequestMeta {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return service.RequestMeta{
		IP:        ip,
		UserAgent: r.UserAgent(),
		RequestID: httpx.RequestIDFromContext(r.Context()),
		Path:      r.URL.Path,
	}
}

// RegisterStatus handles GET /auth/register/status.
func (h *AuthHandler) RegisterStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.RegisterStatus(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessDetail, status)
}

// Register handles POST /auth/register (first user only).
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in service.UserCreateInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.svc.Register(r.Context(), in, requestMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessCreate, out)
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var in service.LoginInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.svc.Login(r.Context(), in, requestMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessLogin, out)
}

// Refresh handles POST /auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var in service.RefreshInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.svc.Refresh(r.Context(), in, requestMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessRefresh, out)
}

// Logout handles POST /auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var in service.LogoutInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Logout(r.Context(), in, requestMeta(r)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessLogout, map[string]bool{"logged_out": true})
}

// Me handles GET /auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.svc.Me(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessDetail, user)
}

// ChangePassword handles POST /auth/change-password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var in service.ChangePasswordInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.ChangePassword(r.Context(), in, requestMeta(r)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, r, httpx.SuccessUpdate, map[string]bool{"password_changed": true})
}
