package httpx

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyAuth
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

// AuthClaims is the JWT payload expected from the MWA auth service.
type AuthClaims struct {
	jwt.RegisteredClaims
	UserID      string   `json:"user_id,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// AuthInfo is the authenticated caller attached to a request context.
type AuthInfo struct {
	Subject     string
	UserID      string
	Roles       []string
	Permissions []string
}

// WithAuth stores the authenticated caller on the context.
func WithAuth(ctx context.Context, info AuthInfo) context.Context {
	return context.WithValue(ctx, ctxKeyAuth, info)
}

// AuthFromContext returns the authenticated caller when present.
func AuthFromContext(ctx context.Context) (AuthInfo, bool) {
	info, ok := ctx.Value(ctxKeyAuth).(AuthInfo)
	return info, ok
}
