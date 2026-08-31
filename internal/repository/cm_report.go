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

// CmReportRepository reads and writes rtu.cm_reports. A report has one of
// three origins, distinguished by which nullable FK is set (see the table
// note in doc/rtu-full-schema.dbml):
//
//	STANDALONE / PM_ESCALATED: work_order_id + work_order_round_id set,
//	  pm_report_id NULL — one report per CM work order round, exactly like
//	  pm_reports.
//	PM_ONSITE_FIX: pm_report_id set, work_order_id NULL — a fix made on the
//	  spot during a PM visit, recorded directly against the PM report with
//	  no work order of its own.
type CmReportRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var cmReportConstraints = db.Constraints{
	"uk_cm_reports_round_id":      httpx.ErrCmReportRoundDup,
	"fk_cm_reports_panel_device":  httpx.ErrPanelDeviceNotFnd,
	"fk_cm_reports_pm_report":     httpx.ErrPmReportNotFnd,
	"fk_cm_reports_problem_topic": httpx.ErrProblemTopicNotFnd,
}

// Get returns a single CM report by id.
func (r *CmReportRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.CmReport, error) {
	report, err := r.q.GetCmReport(ctx, id)
	if err != nil {
		return sqlc.CmReport{}, db.Translate(err, db.WithNotFound(httpx.ErrCmReportNotFnd))
	}
	return report, nil
}

// GetByRound returns the CM report of a work order round.
func (r *CmReportRepository) GetByRound(ctx context.Context, roundID uuid.UUID) (sqlc.CmReport, error) {
	report, err := r.q.GetCmReportByRound(ctx, roundID)
	if err != nil {
		return sqlc.CmReport{}, db.Translate(err, db.WithNotFound(httpx.ErrCmReportNotFnd))
	}
	return report, nil
}

// FindByRound returns the CM report of a work order round, or nil if the
// round has none yet — used to decide between create and update without
// forcing callers to pattern-match on a not-found error.
func (r *CmReportRepository) FindByRound(ctx context.Context, roundID uuid.UUID) (*sqlc.CmReport, error) {
	report, err := r.q.GetCmReportByRound(ctx, roundID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, db.Translate(err)
	}
	return &report, nil
}

// ListByPmReport returns every onsite-fix report recorded against a PM
// visit (one per device that got fixed on the spot).
func (r *CmReportRepository) ListByPmReport(ctx context.Context, pmReportID uuid.UUID) ([]sqlc.CmReport, error) {
	reports, err := r.q.ListCmReportsByPmReport(ctx, pmReportID)
	if err != nil {
		return nil, db.Translate(err)
	}
	return reports, nil
}

// Create inserts a CM report for either origin.
func (r *CmReportRepository) Create(ctx context.Context, arg sqlc.CreateCmReportParams) (sqlc.CmReport, error) {
	return r.CreateQ(ctx, r.q, arg)
}

// CreateQ inserts a CM report using the given Queries handle (for use inside a transaction).
func (r *CmReportRepository) CreateQ(ctx context.Context, q *sqlc.Queries, arg sqlc.CreateCmReportParams) (sqlc.CmReport, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	report, err := q.CreateCmReport(ctx, arg)
	if err != nil {
		return sqlc.CmReport{}, db.Translate(err, db.Options{Constraints: cmReportConstraints})
	}
	return report, nil
}

// Update applies a partial update.
func (r *CmReportRepository) Update(ctx context.Context, arg sqlc.UpdateCmReportParams) (sqlc.CmReport, error) {
	return r.UpdateQ(ctx, r.q, arg)
}

// UpdateQ applies a partial update using the given Queries handle.
func (r *CmReportRepository) UpdateQ(ctx context.Context, q *sqlc.Queries, arg sqlc.UpdateCmReportParams) (sqlc.CmReport, error) {
	arg.UpdatedBy = updateAudit(ctx)
	report, err := q.UpdateCmReport(ctx, arg)
	if err != nil {
		return sqlc.CmReport{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrCmReportNotFnd,
			Constraints: cmReportConstraints,
		})
	}
	return report, nil
}

// WithPanelCmLock runs fn inside a transaction serialized per panel for CM writes.
func (r *CmReportRepository) WithPanelCmLock(ctx context.Context, panelID uuid.UUID, fn func(tx pgx.Tx, q *sqlc.Queries) error) error {
	return db.InTxConn(ctx, r.pool, func(tx pgx.Tx, q *sqlc.Queries) error {
		if err := LockPanelCmWrites(ctx, tx, panelID); err != nil {
			return err
		}
		return fn(tx, q)
	})
}

// FindByRoundQ returns the CM report for a round using the given Queries handle.
func (r *CmReportRepository) FindByRoundQ(ctx context.Context, q *sqlc.Queries, roundID uuid.UUID) (*sqlc.CmReport, error) {
	report, err := q.GetCmReportByRound(ctx, roundID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, db.Translate(err)
	}
	return &report, nil
}

// Delete removes a CM report permanently.
func (r *CmReportRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteCmReport(ctx, id)
	if err != nil {
		return db.Translate(err)
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrCmReportNotFnd)
	}
	return nil
}

var cmReportHistorySortable = httpx.Sortable{
	"reported_at": "cr.reported_at",
	"started_at":  "cr.started_at",
	"ended_at":    "cr.ended_at",
	"created_at":  "cr.created_at",
	"round_no":    "wor.round_no",
}

// CmReportHistorySortable lists sort keys accepted by the history endpoints.
func CmReportHistorySortable() httpx.Sortable { return cmReportHistorySortable }

// CmReportHistoryItem is a report row enriched with its work order/round and
// PM-visit context, used to render repair history for a panel or a device.
type CmReportHistoryItem struct {
	sqlc.CmReport
	WorkOrderNo *string `db:"work_order_no" json:"work_order_no"`
	RoundNo     *int16  `db:"round_no" json:"round_no"`
	TotalCount  int64   `db:"total_count" json:"-"`
}

const cmReportHistorySelect = `
SELECT
    cr.id, cr.work_order_id, cr.work_order_round_id, cr.pm_report_id, cr.panel_id, cr.panel_device_id,
    cr.reported_by, cr.tag_code, cr.error_logs, cr.problem_detail, cr.root_cause, cr.reference_info,
    cr.corrective_action, cr.recommendation, cr.pending_reason, cr.repaired_by,
    cr.reported_at, cr.started_at, cr.ended_at,
    cr.created_at, cr.updated_at, cr.created_by, cr.updated_by,
    wo.work_order_no AS work_order_no,
    wor.round_no AS round_no,
    count(*) OVER ()::bigint AS total_count
FROM rtu.cm_reports cr
LEFT JOIN rtu.work_orders wo ON wo.id = cr.work_order_id
LEFT JOIN rtu.work_order_rounds wor ON wor.id = cr.work_order_round_id
WHERE %s
ORDER BY %s %s NULLS LAST, cr.id %s
LIMIT %s OFFSET %s`

// ListByPanel returns the CM (repair) history of a panel.
func (r *CmReportRepository) ListByPanel(ctx context.Context, panelID uuid.UUID, page httpx.Page) ([]CmReportHistoryItem, int64, error) {
	a := &args{}
	conds := conditions{"cr.panel_id = " + a.add(panelID)}
	return r.listHistory(ctx, a, conds, page)
}

// ListByPanelDevice returns the repair history of a single device — when it
// was fixed, by whom and what was done.
func (r *CmReportRepository) ListByPanelDevice(ctx context.Context, panelDeviceID uuid.UUID, page httpx.Page) ([]CmReportHistoryItem, int64, error) {
	a := &args{}
	conds := conditions{"cr.panel_device_id = " + a.add(panelDeviceID)}
	return r.listHistory(ctx, a, conds, page)
}

func (r *CmReportRepository) listHistory(ctx context.Context, a *args, conds conditions, page httpx.Page) ([]CmReportHistoryItem, int64, error) {
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(cr.problem_detail ILIKE %s ESCAPE '\' OR cr.root_cause ILIKE %s ESCAPE '\' OR wo.work_order_no ILIKE %s ESCAPE '\')`,
			p, p, p,
		))
	}

	query := fmt.Sprintf(cmReportHistorySelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[CmReportHistoryItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// ListByWorkOrder returns every CM report across a work order's rounds
// (rework history), newest round first.
func (r *CmReportRepository) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]CmReportHistoryItem, error) {
	a := &args{}
	conds := conditions{"cr.work_order_id = " + a.add(workOrderID)}
	page := httpx.Page{Number: 1, Limit: 1000, SortSQL: "wor.round_no", Order: "DESC"}

	items, _, err := r.listHistory(ctx, a, conds, page)
	if err != nil {
		return nil, err
	}
	return items, nil
}
