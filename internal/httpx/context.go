package httpx

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyAuth
	ctxKeyAuthSlot
)

// WithRequestID stores the correlation id of the current request.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestIDFromContext returns the correlation id, or "" when it is absent.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// AuthClaims is the JWT payload issued by this service (and accepted from
// compatible HS256 access tokens). Roles/permissions are ignored — RTU does
// not enforce RBAC.
type AuthClaims struct {
	jwt.RegisteredClaims
	UserID       string   `json:"user_id,omitempty"`
	TokenType    string   `json:"type,omitempty"`
	EmployeeCode string   `json:"employee_code,omitempty"`
	Email        string   `json:"email,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
}

// AuthInfo is the authenticated caller attached to a request context.
type AuthInfo struct {
	Subject      string
	UserID       string
	EmployeeCode string
	Email        string
	Roles        []string
	Permissions  []string
}

type authSlot struct {
	info AuthInfo
	ok   bool
}

// EnsureAuthSlot hangs a mutable auth holder on ctx so outer middleware (audit)
// can read the caller after Auth swaps the request context.
func EnsureAuthSlot(ctx context.Context) context.Context {
	if _, ok := ctx.Value(ctxKeyAuthSlot).(*authSlot); ok {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyAuthSlot, &authSlot{})
}

// WithAuth stores the authenticated caller on the context.
func WithAuth(ctx context.Context, info AuthInfo) context.Context {
	if slot, ok := ctx.Value(ctxKeyAuthSlot).(*authSlot); ok {
		slot.info = info
		slot.ok = true
	}
	return context.WithValue(ctx, ctxKeyAuth, info)
}

// AuthFromContext returns the authenticated caller when present.
func AuthFromContext(ctx context.Context) (AuthInfo, bool) {
	if info, ok := ctx.Value(ctxKeyAuth).(AuthInfo); ok {
		return info, true
	}
	if slot, ok := ctx.Value(ctxKeyAuthSlot).(*authSlot); ok && slot.ok {
		return slot.info, true
	}
	return AuthInfo{}, false
}
