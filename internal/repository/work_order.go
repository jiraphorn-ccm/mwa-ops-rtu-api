package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/domain"
	"github.com/rtu-api/internal/httpx"
)

// WorkOrderRepository reads and writes rtu.work_orders, together with the
// creation of a work order's first round (see CreateWithFirstRound).
type WorkOrderRepository struct {
	pool   *pgxpool.Pool
	q      *sqlc.Queries
	topics *WorkOrderProblemTopicRepository
}

var workOrderConstraints = db.Constraints{
	"uk_work_orders_no":               httpx.ErrWorkOrderNoDup,
	"fk_work_orders_panel":            httpx.ErrPanelNotFound,
	"fk_work_orders_panel_device":     httpx.ErrPanelDeviceNotFnd,
	"fk_work_orders_related":          httpx.ErrWorkOrderNotFnd,
	"ck_work_orders_pm_schedule":      httpx.ErrPmScheduleTypeRequired,
	"ck_work_orders_pm_schedule_type": httpx.ErrPmScheduleTypeRequired,
}

var workOrderSortable = httpx.Sortable{
	"work_order_no":   "wo.work_order_no",
	"work_order_type": "wo.work_order_type",
	"status":          "wo.status",
	"priority":        "wo.priority",
	"planned_date":    "wo.planned_date",
	"due_date":        "wo.due_date",
	"panel_code":      "p.code",
	"created_at":      "wo.created_at",
	"updated_at":      "wo.updated_at",
}

// WorkOrderSortable lists the sort keys accepted by the list endpoint.
func WorkOrderSortable() httpx.Sortable { return workOrderSortable }

// WorkOrderFilter narrows a work order list query.
type WorkOrderFilter struct {
	WorkOrderType  *string
	PmScheduleType *string
	Status         *string
	Statuses       []string
	Priority       *string
	PanelID        *uuid.UUID
	PanelDeviceID  *uuid.UUID
	ProblemTopicID *uuid.UUID
	AssignedTo     *uuid.UUID
	Active         *bool
	PlannedFrom    *time.Time
	PlannedTo      *time.Time
	DueFrom        *time.Time
	DueTo          *time.Time
}

// WorkOrderView is a work order joined with its panel and the state of its
// current round (assignee, check-in/out, submission). CM rows include
// problem_topics loaded from rtu.work_order_problem_topics.
type WorkOrderView struct {
	sqlc.WorkOrder
	PanelCode          string              `db:"panel_code" json:"panel_code"`
	PanelDeviceTagName *string             `db:"panel_device_tag_name" json:"panel_device_tag_name"`
	ProblemTopicID     *uuid.UUID          `json:"problem_topic_id,omitempty"`
	ProblemTopicCode   *string             `json:"problem_topic_code,omitempty"`
	ProblemTopicName   *string             `json:"problem_topic_name,omitempty"`
	ProblemTopics      []ProblemTopicBrief `json:"problem_topics"`
	CurrentRoundNo     *int16              `db:"current_round_no" json:"current_round_no"`
	CurrentAssignedTo  *uuid.UUID          `db:"current_assigned_to" json:"current_assigned_to"`
	CurrentAssignedAt  *time.Time          `db:"current_assigned_at" json:"current_assigned_at"`
	CurrentCheckInAt   *time.Time          `db:"current_check_in_at" json:"current_check_in_at"`
	CurrentCheckOutAt  *time.Time          `db:"current_check_out_at" json:"current_check_out_at"`
	CurrentSubmittedAt *time.Time          `db:"current_submitted_at" json:"current_submitted_at"`
	TotalCount         int64               `db:"total_count" json:"-"`
}

const workOrderColumns = `
    wo.id, wo.work_order_no, wo.work_order_type, wo.pm_schedule_type, wo.panel_id, wo.panel_device_id,
    wo.title, wo.description, wo.status, wo.priority, wo.source, wo.requested_by, wo.current_round_id,
    wo.related_work_order_id, wo.planned_date, wo.due_date, wo.closed_at, wo.active,
    wo.created_at, wo.updated_at, wo.created_by, wo.updated_by,
    p.code AS panel_code,
    pd.tag_name AS panel_device_tag_name,
    cr.round_no AS current_round_no,
    cr.assigned_to AS current_assigned_to,
    cr.assigned_at AS current_assigned_at,
    cr.check_in_at AS current_check_in_at,
    cr.check_out_at AS current_check_out_at,
    cr.submitted_at AS current_submitted_at`

const workOrderFrom = `
FROM rtu.work_orders wo
JOIN rtu.panels p ON p.id = wo.panel_id
LEFT JOIN rtu.panel_devices pd ON pd.id = wo.panel_device_id
LEFT JOIN rtu.work_order_rounds cr ON cr.id = wo.current_round_id`

// List returns one page of work orders together with the total row count.
func (r *WorkOrderRepository) List(ctx context.Context, page httpx.Page, filter WorkOrderFilter) ([]WorkOrderView, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.WorkOrderType != nil {
		conds = append(conds, "wo.work_order_type = "+a.add(*filter.WorkOrderType))
	}
	if filter.PmScheduleType != nil {
		conds = append(conds, "wo.pm_schedule_type = "+a.add(*filter.PmScheduleType))
	}
	if len(filter.Statuses) > 0 {
		conds = append(conds, "wo.status = ANY("+a.add(filter.Statuses)+")")
	} else if filter.Status != nil {
		conds = append(conds, "wo.status = "+a.add(*filter.Status))
	}
	if filter.Priority != nil {
		conds = append(conds, "wo.priority = "+a.add(*filter.Priority))
	}
	if filter.PanelID != nil {
		conds = append(conds, "wo.panel_id = "+a.add(*filter.PanelID))
	}
	if filter.PanelDeviceID != nil {
		conds = append(conds, "wo.panel_device_id = "+a.add(*filter.PanelDeviceID))
	}
	if filter.ProblemTopicID != nil {
		topic := a.add(*filter.ProblemTopicID)
		conds = append(conds, fmt.Sprintf(`EXISTS (
    SELECT 1 FROM rtu.work_order_problem_topics wopt
    WHERE wopt.work_order_id = wo.id AND wopt.problem_topic_id = %s
)`, topic))
	}
	if filter.AssignedTo != nil {
		conds = append(conds, "cr.assigned_to = "+a.add(*filter.AssignedTo))
	}
	if filter.Active != nil {
		conds = append(conds, "wo.active = "+a.add(*filter.Active))
	}
	if filter.PlannedFrom != nil {
		conds = append(conds, "wo.planned_date >= "+a.add(*filter.PlannedFrom))
	}
	if filter.PlannedTo != nil {
		conds = append(conds, "wo.planned_date <= "+a.add(*filter.PlannedTo))
	}
	if filter.DueFrom != nil {
		conds = append(conds, "wo.due_date >= "+a.add(*filter.DueFrom))
	}
	if filter.DueTo != nil {
		conds = append(conds, "wo.due_date <= "+a.add(*filter.DueTo))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(wo.work_order_no ILIKE %s ESCAPE '\' OR wo.title ILIKE %s ESCAPE '\' OR p.code ILIKE %s ESCAPE '\')`,
			p, p, p,
		))
	}

	query := fmt.Sprintf(
		"SELECT %s,\n    count(*) OVER ()::bigint AS total_count%s\nWHERE %s\nORDER BY %s %s NULLS LAST, wo.id %s\nLIMIT %s OFFSET %s",
		workOrderColumns, workOrderFrom, conds.where(),
		page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[WorkOrderView])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	if err := r.attachProblemTopics(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetView returns a single work order with its joined context.
func (r *WorkOrderRepository) GetView(ctx context.Context, id uuid.UUID) (WorkOrderView, error) {
	query := fmt.Sprintf("SELECT %s%s\nWHERE wo.id = $1", workOrderColumns, workOrderFrom)

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return WorkOrderView{}, db.Translate(err)
	}

	view, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[WorkOrderView])
	if err != nil {
		return WorkOrderView{}, db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderNotFnd))
	}
	views := []WorkOrderView{view}
	if err := r.attachProblemTopics(ctx, views); err != nil {
		return WorkOrderView{}, err
	}
	return views[0], nil
}

func (r *WorkOrderRepository) attachProblemTopics(ctx context.Context, views []WorkOrderView) error {
	if r.topics == nil || len(views) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(views))
	for i := range views {
		ids[i] = views[i].ID
	}
	topics, err := r.topics.MapByWorkOrders(ctx, ids)
	if err != nil {
		return err
	}
	applyProblemTopicsToViews(views, topics)
	return nil
}

// Get returns the raw row of a work order.
func (r *WorkOrderRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.WorkOrder, error) {
	wo, err := r.q.GetWorkOrder(ctx, id)
	if err != nil {
		return sqlc.WorkOrder{}, db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderNotFnd))
	}
	return wo, nil
}

// CountByPanelAndType returns how many work orders of a type exist on a panel.
// Used inside CreateWithFirstRound to allocate the next sequence number.
func (r *WorkOrderRepository) CountByPanelAndType(ctx context.Context, panelID uuid.UUID, workOrderType string) (int64, error) {
	total, err := r.q.CountWorkOrdersByPanelAndType(ctx, sqlc.CountWorkOrdersByPanelAndTypeParams{
		PanelID:       panelID,
		WorkOrderType: workOrderType,
	})
	if err != nil {
		return 0, db.Translate(err)
	}
	return total, nil
}

// FindOpenForPanel returns the id of an open (not completed/cancelled) work
// order of the given type for a panel, if any — used to decide whether a
// rejected PM should reuse an existing CM work order or spawn a new one.
func (r *WorkOrderRepository) FindOpenForPanel(ctx context.Context, panelID uuid.UUID, workOrderType string) (*uuid.UUID, error) {
	id, err := r.q.CountOpenWorkOrdersForPanel(ctx, sqlc.CountOpenWorkOrdersForPanelParams{
		PanelID:       panelID,
		WorkOrderType: workOrderType,
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, db.Translate(err)
	}
	return &id, nil
}

// FindReusableCMForPanel returns a PENDING CM work order on the panel whose
// current round has no CM report yet — safe to attach a new escalation.
func (r *WorkOrderRepository) FindReusableCMForPanel(ctx context.Context, panelID uuid.UUID) (*uuid.UUID, error) {
	id, err := r.q.FindReusableCmWorkOrderForPanel(ctx, panelID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, db.Translate(err)
	}
	return &id, nil
}

// CreateWithFirstRound inserts a work order together with its first round
// (round_no = 1), points current_round_id at it and records the ASSIGNED
// activity — all inside one transaction so a work order is never observed
// without a round. work_order_no is allocated here via domain.FormatWorkOrderNo.
func (r *WorkOrderRepository) CreateWithFirstRound(
	ctx context.Context,
	woArg sqlc.CreateWorkOrderParams,
	panelCode string,
	assignedTo, assignedBy uuid.UUID,
	assignedAt time.Time,
	actorID uuid.UUID,
	initialCmReport *sqlc.CreateCmReportParams,
	openCmDuplicate *OpenCmDuplicateCheck,
	problemTopicIDs []uuid.UUID,
) (sqlc.WorkOrder, sqlc.WorkOrderRound, error) {
	createdBy, updatedBy := createAudit(ctx)
	woArg.CreatedBy, woArg.UpdatedBy = createdBy, updatedBy

	var (
		wo    sqlc.WorkOrder
		round sqlc.WorkOrderRound
	)

	err := db.InTxConn(ctx, r.pool, func(tx pgx.Tx, q *sqlc.Queries) error {
		if openCmDuplicate != nil && len(openCmDuplicate.TopicIDs) > 0 {
			if err := LockPanelCmWrites(ctx, tx, woArg.PanelID); err != nil {
				return err
			}
			if err := EnsureNoOpenCmConflictForTopics(ctx, tx, woArg.PanelID, *openCmDuplicate); err != nil {
				return err
			}
		}

		total, err := q.CountWorkOrdersByPanelAndType(ctx, sqlc.CountWorkOrdersByPanelAndTypeParams{
			PanelID:       woArg.PanelID,
			WorkOrderType: woArg.WorkOrderType,
		})
		if err != nil {
			return db.Translate(err)
		}
		woArg.WorkOrderNo = domain.FormatWorkOrderNo(woArg.WorkOrderType, panelCode, total+1)

		created, err := q.CreateWorkOrder(ctx, woArg)
		if err != nil {
			return db.Translate(err, db.Options{Constraints: workOrderConstraints})
		}
		wo = created

		round, err = q.CreateWorkOrderRound(ctx, sqlc.CreateWorkOrderRoundParams{
			WorkOrderID: created.ID,
			RoundNo:     1,
			AssignedTo:  assignedTo,
			AssignedBy:  assignedBy,
			AssignedAt:  assignedAt,
			CreatedBy:   createdBy,
			UpdatedBy:   updatedBy,
		})
		if err != nil {
			return db.Translate(err)
		}

		wo, err = q.SetWorkOrderCurrentRound(ctx, sqlc.SetWorkOrderCurrentRoundParams{
			ID:             created.ID,
			CurrentRoundID: round.ID,
			UpdatedBy:      updatedBy,
		})
		if err != nil {
			return db.Translate(err)
		}

		toStatus := wo.Status
		if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID:      created.ID,
			WorkOrderRoundID: &round.ID,
			Action:           "ASSIGNED",
			ToStatus:         &toStatus,
			ToAssignee:       &assignedTo,
			ActorID:          actorID,
		}); err != nil {
			return db.Translate(err)
		}

		if initialCmReport != nil {
			woID := created.ID
			roundID := round.ID
			initialCmReport.WorkOrderID = &woID
			initialCmReport.WorkOrderRoundID = &roundID
			initialCmReport.PanelID = created.PanelID
			initialCmReport.CreatedBy = createdBy
			initialCmReport.UpdatedBy = updatedBy
			if _, err := q.CreateCmReport(ctx, *initialCmReport); err != nil {
				return db.Translate(err, db.Options{Constraints: cmReportConstraints})
			}
		}
		if len(problemTopicIDs) > 0 && r.topics != nil {
			if err := r.topics.InsertAll(ctx, tx, created.ID, problemTopicIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var conflict *OpenCmConflictError
		if errors.As(err, &conflict) {
			return sqlc.WorkOrder{}, sqlc.WorkOrderRound{}, err
		}
		return sqlc.WorkOrder{}, sqlc.WorkOrderRound{}, db.Translate(err)
	}

	return wo, round, nil
}

// OpenNewRound closes out a rejected round by opening round_no+1 on the same
// work order, pointing current_round_id at it, moving the work order back to
// newStatus (PENDING for rework, unchanged for a CM escalation that stays on
// its own approval track) and recording the ASSIGNED activity. Everything
// commits in one transaction so the work order is never left pointing at a
// round that failed to insert.
func (r *WorkOrderRepository) OpenNewRound(
	ctx context.Context,
	workOrderID uuid.UUID,
	assignedTo, assignedBy uuid.UUID,
	assignedAt time.Time,
	newStatus string,
	actorID uuid.UUID,
	fromStatus string,
) (sqlc.WorkOrder, sqlc.WorkOrderRound, error) {
	createdBy, updatedBy := createAudit(ctx)

	var (
		wo    sqlc.WorkOrder
		round sqlc.WorkOrderRound
	)

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		nextNo, err := q.NextWorkOrderRoundNo(ctx, workOrderID)
		if err != nil {
			return db.Translate(err)
		}

		round, err = q.CreateWorkOrderRound(ctx, sqlc.CreateWorkOrderRoundParams{
			WorkOrderID: workOrderID,
			RoundNo:     nextNo,
			AssignedTo:  assignedTo,
			AssignedBy:  assignedBy,
			AssignedAt:  assignedAt,
			CreatedBy:   createdBy,
			UpdatedBy:   updatedBy,
		})
		if err != nil {
			return db.Translate(err)
		}

		if _, err := q.SetWorkOrderCurrentRound(ctx, sqlc.SetWorkOrderCurrentRoundParams{
			ID:             workOrderID,
			CurrentRoundID: round.ID,
			UpdatedBy:      updatedBy,
		}); err != nil {
			return db.Translate(err)
		}

		wo, err = q.UpdateWorkOrderStatus(ctx, sqlc.UpdateWorkOrderStatusParams{
			ID: workOrderID, Status: newStatus, UpdatedBy: updatedBy,
		})
		if err != nil {
			return db.Translate(err)
		}

		from := fromStatus
		to := wo.Status
		if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID:      workOrderID,
			WorkOrderRoundID: &round.ID,
			Action:           "ASSIGNED",
			FromStatus:       &from,
			ToStatus:         &to,
			ToAssignee:       &assignedTo,
			ActorID:          actorID,
		}); err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return sqlc.WorkOrder{}, sqlc.WorkOrderRound{}, db.Translate(err)
	}

	return wo, round, nil
}

// SyncProblemTopicFromReport aligns work_order_problem_topics when a CM report
// topic changes. Pass tx when already inside a panel CM transaction.
func (r *WorkOrderRepository) SyncProblemTopicFromReport(
	ctx context.Context,
	tx pgx.Tx,
	workOrderID uuid.UUID,
	newTopicID *uuid.UUID,
) error {
	sync := func(qtx pgx.Tx) error {
		return r.topics.SyncFromReport(ctx, qtx, workOrderID, newTopicID)
	}
	if tx != nil {
		return sync(tx)
	}
	return db.InTxConn(ctx, r.pool, func(qtx pgx.Tx, _ *sqlc.Queries) error {
		return sync(qtx)
	})
}

// SyncProblemTopicsFromReport adds each report topic to the work order junction.
func (r *WorkOrderRepository) SyncProblemTopicsFromReport(
	ctx context.Context,
	tx pgx.Tx,
	workOrderID uuid.UUID,
	topicIDs []uuid.UUID,
) error {
	sync := func(qtx pgx.Tx) error {
		return r.topics.SyncAllFromReport(ctx, qtx, workOrderID, topicIDs)
	}
	if tx != nil {
		return sync(tx)
	}
	return db.InTxConn(ctx, r.pool, func(qtx pgx.Tx, _ *sqlc.Queries) error {
		return sync(qtx)
	})
}

// Update applies a partial update to a work order.
func (r *WorkOrderRepository) Update(ctx context.Context, arg sqlc.UpdateWorkOrderParams) (sqlc.WorkOrder, error) {
	return r.UpdateQ(ctx, r.q, arg)
}

// UpdateQ applies a partial update using the given Queries handle.
func (r *WorkOrderRepository) UpdateQ(ctx context.Context, q *sqlc.Queries, arg sqlc.UpdateWorkOrderParams) (sqlc.WorkOrder, error) {
	arg.UpdatedBy = updateAudit(ctx)
	wo, err := q.UpdateWorkOrder(ctx, arg)
	if err != nil {
		return sqlc.WorkOrder{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrWorkOrderNotFnd,
			Constraints: workOrderConstraints,
		})
	}
	return wo, nil
}

// ReplaceCmProblemTopics atomically replaces junction topics for an open CM work
// order, optionally applying a work-order field update in the same transaction.
func (r *WorkOrderRepository) ReplaceCmProblemTopics(
	ctx context.Context,
	panelID, workOrderID uuid.UUID,
	newTopicIDs []uuid.UUID,
	woUpdate *sqlc.UpdateWorkOrderParams,
) error {
	return db.InTxConn(ctx, r.pool, func(tx pgx.Tx, q *sqlc.Queries) error {
		if err := LockPanelCmWrites(ctx, tx, panelID); err != nil {
			return err
		}

		currentRows, err := q.ListProblemTopicsByWorkOrder(ctx, workOrderID)
		if err != nil {
			return db.Translate(err)
		}
		currentSet := make(map[uuid.UUID]struct{}, len(currentRows))
		for _, row := range currentRows {
			currentSet[row.ID] = struct{}{}
		}
		var added []uuid.UUID
		for _, id := range newTopicIDs {
			if _, ok := currentSet[id]; !ok {
				added = append(added, id)
			}
		}
		if len(added) > 0 {
			exclude := workOrderID
			if err := EnsureNoOpenCmConflictForTopics(ctx, tx, panelID, OpenCmDuplicateCheck{
				TopicIDs:           added,
				ExcludeWorkOrderID: &exclude,
			}); err != nil {
				return err
			}
		}

		if r.topics == nil {
			return nil
		}
		if err := r.topics.ReplaceAll(ctx, tx, workOrderID, newTopicIDs); err != nil {
			return err
		}
		if woUpdate != nil && workOrderUpdateParamsHasChanges(*woUpdate) {
			if _, err := r.UpdateQ(ctx, q, *woUpdate); err != nil {
				return err
			}
		}
		return nil
	})
}

func workOrderUpdateParamsHasChanges(p sqlc.UpdateWorkOrderParams) bool {
	return p.PmScheduleTypeDoUpdate || p.PanelDeviceIDDoUpdate || p.TitleDoUpdate || p.DescriptionDoUpdate ||
		p.PriorityDoUpdate || p.PlannedDateDoUpdate || p.DueDateDoUpdate
}

// UpdateStatus transitions a work order's status, optionally stamping closed_at.
func (r *WorkOrderRepository) UpdateStatus(ctx context.Context, arg sqlc.UpdateWorkOrderStatusParams) (sqlc.WorkOrder, error) {
	arg.UpdatedBy = updateAudit(ctx)
	wo, err := r.q.UpdateWorkOrderStatus(ctx, arg)
	if err != nil {
		return sqlc.WorkOrder{}, db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderNotFnd))
	}
	return wo, nil
}

// SetActive soft-cancels or reactivates a work order.
func (r *WorkOrderRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.WorkOrder, error) {
	wo, err := r.q.SetWorkOrderActive(ctx, sqlc.SetWorkOrderActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.WorkOrder{}, db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderNotFnd))
	}
	return wo, nil
}

// OpenCmWorkOrderFilter narrows open CM work orders on a panel — used when
// listing open CMs on a PM visit or checking panel+topic duplicates (device
// filter applies to list queries only; duplicate checks ignore device scope).
type OpenCmWorkOrderFilter struct {
	PanelDeviceID      *uuid.UUID
	ProblemTopicID     *uuid.UUID
	ExcludeWorkOrderID *uuid.UUID
}

// OpenCmWorkOrderSummary is one in-progress CM work order on a panel, enriched
// with the device and problem topics shown on the UI warning card.
type OpenCmWorkOrderSummary struct {
	WorkOrderID        uuid.UUID           `db:"work_order_id" json:"work_order_id"`
	WorkOrderNo        string              `db:"work_order_no" json:"work_order_no"`
	Status             string              `db:"status" json:"status"`
	PanelDeviceID      *uuid.UUID          `db:"panel_device_id" json:"panel_device_id"`
	PanelDeviceName    *string             `db:"panel_device_name" json:"panel_device_name"`
	PanelDeviceTagName *string             `db:"panel_device_tag_name" json:"panel_device_tag_name"`
	ProblemTopicID     *uuid.UUID          `db:"problem_topic_id" json:"problem_topic_id,omitempty"`
	ProblemTopicCode   *string             `db:"problem_topic_code" json:"problem_topic_code,omitempty"`
	ProblemTopicName   *string             `db:"problem_topic_name" json:"problem_topic_name,omitempty"`
	ProblemTopics      []ProblemTopicBrief `json:"problem_topics"`
	TagCode            *string             `db:"tag_code" json:"tag_code"`
}

const openCmEffectiveDevice = "COALESCE(cr.panel_device_id, wo.panel_device_id)"

const openCmWorkOrderSelect = `
SELECT
    wo.id AS work_order_id,
    wo.work_order_no,
    wo.status,
    ` + openCmEffectiveDevice + ` AS panel_device_id,
    pd.name AS panel_device_name,
    pd.tag_name AS panel_device_tag_name,
    cr.tag_code
FROM rtu.work_orders wo
LEFT JOIN rtu.cm_reports cr ON cr.work_order_round_id = wo.current_round_id
LEFT JOIN rtu.panel_devices pd ON pd.id = ` + openCmEffectiveDevice + `
WHERE %s
ORDER BY wo.created_at DESC, wo.id DESC`

func buildOpenCmWorkOrderConditions(a *args, panelID uuid.UUID, filter OpenCmWorkOrderFilter) conditions {
	conds := conditions{
		"wo.panel_id = " + a.add(panelID),
		"wo.work_order_type = 'CM'",
		"wo.active = true",
		"wo.status IN ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL')",
	}
	if filter.PanelDeviceID != nil {
		conds = append(conds, openCmEffectiveDevice+" = "+a.add(*filter.PanelDeviceID))
	}
	if filter.ProblemTopicID != nil {
		topic := a.add(*filter.ProblemTopicID)
		conds = append(conds, fmt.Sprintf(`EXISTS (
    SELECT 1 FROM rtu.work_order_problem_topics wopt
    WHERE wopt.work_order_id = wo.id AND wopt.problem_topic_id = %s
)`, topic))
	}
	if filter.ExcludeWorkOrderID != nil {
		conds = append(conds, "wo.id <> "+a.add(*filter.ExcludeWorkOrderID))
	}
	return conds
}

// ListOpenCmForPanel returns active CM work orders on a panel whose status
// is ASSIGNED, IN_PROGRESS, PENDING, or PENDING_APPROVAL.
func (r *WorkOrderRepository) ListOpenCmForPanel(ctx context.Context, panelID uuid.UUID, filter OpenCmWorkOrderFilter) ([]OpenCmWorkOrderSummary, error) {
	items, err := queryOpenCmForPanel(ctx, r.pool, panelID, filter)
	if err != nil {
		return nil, err
	}
	if r.topics == nil || len(items) == 0 {
		for i := range items {
			if items[i].ProblemTopics == nil {
				items[i].ProblemTopics = []ProblemTopicBrief{}
			}
		}
		return items, nil
	}
	ids := make([]uuid.UUID, len(items))
	for i := range items {
		ids[i] = items[i].WorkOrderID
	}
	topics, err := r.topics.MapByWorkOrders(ctx, ids)
	if err != nil {
		return nil, err
	}
	applyProblemTopicsToOpenCm(items, topics)
	return items, nil
}
