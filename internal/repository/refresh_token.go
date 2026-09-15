package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// RefreshTokenRepository stores hashed refresh sessions.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// CreateRefreshTokenInput is a new session.
type CreateRefreshTokenInput struct {
	UserID    uuid.UUID
	TokenHash string
	IP        string
	UserAgent string
	ExpiresAt time.Time
}

// Create inserts a hashed refresh token.
func (r *RefreshTokenRepository) Create(ctx context.Context, in CreateRefreshTokenInput) (sqlc.RefreshToken, error) {
	row, err := r.q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		UserID:    in.UserID,
		TokenHash: in.TokenHash,
		IpAddress: optionalStr(in.IP),
		UserAgent: optionalStr(in.UserAgent),
		ExpiresAt: in.ExpiresAt,
	})
	if err != nil {
		return sqlc.RefreshToken{}, db.Translate(err)
	}
	return row, nil
}

// GetByHash returns the session and whether the owning user is active.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (sqlc.GetRefreshTokenByHashRow, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return sqlc.GetRefreshTokenByHashRow{}, db.Translate(err, db.WithNotFound(httpx.ErrRefreshInvalid))
	}
	return row, nil
}

// TouchMeta updates last-seen IP / user-agent.
func (r *RefreshTokenRepository) TouchMeta(ctx context.Context, id uuid.UUID, ip, userAgent string) error {
	if err := r.q.TouchRefreshTokenMeta(ctx, sqlc.TouchRefreshTokenMetaParams{
		ID:        id,
		IpAddress: optionalStr(ip),
		UserAgent: optionalStr(userAgent),
	}); err != nil {
		return db.Translate(err)
	}
	return nil
}

// Revoke marks one session unusable.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	if err := r.q.RevokeRefreshToken(ctx, id); err != nil {
		return db.Translate(err)
	}
	return nil
}

// RevokeAllForUser ends every session for the user (logout-all / password change).
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.q.RevokeRefreshTokensForUser(ctx, userID); err != nil {
		return db.Translate(err)
	}
	return nil
}
