package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/httpx"
)

// Auth validates Bearer JWT access tokens when authentication is enabled.
// Health, metrics and OPTIONS preflights are always allowed through.
func Auth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.AuthEnabled || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			raw := r.Header.Get("Authorization")
			if raw == "" {
				httpx.Error(w, r, httpx.Err(httpx.ErrTokenRequired))
				return
			}

			tokenStr, ok := strings.CutPrefix(raw, "Bearer ")
			if !ok || strings.TrimSpace(tokenStr) == "" {
				httpx.Error(w, r, httpx.Err(httpx.ErrTokenMalformed))
				return
			}

			claims := &httpx.AuthClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				if t.Method != jwt.SigningMethodHS256 {
					return nil, httpx.Err(httpx.ErrTokenMalformed)
				}
				return []byte(cfg.AuthJWTSecret), nil
			})
			if err != nil || !token.Valid {
				if err != nil {
					if strings.Contains(err.Error(), "token is expired") {
						httpx.Error(w, r, httpx.Err(httpx.ErrTokenExpired))
						return
					}
				}
				httpx.Error(w, r, httpx.Err(httpx.ErrTokenMalformed))
				return
			}

			if cfg.AuthJWTIssuer != "" && claims.Issuer != cfg.AuthJWTIssuer {
				httpx.Error(w, r, httpx.Err(httpx.ErrTokenMalformed))
				return
			}

			ctx := httpx.WithAuth(r.Context(), httpx.AuthInfo{
				Subject:     claims.Subject,
				UserID:      claims.UserID,
				Roles:       claims.Roles,
				Permissions: claims.Permissions,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission rejects the request when the caller lacks a permission.
// Mount on routes that need finer control than "any authenticated user".
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, ok := httpx.AuthFromContext(r.Context())
			if !ok {
				httpx.Error(w, r, httpx.Err(httpx.ErrUnauthorized))
				return
			}
			for _, p := range auth.Permissions {
				if p == permission || p == "*" {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.Error(w, r, httpx.Err(httpx.ErrInsufficientPerms))
		})
	}
}

// AuthOptional attaches claims when a valid Bearer token is present but does
// not reject requests that arrive without one.
func AuthOptional(cfg *config.Config) func(http.Handler) http.Handler {
	if !cfg.AuthEnabled {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			tokenStr, ok := strings.CutPrefix(raw, "Bearer ")
			if !ok || strings.TrimSpace(tokenStr) == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims := &httpx.AuthClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				return []byte(cfg.AuthJWTSecret), nil
			})
			if err != nil || !token.Valid {
				next.ServeHTTP(w, r)
				return
			}

			ctx := httpx.WithAuth(r.Context(), httpx.AuthInfo{
				Subject:     claims.Subject,
				UserID:      claims.UserID,
				Roles:       claims.Roles,
				Permissions: claims.Permissions,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
