package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// PanelImageRepository reads and writes rtu.panel_images.
type PanelImageRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var panelImageSortable = httpx.Sortable{
	"sort_order": "pi.sort_order",
	"created_at": "pi.created_at",
	"image_type": "pi.image_type",
}

// PanelImageSortable lists sort keys accepted by the image list endpoint.
func PanelImageSortable() httpx.Sortable { return panelImageSortable }

// PanelImageFilter narrows an image list query.
type PanelImageFilter struct {
	PanelID   uuid.UUID
	ImageType *string
}

const panelImageListSelect = `
SELECT
    pi.id, pi.panel_id, pi.image_type,
    pi.s3_bucket, pi.s3_key, pi.original_name, pi.mime_type, pi.file_size,
    pi.caption, pi.sort_order, pi.active,
    pi.created_at, pi.updated_at, pi.created_by, pi.updated_by,
    count(*) OVER ()::bigint AS total_count
FROM rtu.panel_images pi
WHERE %s
ORDER BY %s %s, pi.id %s
LIMIT %s OFFSET %s`

// List returns one page of panel images.
func (r *PanelImageRepository) List(ctx context.Context, page httpx.Page, filter PanelImageFilter) ([]PanelImageListItem, int64, error) {
	a := &args{}
	conds := conditions{"pi.panel_id = " + a.add(filter.PanelID), "pi.active = true"}

	if filter.ImageType != nil {
		conds = append(conds, "pi.image_type = "+a.add(*filter.ImageType))
	}

	query := fmt.Sprintf(panelImageListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[PanelImageListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// PanelImageListItem is a panel image row with the list total count.
type PanelImageListItem struct {
	sqlc.PanelImage
	TotalCount int64 `db:"total_count" json:"-"`
}

// GetForPanel returns an image scoped to a panel.
func (r *PanelImageRepository) GetForPanel(ctx context.Context, panelID, id uuid.UUID) (sqlc.PanelImage, error) {
	img, err := r.q.GetPanelImageForPanel(ctx, sqlc.GetPanelImageForPanelParams{
		ID: id, PanelID: panelID,
	})
	if err != nil {
		return sqlc.PanelImage{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelImageNotFnd))
	}
	return img, nil
}

// Create inserts a panel image record.
func (r *PanelImageRepository) Create(ctx context.Context, arg sqlc.CreatePanelImageParams) (sqlc.PanelImage, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	img, err := r.q.CreatePanelImage(ctx, arg)
	if err != nil {
		return sqlc.PanelImage{}, db.Translate(err)
	}
	return img, nil
}

// Update applies metadata changes.
func (r *PanelImageRepository) Update(ctx context.Context, arg sqlc.UpdatePanelImageParams) (sqlc.PanelImage, error) {
	arg.UpdatedBy = updateAudit(ctx)
	img, err := r.q.UpdatePanelImage(ctx, arg)
	if err != nil {
		return sqlc.PanelImage{}, db.Translate(err, db.Options{
			NotFound: &httpx.ErrPanelImageNotFnd,
		})
	}
	return img, nil
}

// ReplaceFile updates S3 metadata after a file swap.
func (r *PanelImageRepository) ReplaceFile(ctx context.Context, arg sqlc.ReplacePanelImageFileParams) (sqlc.PanelImage, error) {
	arg.UpdatedBy = updateAudit(ctx)
	img, err := r.q.ReplacePanelImageFile(ctx, arg)
	if err != nil {
		return sqlc.PanelImage{}, db.Translate(err, db.Options{
			NotFound: &httpx.ErrPanelImageNotFnd,
		})
	}
	return img, nil
}

// Delete removes an image record permanently.
func (r *PanelImageRepository) Delete(ctx context.Context, panelID, id uuid.UUID) (sqlc.PanelImage, error) {
	img, err := r.GetForPanel(ctx, panelID, id)
	if err != nil {
		return sqlc.PanelImage{}, err
	}
	affected, err := r.q.DeletePanelImage(ctx, sqlc.DeletePanelImageParams{ID: id, PanelID: panelID})
	if err != nil {
		return sqlc.PanelImage{}, db.Translate(err)
	}
	if affected == 0 {
		return sqlc.PanelImage{}, httpx.Err(httpx.ErrPanelImageNotFnd)
	}
	return img, nil
}
