package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/middleware"
)

const testSecret = "01234567890123456789012345678901"

func TestAuthRejectsMissingToken(t *testing.T) {
	cfg := &config.Config{AuthEnabled: true, AuthJWTSecret: testSecret}
	handler := middleware.Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/rtu/v1/panels", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthAcceptsValidToken(t *testing.T) {
	cfg := &config.Config{AuthEnabled: true, AuthJWTSecret: testSecret}
	token := signToken(t, httpx.AuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: "user-1",
	})

	var got httpx.AuthInfo
	handler := middleware.Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := httpx.AuthFromContext(r.Context())
		if !ok {
			t.Fatal("expected auth in context")
		}
		got = info
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/rtu/v1/panels", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got.UserID != "user-1" {
		t.Fatalf("user_id=%q", got.UserID)
	}
}

func TestStagingGuardBlocksMutations(t *testing.T) {
	cfg := &config.Config{AppEnv: config.EnvStaging}
	handler := middleware.StagingGuard(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/rtu/v1/panels", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusLocked {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStagingGuardAllowsGET(t *testing.T) {
	cfg := &config.Config{AppEnv: config.EnvStaging}
	handler := middleware.StagingGuard(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/rtu/v1/panels", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
}

func signToken(t *testing.T, claims httpx.AuthClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
