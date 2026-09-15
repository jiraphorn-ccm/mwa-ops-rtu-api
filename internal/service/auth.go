package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/middleware"
	"github.com/rtu-api/internal/repository"
)

// dummyHash keeps bcrypt compare time similar when the login is unknown.
var dummyHash = []byte("$2a$10$C6UzMDM.H6dfI/f/IKcEe.O4p0q0q0q0q0q0q0q0q0q0q0q0q0q0q")

func init() {
	if h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-timing"), bcryptCost); err == nil {
		dummyHash = h
	}
}

// AuthService issues access/refresh tokens. It does not check roles or permissions.
type AuthService struct {
	users  *repository.UserRepository
	tokens *repository.RefreshTokenRepository
	audit  *repository.AuditLogRepository
	cfg    *config.Config
}

// LoginInput is the POST /auth/login body.
type LoginInput struct {
	Username string `json:"username" validate:"required,max=100"`
	Password string `json:"password" validate:"required,max=72"`
}

// RefreshInput is the POST /auth/refresh body.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutInput is the POST /auth/logout body.
type LogoutInput struct {
	RefreshToken string `json:"refresh_token" validate:"omitempty"`
}

// ChangePasswordInput is the POST /auth/change-password body.
type ChangePasswordInput struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// RequestMeta is the client address captured at the HTTP edge.
type RequestMeta struct {
	IP        string
	UserAgent string
	RequestID string
	Path      string
}

// TokenResponse is returned by login and refresh.
type TokenResponse struct {
	User         *repository.UserView `json:"user,omitempty"`
	AccessToken  string               `json:"access_token"`
	RefreshToken string               `json:"refresh_token,omitempty"`
	TokenType    string               `json:"token_type"`
	ExpiresIn    int64                `json:"expires_in"`
}

// RegisterStatus reports whether the first-user bootstrap is still open.
type RegisterStatus struct {
	RegistrationOpen bool  `json:"registration_open"`
	UserCount        int64 `json:"user_count"`
}

// RegisterStatus returns whether POST /auth/register is still allowed.
func (s *AuthService) RegisterStatus(ctx context.Context) (RegisterStatus, error) {
	n, err := s.users.Count(ctx)
	if err != nil {
		return RegisterStatus{}, err
	}
	return RegisterStatus{RegistrationOpen: n == 0, UserCount: n}, nil
}

// Register creates the first user and signs them in. Closed once any user exists.
func (s *AuthService) Register(ctx context.Context, in UserCreateInput, meta RequestMeta) (TokenResponse, error) {
	n, err := s.users.Count(ctx)
	if err != nil {
		return TokenResponse{}, err
	}
	if n > 0 {
		return TokenResponse{}, httpx.Err(httpx.ErrRegistrationClosed)
	}
	active := true
	in.Active = &active

	user, err := (&UserService{repo: s.users}).Create(ctx, in)
	if err != nil {
		return TokenResponse{}, err
	}

	s.record(ctx, meta, &user.ID, "REGISTER", user.ID)
	return s.issueSession(ctx, user.ID, meta, true)
}

// Login verifies username (email or employee_code) + password.
func (s *AuthService) Login(ctx context.Context, in LoginInput, meta RequestMeta) (TokenResponse, error) {
	login := strings.TrimSpace(in.Username)
	user, err := s.users.GetByLogin(ctx, login)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		return TokenResponse{}, httpx.Err(httpx.ErrInvalidCredentials)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return TokenResponse{}, httpx.Err(httpx.ErrInvalidCredentials)
	}
	if !user.Active {
		return TokenResponse{}, httpx.Err(httpx.ErrAccountDisabled)
	}

	if err := s.users.TouchLastLogin(ctx, user.ID); err != nil {
		return TokenResponse{}, err
	}

	s.record(ctx, meta, &user.ID, "LOGIN", user.ID)
	return s.issueSession(ctx, user.ID, meta, true)
}

// Refresh mints a new access token from a still-valid refresh session.
func (s *AuthService) Refresh(ctx context.Context, in RefreshInput, meta RequestMeta) (TokenResponse, error) {
	row, err := s.tokens.GetByHash(ctx, hashRefreshToken(in.RefreshToken))
	if err != nil {
		return TokenResponse{}, httpx.Err(httpx.ErrRefreshInvalid)
	}
	if row.RevokedAt != nil {
		return TokenResponse{}, httpx.Err(httpx.ErrRefreshRevoked)
	}
	if time.Now().After(row.ExpiresAt) {
		return TokenResponse{}, httpx.Err(httpx.ErrRefreshInvalid)
	}
	if !row.UserActive {
		return TokenResponse{}, httpx.Err(httpx.ErrAccountDisabled)
	}

	if err := s.tokens.TouchMeta(ctx, row.ID, meta.IP, meta.UserAgent); err != nil {
		return TokenResponse{}, err
	}

	access, expiresIn, err := s.signAccess(ctx, row.UserID)
	if err != nil {
		return TokenResponse{}, err
	}
	s.record(ctx, meta, &row.UserID, "REFRESH", row.UserID)
	return TokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}, nil
}

// Logout revokes the given refresh token. Missing/unknown tokens succeed.
func (s *AuthService) Logout(ctx context.Context, in LogoutInput, meta RequestMeta) error {
	if strings.TrimSpace(in.RefreshToken) == "" {
		return nil
	}
	row, err := s.tokens.GetByHash(ctx, hashRefreshToken(in.RefreshToken))
	if err != nil {
		return nil
	}
	if err := s.tokens.Revoke(ctx, row.ID); err != nil {
		return err
	}
	s.record(ctx, meta, &row.UserID, "LOGOUT", row.UserID)
	return nil
}

// Me returns the caller described by the access token.
func (s *AuthService) Me(ctx context.Context) (repository.UserView, error) {
	id, err := requireActor(ctx)
	if err != nil {
		return repository.UserView{}, err
	}
	user, err := s.users.Get(ctx, id)
	if err != nil {
		return repository.UserView{}, err
	}
	if !user.Active {
		return repository.UserView{}, httpx.Err(httpx.ErrAccountDisabled)
	}
	return user, nil
}

// ChangePassword updates the caller's password and revokes every refresh session.
func (s *AuthService) ChangePassword(ctx context.Context, in ChangePasswordInput, meta RequestMeta) error {
	id, err := requireActor(ctx)
	if err != nil {
		return err
	}
	user, err := s.users.GetAuth(ctx, id)
	if err != nil {
		return err
	}
	if !user.Active {
		return httpx.Err(httpx.ErrAccountDisabled)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.OldPassword)); err != nil {
		return httpx.Err(httpx.ErrOldPasswordInvalid)
	}
	hash, err := hashPassword(in.NewPassword)
	if err != nil {
		return err
	}
	if _, err := s.users.SetPassword(ctx, id, hash); err != nil {
		return err
	}
	if err := s.tokens.RevokeAllForUser(ctx, id); err != nil {
		return err
	}
	s.record(ctx, meta, &id, "CHANGE_PASSWORD", id)
	return nil
}

func (s *AuthService) issueSession(ctx context.Context, userID uuid.UUID, meta RequestMeta, withRefresh bool) (TokenResponse, error) {
	access, expiresIn, err := s.signAccess(ctx, userID)
	if err != nil {
		return TokenResponse{}, err
	}
	view, err := s.users.Get(ctx, userID)
	if err != nil {
		return TokenResponse{}, err
	}

	out := TokenResponse{
		User:        &view,
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}
	if !withRefresh {
		return out, nil
	}

	plain, hash, err := newRefreshToken()
	if err != nil {
		return TokenResponse{}, err
	}
	if _, err := s.tokens.Create(ctx, repository.CreateRefreshTokenInput{
		UserID:    userID,
		TokenHash: hash,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: time.Now().Add(s.cfg.JWTRefreshTTL()),
	}); err != nil {
		return TokenResponse{}, err
	}
	out.RefreshToken = plain
	return out, nil
}

func (s *AuthService) signAccess(ctx context.Context, userID uuid.UUID) (string, int64, error) {
	if s.cfg == nil || strings.TrimSpace(s.cfg.AuthJWTSecret) == "" {
		return "", 0, httpx.Err(httpx.ErrInternal).WithCause(errMissingJWTSecret{})
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return "", 0, err
	}

	ttl := s.cfg.JWTAccessTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := time.Now()
	claims := httpx.AuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    s.cfg.AuthJWTIssuer,
		},
		UserID:       user.ID.String(),
		TokenType:    "access",
		EmployeeCode: user.EmployeeCode,
		Email:        user.Email,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.AuthJWTSecret))
	if err != nil {
		return "", 0, httpx.Err(httpx.ErrInternal).WithCause(err)
	}
	return token, int64(ttl / time.Second), nil
}

type errMissingJWTSecret struct{}

func (errMissingJWTSecret) Error() string { return "AUTH_JWT_SECRET / JWT_SECRET is not set" }

func (s *AuthService) record(ctx context.Context, meta RequestMeta, userID *uuid.UUID, action string, resourceID uuid.UUID) {
	if s.audit == nil {
		return
	}
	path := meta.Path
	if path == "" {
		path = "/auth"
	}
	_ = s.audit.Record(ctx, middleware.AuditEntry{
		UserID:     userID,
		Action:     action,
		Method:     "POST",
		Path:       path,
		Resource:   strPtr("auth"),
		ResourceID: &resourceID,
		StatusCode: 200,
		IP:         meta.IP,
		UserAgent:  meta.UserAgent,
		RequestID:  meta.RequestID,
	})
}

func requireActor(ctx context.Context) (uuid.UUID, error) {
	auth, ok := httpx.AuthFromContext(ctx)
	if !ok {
		return uuid.Nil, httpx.Err(httpx.ErrUnauthorized)
	}
	for _, raw := range []string{auth.UserID, auth.Subject} {
		if id, err := uuid.Parse(raw); err == nil {
			return id, nil
		}
	}
	return uuid.Nil, httpx.Err(httpx.ErrUnauthorized)
}

func newRefreshToken() (plain, hash string, err error) {
	plain = uuid.NewString()
	return plain, hashRefreshToken(plain), nil
}

func hashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func strPtr(s string) *string { return &s }
