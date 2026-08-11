package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// AttachmentRepository reads and writes rtu.attachments — the polymorphic
// file table shared by work orders, PM/CM reports, calibrations and their
// sub-records (see entity_type in doc/rtu-full-schema.dbml). entity_id has
// no FK (it points at different tables depending on entity_type), so
// existence of the referenced row is checked by the service layer.
type AttachmentRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// EntityExists checks that the row an attachment would be attached to
// actually exists. entity_id has no FK (rtu.attachments is polymorphic), so
// this is the only guard against orphaned attachments.
func (r *AttachmentRepository) EntityExists(ctx context.Context, entityType string, entityID uuid.UUID) (bool, error) {
	var (
		found bool
		err   error
	)
	switch entityType {
	case "WORK_ORDER":
		found, err = r.q.WorkOrderExists(ctx, entityID)
	case "PM_REPORT":
		found, err = r.q.PmReportExists(ctx, entityID)
	case "CM_REPORT":
		found, err = r.q.CmReportExists(ctx, entityID)
	case "CALIBRATION":
		found, err = r.q.CalibrationExists(ctx, entityID)
	case "PM_GROUND_TEST":
		found, err = r.q.PmGroundTestExists(ctx, entityID)
	case "PM_POWER_TEST_POINT":
		found, err = r.q.PmPowerTestPointExists(ctx, entityID)
	case "PANEL_DEVICE":
		found, err = r.q.PanelDeviceExists(ctx, entityID)
	default:
		return false, httpx.Err(httpx.ErrAttachmentEntityInvalid)
	}
	if err != nil {
		return false, db.Translate(err)
	}
	return found, nil
}

// Get returns a single attachment by id.
func (r *AttachmentRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.Attachment, error) {
	att, err := r.q.GetAttachment(ctx, id)
	if err != nil {
		return sqlc.Attachment{}, db.Translate(err, db.WithNotFound(httpx.ErrAttachmentNotFnd))
	}
	return att, nil
}

// ListByEntity returns every attachment of one entity, oldest first.
func (r *AttachmentRepository) ListByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]sqlc.Attachment, error) {
	items, err := r.q.ListAttachmentsByEntity(ctx, sqlc.ListAttachmentsByEntityParams{
		EntityType: entityType, EntityID: entityID,
	})
	if err != nil {
		return nil, db.Translate(err)
	}
	return items, nil
}

// Create records a new attachment after its file has been uploaded to S3.
func (r *AttachmentRepository) Create(ctx context.Context, arg sqlc.CreateAttachmentParams) (sqlc.Attachment, error) {
	att, err := r.q.CreateAttachment(ctx, arg)
	if err != nil {
		return sqlc.Attachment{}, db.Translate(err)
	}
	return att, nil
}

// UpdateCaption changes the caption of an attachment.
func (r *AttachmentRepository) UpdateCaption(ctx context.Context, id uuid.UUID, caption *string) (sqlc.Attachment, error) {
	att, err := r.q.UpdateAttachmentCaption(ctx, sqlc.UpdateAttachmentCaptionParams{
		ID: id, Caption: caption, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.Attachment{}, db.Translate(err, db.WithNotFound(httpx.ErrAttachmentNotFnd))
	}
	return att, nil
}

// Delete removes an attachment's DB row. The caller is responsible for
// deleting the underlying S3 object first (see AttachmentService.Delete).
func (r *AttachmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteAttachment(ctx, id)
	if err != nil {
		return db.Translate(err)
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrAttachmentNotFnd)
	}
	return nil
}
