package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// PmReportRepository reads and writes the whole PM report aggregate:
// rtu.pm_reports plus its children (checklist_results, pm_ground_tests,
// pm_power_tests, pm_power_test_points). Children are always written whole —
// every save replaces the full set for the report, mirroring how
// CalibrationRepository treats calibration_readings.
type PmReportRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var pmReportConstraints = db.Constraints{
	"uk_pm_reports_round_id": httpx.ErrPmReportRoundDup,
	"fk_pm_reports_engineer": httpx.ErrEngineerNotFnd,
}

var checklistResultConstraints = db.Constraints{
	"fk_checklist_results_item":                    httpx.ErrChecklistItemNotFnd,
	"fk_checklist_results_panel_device":            httpx.ErrPanelDeviceNotFnd,
	"uk_checklist_results_report_item_device":      httpx.ErrChecklistResultDup,
	"uk_checklist_results_report_item_null_device": httpx.ErrChecklistResultDup,
}

var powerTestConstraints = db.Constraints{
	"fk_pm_power_tests_instrument": httpx.ErrInstrumentNotFnd,
}

var powerTestPointConstraints = db.Constraints{
	"uk_power_test_points_test_role": httpx.ErrPowerTestPointRoleDup,
}

// PmReportDetail is a PM report with every child collection attached — the
// shape returned to API clients so one call renders the whole report.
type PmReportDetail struct {
	sqlc.PmReport
	ChecklistResults []sqlc.ListChecklistResultsByReportRow `json:"checklist_results"`
	GroundTest       *sqlc.PmGroundTest                     `json:"ground_test"`
	PowerTest        *sqlc.PmPowerTest                      `json:"power_test"`
	PowerTestPoints  []sqlc.PmPowerTestPoint                `json:"power_test_points"`
	Calibrations     []sqlc.Calibration                     `json:"calibrations"`
}

// ChecklistResultInput is one line of the checklist submitted with a report.
type ChecklistResultInput struct {
	ChecklistItemID uuid.UUID
	PanelDeviceID   *uuid.UUID
	Status          *string
	Value           *string
	MeterNo         *string
	Note            *string
	CheckedBy       *uuid.UUID
	CheckedAt       *time.Time
}

// GroundTestInput is the optional ground resistance/voltage test.
type GroundTestInput struct {
	ResistanceLg *decimal.Decimal
	ResistanceNg *decimal.Decimal
	VoltageLg    *decimal.Decimal
	VoltageNg    *decimal.Decimal
	Result       *string
	Note         *string
	MeasuredBy   *uuid.UUID
	MeasuredAt   *time.Time
}

// PowerTestPointInput is one equipment row (breaker or DC supply) of the
// power test.
type PowerTestPointInput struct {
	EquipmentRole     string
	Brand             *string
	Model             *string
	InputAcceptRange  *string
	InputResultValue  *decimal.Decimal
	InputUnit         *string
	OutputAcceptRange *string
	OutputResultValue *decimal.Decimal
	OutputUnit        *string
	Result            *string
	CorrectiveAction  *string
}

// PowerTestInput is the optional power supply test, with its equipment
// points.
type PowerTestInput struct {
	InstrumentID *uuid.UUID
	TestedBy     *uuid.UUID
	TestedAt     *time.Time
	Points       []PowerTestPointInput
}

// SaveInput bundles every part of the aggregate a single save touches.
type SaveInput struct {
	EngineerID *uuid.UUID
	Note       *string
	ReportDate *time.Time
	Checklist  []ChecklistResultInput
	Ground     *GroundTestInput
	Power      *PowerTestInput
}

// GetDetail returns the full aggregate of one report.
func (r *PmReportRepository) GetDetail(ctx context.Context, id uuid.UUID) (PmReportDetail, error) {
	report, err := r.q.GetPmReport(ctx, id)
	if err != nil {
		return PmReportDetail{}, db.Translate(err, db.WithNotFound(httpx.ErrPmReportNotFnd))
	}
	return r.loadDetail(ctx, report)
}

// GetDetailByRound returns the full aggregate of the report tied to one
// work order round, if any has been started yet.
func (r *PmReportRepository) GetDetailByRound(ctx context.Context, roundID uuid.UUID) (PmReportDetail, error) {
	report, err := r.q.GetPmReportByRound(ctx, roundID)
	if err != nil {
		return PmReportDetail{}, db.Translate(err, db.WithNotFound(httpx.ErrPmReportNotFnd))
	}
	return r.loadDetail(ctx, report)
}

func (r *PmReportRepository) loadDetail(ctx context.Context, report sqlc.PmReport) (PmReportDetail, error) {
	detail := PmReportDetail{PmReport: report}

	checklist, err := r.q.ListChecklistResultsByReport(ctx, report.ID)
	if err != nil {
		return PmReportDetail{}, db.Translate(err)
	}
	detail.ChecklistResults = checklist

	ground, err := r.q.GetPmGroundTestByReport(ctx, report.ID)
	if err == nil {
		detail.GroundTest = &ground
	} else if !db.IsNotFound(err) {
		return PmReportDetail{}, db.Translate(err)
	}

	power, err := r.q.GetPmPowerTestByReport(ctx, report.ID)
	if err == nil {
		detail.PowerTest = &power
		points, err := r.q.ListPmPowerTestPointsByTest(ctx, power.ID)
		if err != nil {
			return PmReportDetail{}, db.Translate(err)
		}
		detail.PowerTestPoints = points
	} else if !db.IsNotFound(err) {
		return PmReportDetail{}, db.Translate(err)
	}

	cals, err := r.q.ListCalibrationsForPmReport(ctx, sqlc.ListCalibrationsForPmReportParams{
		PmReportID:  report.ID,
		WorkOrderID: report.WorkOrderID,
	})
	if err != nil {
		return PmReportDetail{}, db.Translate(err)
	}
	detail.Calibrations = cals

	return detail, nil
}

// Save creates the report for a round on first call (status DRAFT) and, on
// every later call while it is still DRAFT, replaces its meta fields and the
// whole child set atomically. Once SUBMITTED the report is immutable here —
// callers get ErrPmReportNotDraft and must reopen a new round to edit again.
func (r *PmReportRepository) Save(
	ctx context.Context, workOrderID, roundID, panelID uuid.UUID, in SaveInput,
) (PmReportDetail, error) {
	createdBy, updatedBy := createAudit(ctx)

	var report sqlc.PmReport
	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		existing, getErr := q.GetPmReportByRound(ctx, roundID)
		switch {
		case getErr == nil:
			if existing.Status != "DRAFT" {
				return httpx.Err(httpx.ErrPmReportNotDraft)
			}
			updated, err := q.UpdatePmReport(ctx, sqlc.UpdatePmReportParams{
				ID:                 existing.ID,
				EngineerID:         in.EngineerID,
				EngineerIDDoUpdate: true,
				Note:               in.Note,
				NoteDoUpdate:       true,
				ReportDate:         in.ReportDate,
				ReportDateDoUpdate: true,
				UpdatedBy:          updatedBy,
			})
			if err != nil {
				return db.Translate(err, db.Options{NotFound: &httpx.ErrPmReportNotFnd, Constraints: pmReportConstraints})
			}
			report = updated
		case db.IsNotFound(getErr):
			created, err := q.CreatePmReport(ctx, sqlc.CreatePmReportParams{
				WorkOrderID:      workOrderID,
				WorkOrderRoundID: roundID,
				PanelID:          panelID,
				EngineerID:       in.EngineerID,
				Note:             in.Note,
				ReportDate:       in.ReportDate,
				CreatedBy:        createdBy,
				UpdatedBy:        updatedBy,
			})
			if err != nil {
				return db.Translate(err, db.Options{Constraints: pmReportConstraints})
			}
			report = created
		default:
			return db.Translate(getErr)
		}

		if err := r.replaceChecklist(ctx, q, report.ID, in.Checklist, createdBy); err != nil {
			return err
		}
		if err := r.replaceGroundTest(ctx, q, report.ID, in.Ground, createdBy, updatedBy); err != nil {
			return err
		}
		if err := r.replacePowerTest(ctx, q, report.ID, in.Power, createdBy, updatedBy); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return PmReportDetail{}, err
	}

	return r.GetDetail(ctx, report.ID)
}

func (r *PmReportRepository) replaceChecklist(ctx context.Context, q *sqlc.Queries, reportID uuid.UUID, items []ChecklistResultInput, createdBy *uuid.UUID) error {
	if _, err := q.DeleteChecklistResultsByReport(ctx, reportID); err != nil {
		return db.Translate(err)
	}
	if len(items) == 0 {
		return nil
	}

	rows := make([]sqlc.BulkCreateChecklistResultsParams, len(items))
	for i, it := range items {
		checkedAt := time.Now()
		if it.CheckedAt != nil {
			checkedAt = *it.CheckedAt
		}
		rows[i] = sqlc.BulkCreateChecklistResultsParams{
			PmReportID:      reportID,
			ChecklistItemID: it.ChecklistItemID,
			PanelDeviceID:   it.PanelDeviceID,
			Status:          it.Status,
			Value:           it.Value,
			MeterNo:         it.MeterNo,
			Note:            it.Note,
			CheckedBy:       it.CheckedBy,
			CheckedAt:       checkedAt,
			CreatedBy:       createdBy,
		}
	}
	if _, err := q.BulkCreateChecklistResults(ctx, rows); err != nil {
		return db.Translate(err, db.Options{Constraints: checklistResultConstraints})
	}
	return nil
}

func (r *PmReportRepository) replaceGroundTest(ctx context.Context, q *sqlc.Queries, reportID uuid.UUID, in *GroundTestInput, createdBy, updatedBy *uuid.UUID) error {
	if in == nil {
		if _, err := q.DeletePmGroundTestByReport(ctx, reportID); err != nil {
			return db.Translate(err)
		}
		return nil
	}

	_, err := q.UpsertPmGroundTest(ctx, sqlc.UpsertPmGroundTestParams{
		PmReportID:   reportID,
		ResistanceLg: in.ResistanceLg,
		ResistanceNg: in.ResistanceNg,
		VoltageLg:    in.VoltageLg,
		VoltageNg:    in.VoltageNg,
		Result:       in.Result,
		Note:         in.Note,
		MeasuredBy:   in.MeasuredBy,
		MeasuredAt:   in.MeasuredAt,
		CreatedBy:    createdBy,
		UpdatedBy:    updatedBy,
	})
	if err != nil {
		return db.Translate(err)
	}
	return nil
}

func (r *PmReportRepository) replacePowerTest(ctx context.Context, q *sqlc.Queries, reportID uuid.UUID, in *PowerTestInput, createdBy, updatedBy *uuid.UUID) error {
	if in == nil {
		existing, err := q.GetPmPowerTestByReport(ctx, reportID)
		if err == nil {
			if _, err := q.DeletePmPowerTestPointsByTest(ctx, existing.ID); err != nil {
				return db.Translate(err)
			}
		} else if !db.IsNotFound(err) {
			return db.Translate(err)
		}
		if _, err := q.DeletePmPowerTestByReport(ctx, reportID); err != nil {
			return db.Translate(err)
		}
		return nil
	}

	test, err := q.UpsertPmPowerTest(ctx, sqlc.UpsertPmPowerTestParams{
		PmReportID:   reportID,
		InstrumentID: in.InstrumentID,
		TestedBy:     in.TestedBy,
		TestedAt:     in.TestedAt,
		CreatedBy:    createdBy,
		UpdatedBy:    updatedBy,
	})
	if err != nil {
		return db.Translate(err, db.Options{Constraints: powerTestConstraints})
	}

	if _, err := q.DeletePmPowerTestPointsByTest(ctx, test.ID); err != nil {
		return db.Translate(err)
	}
	if len(in.Points) == 0 {
		return nil
	}

	rows := make([]sqlc.BulkCreatePmPowerTestPointsParams, len(in.Points))
	for i, pt := range in.Points {
		rows[i] = sqlc.BulkCreatePmPowerTestPointsParams{
			PmPowerTestID:     test.ID,
			EquipmentRole:     pt.EquipmentRole,
			Brand:             pt.Brand,
			Model:             pt.Model,
			InputAcceptRange:  pt.InputAcceptRange,
			InputResultValue:  pt.InputResultValue,
			InputUnit:         pt.InputUnit,
			OutputAcceptRange: pt.OutputAcceptRange,
			OutputResultValue: pt.OutputResultValue,
			OutputUnit:        pt.OutputUnit,
			Result:            pt.Result,
			CorrectiveAction:  pt.CorrectiveAction,
			CreatedBy:         createdBy,
		}
	}
	if _, err := q.BulkCreatePmPowerTestPoints(ctx, rows); err != nil {
		return db.Translate(err, db.Options{Constraints: powerTestPointConstraints})
	}
	return nil
}

// Submit finalizes a DRAFT report: SUBMITTED status, submitted_by/at.
func (r *PmReportRepository) Submit(ctx context.Context, id uuid.UUID, submittedBy uuid.UUID, submittedAt time.Time) (sqlc.PmReport, error) {
	report, err := r.q.SetPmReportSubmitted(ctx, sqlc.SetPmReportSubmittedParams{
		ID:          id,
		SubmittedBy: &submittedBy,
		SubmittedAt: submittedAt,
		UpdatedBy:   &submittedBy,
	})
	if err != nil {
		return sqlc.PmReport{}, db.Translate(err, db.WithNotFound(httpx.ErrPmReportNotFnd))
	}
	return report, nil
}

// Delete removes a report while it is still DRAFT; children cascade.
func (r *PmReportRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeletePmReport(ctx, id)
	if err != nil {
		return db.Translate(err)
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrPmReportNotDraft)
	}
	return nil
}

var pmReportHistorySortable = httpx.Sortable{
	"report_date": "pr.report_date",
	"created_at":  "pr.created_at",
	"round_no":    "wor.round_no",
}

// PmReportHistorySortable lists sort keys accepted by the history endpoints.
func PmReportHistorySortable() httpx.Sortable { return pmReportHistorySortable }

// PmReportHistoryItem is a report row enriched with its work order and
// engineer context, used to render PM history for a panel or a work order.
type PmReportHistoryItem struct {
	sqlc.PmReport
	RoundNo        int16   `db:"round_no" json:"round_no"`
	WorkOrderNo    string  `db:"work_order_no" json:"work_order_no"`
	PmScheduleType *string `db:"pm_schedule_type" json:"pm_schedule_type"`
	EngineerName   *string `db:"engineer_name" json:"engineer_name"`
	TotalCount     int64   `db:"total_count" json:"-"`
}

const pmReportHistorySelect = `
SELECT
    pr.id, pr.work_order_id, pr.work_order_round_id, pr.panel_id, pr.engineer_id,
    pr.submitted_by, pr.status, pr.note, pr.report_date, pr.submitted_at,
    pr.created_at, pr.updated_at, pr.created_by, pr.updated_by,
    wor.round_no AS round_no,
    wo.work_order_no AS work_order_no,
    wo.pm_schedule_type AS pm_schedule_type,
    e.full_name AS engineer_name,
    count(*) OVER ()::bigint AS total_count
FROM rtu.pm_reports pr
JOIN rtu.work_orders wo ON wo.id = pr.work_order_id
JOIN rtu.work_order_rounds wor ON wor.id = pr.work_order_round_id
LEFT JOIN rtu.engineers e ON e.id = pr.engineer_id
WHERE %s
ORDER BY %s %s, pr.id %s
LIMIT %s OFFSET %s`

// ListByPanel returns the PM report history of a panel — every PM ever
// submitted for it, newest first by default.
func (r *PmReportRepository) ListByPanel(ctx context.Context, panelID uuid.UUID, page httpx.Page) ([]PmReportHistoryItem, int64, error) {
	a := &args{}
	conds := conditions{"pr.panel_id = " + a.add(panelID)}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(`(wo.work_order_no ILIKE %s ESCAPE '\' OR e.full_name ILIKE %s ESCAPE '\')`, p, p))
	}

	query := fmt.Sprintf(pmReportHistorySelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[PmReportHistoryItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// ListByWorkOrder returns every report of a work order across all of its
// rounds (i.e. rework history), newest round first.
func (r *PmReportRepository) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]PmReportHistoryItem, error) {
	a := &args{}
	conds := conditions{"pr.work_order_id = " + a.add(workOrderID)}
	page := httpx.Page{Number: 1, Limit: 1000, SortSQL: "wor.round_no", Order: "DESC"}

	query := fmt.Sprintf(pmReportHistorySelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, db.Translate(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[PmReportHistoryItem])
	if err != nil {
		return nil, db.Translate(err)
	}
	return items, nil
}
