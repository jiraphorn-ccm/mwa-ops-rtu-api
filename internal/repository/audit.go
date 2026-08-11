package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// actorID returns the authenticated caller id from the request context.
// Prefers user_id over sub; returns nil when auth is off or not a UUID.
func actorID(ctx context.Context) *uuid.UUID {
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

func createAudit(ctx context.Context) (createdBy, updatedBy *uuid.UUID) {
	a := actorID(ctx)
	return a, a
}

func updateAudit(ctx context.Context) *uuid.UUID {
	return actorID(ctx)
}

func stampBulkReadings(ctx context.Context, rows []sqlc.BulkCreateCalibrationReadingsParams) {
	a := actorID(ctx)
	for i := range rows {
		rows[i].CreatedBy = a
	}
}
