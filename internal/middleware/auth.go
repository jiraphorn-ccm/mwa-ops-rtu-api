package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/httpx"
)

// Auth requires a valid Bearer access token. Public /auth/login, refresh,
// logout and register sit outside this middleware. There is no permission check.
func Auth(cfg *config.Config) func(http.Handler) http.Handler {
	return RequireAccessToken(cfg)
}

// RequireAccessToken always requires a valid access JWT.
func RequireAccessToken(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			info, err := bearerAuth(cfg, r.Header.Get("Authorization"))
			if err != nil {
				if errors.Is(err, errNoToken) {
					httpx.Error(w, r, httpx.Err(httpx.ErrTokenRequired))
					return
				}
				httpx.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(httpx.WithAuth(r.Context(), info)))
		})
	}
}

var errNoToken = errors.New("missing bearer token")

func bearerAuth(cfg *config.Config, header string) (httpx.AuthInfo, error) {
	if strings.TrimSpace(header) == "" {
		return httpx.AuthInfo{}, errNoToken
	}
	tokenStr, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || strings.TrimSpace(tokenStr) == "" {
		return httpx.AuthInfo{}, httpx.Err(httpx.ErrTokenMalformed)
	}
	return parseAccessToken(cfg, tokenStr)
}

func parseAccessToken(cfg *config.Config, tokenStr string) (httpx.AuthInfo, error) {
	claims := &httpx.AuthClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, httpx.Err(httpx.ErrTokenMalformed)
		}
		return []byte(cfg.AuthJWTSecret), nil
	})
	if err != nil || token == nil || !token.Valid {
		if err != nil && strings.Contains(err.Error(), "token is expired") {
			return httpx.AuthInfo{}, httpx.Err(httpx.ErrTokenExpired)
		}
		return httpx.AuthInfo{}, httpx.Err(httpx.ErrTokenMalformed)
	}

	if claims.TokenType != "" && !strings.EqualFold(claims.TokenType, "access") {
		return httpx.AuthInfo{}, httpx.Err(httpx.ErrTokenMalformed)
	}
	if cfg.AuthJWTIssuer != "" && claims.Issuer != cfg.AuthJWTIssuer {
		return httpx.AuthInfo{}, httpx.Err(httpx.ErrTokenMalformed)
	}

	userID := claims.UserID
	if userID == "" {
		userID = claims.Subject
	}
	return httpx.AuthInfo{
		Subject:      claims.Subject,
		UserID:       userID,
		EmployeeCode: claims.EmployeeCode,
		Email:        claims.Email,
		Roles:        claims.Roles,
		Permissions:  claims.Permissions,
	}, nil
}
