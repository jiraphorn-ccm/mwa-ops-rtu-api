package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

var workOrderProblemTopicConstraints = db.Constraints{
	"fk_wopt_problem_topic": httpx.ErrProblemTopicNotFnd,
	"fk_wopt_work_order":    httpx.ErrWorkOrderNotFnd,
}

// ProblemTopicBrief is one CM problem topic linked to a work order.
type ProblemTopicBrief struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	SortOrder   int16     `json:"-"`
	WorkOrderID uuid.UUID `json:"-"`
}

// WorkOrderProblemTopicRepository reads and writes rtu.work_order_problem_topics.
type WorkOrderProblemTopicRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func NewWorkOrderProblemTopicRepository(pool *pgxpool.Pool, q *sqlc.Queries) *WorkOrderProblemTopicRepository {
	return &WorkOrderProblemTopicRepository{pool: pool, q: q}
}

// InsertAll links problem topics to a CM work order in sort order.
func (r *WorkOrderProblemTopicRepository) InsertAll(ctx context.Context, qtx pgx.Tx, workOrderID uuid.UUID, topicIDs []uuid.UUID) error {
	if len(topicIDs) == 0 {
		return nil
	}
	q := r.q.WithTx(qtx)
	for i, topicID := range topicIDs {
		if err := q.InsertWorkOrderProblemTopic(ctx, sqlc.InsertWorkOrderProblemTopicParams{
			WorkOrderID:    workOrderID,
			ProblemTopicID: topicID,
			SortOrder:      int16(i),
		}); err != nil {
			return db.Translate(err, db.Options{Constraints: workOrderProblemTopicConstraints})
		}
	}
	return nil
}

// ReplaceAll sets the full topic list for a CM work order inside an existing transaction.
func (r *WorkOrderProblemTopicRepository) ReplaceAll(ctx context.Context, qtx pgx.Tx, workOrderID uuid.UUID, topicIDs []uuid.UUID) error {
	q := r.q.WithTx(qtx)
	if err := q.DeleteWorkOrderProblemTopicsByWorkOrder(ctx, workOrderID); err != nil {
		return db.Translate(err)
	}
	return r.InsertAll(ctx, qtx, workOrderID, topicIDs)
}

// SyncFromReport adds the CM report topic to the work order junction when it is
// not already linked. Topics set at create or by earlier syncs are never removed.
func (r *WorkOrderProblemTopicRepository) SyncFromReport(
	ctx context.Context,
	qtx pgx.Tx,
	workOrderID uuid.UUID,
	newTopicID *uuid.UUID,
) error {
	if newTopicID == nil || *newTopicID == uuid.Nil {
		return nil
	}
	q := r.q.WithTx(qtx)

	found, err := q.WorkOrderHasProblemTopic(ctx, sqlc.WorkOrderHasProblemTopicParams{
		WorkOrderID:    workOrderID,
		ProblemTopicID: *newTopicID,
	})
	if err != nil {
		return db.Translate(err)
	}
	if found {
		return nil
	}

	next, err := q.NextWorkOrderProblemTopicSortOrder(ctx, workOrderID)
	if err != nil {
		return db.Translate(err)
	}
	if err := q.InsertWorkOrderProblemTopic(ctx, sqlc.InsertWorkOrderProblemTopicParams{
		WorkOrderID:    workOrderID,
		ProblemTopicID: *newTopicID,
		SortOrder:      next + 1,
	}); err != nil {
		return db.Translate(err, db.Options{Constraints: workOrderProblemTopicConstraints})
	}
	return nil
}

// ListByWorkOrder returns topics linked to one work order.
func (r *WorkOrderProblemTopicRepository) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]ProblemTopicBrief, error) {
	rows, err := r.q.ListProblemTopicsByWorkOrder(ctx, workOrderID)
	if err != nil {
		return nil, db.Translate(err)
	}
	out := make([]ProblemTopicBrief, len(rows))
	for i, row := range rows {
		out[i] = ProblemTopicBrief{
			ID:        row.ID,
			Code:      row.Code,
			Name:      row.Name,
			SortOrder: row.SortOrder,
		}
	}
	return out, nil
}

// MapByWorkOrders returns topics grouped by work order id.
func (r *WorkOrderProblemTopicRepository) MapByWorkOrders(ctx context.Context, workOrderIDs []uuid.UUID) (map[uuid.UUID][]ProblemTopicBrief, error) {
	if len(workOrderIDs) == 0 {
		return map[uuid.UUID][]ProblemTopicBrief{}, nil
	}
	rows, err := r.q.ListProblemTopicsByWorkOrders(ctx, workOrderIDs)
	if err != nil {
		return nil, db.Translate(err)
	}
	out := make(map[uuid.UUID][]ProblemTopicBrief, len(workOrderIDs))
	for _, row := range rows {
		out[row.WorkOrderID] = append(out[row.WorkOrderID], ProblemTopicBrief{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			SortOrder:   row.SortOrder,
			WorkOrderID: row.WorkOrderID,
		})
	}
	return out, nil
}

func applyProblemTopicsToViews(views []WorkOrderView, topics map[uuid.UUID][]ProblemTopicBrief) {
	for i := range views {
		list := topics[views[i].ID]
		if list == nil {
			list = []ProblemTopicBrief{}
		}
		views[i].ProblemTopics = list
		if len(list) > 0 {
			first := list[0]
			id := first.ID
			code := first.Code
			name := first.Name
			views[i].ProblemTopicID = &id
			views[i].ProblemTopicCode = &code
			views[i].ProblemTopicName = &name
		}
	}
}

func applyProblemTopicsToOpenCm(items []OpenCmWorkOrderSummary, topics map[uuid.UUID][]ProblemTopicBrief) {
	for i := range items {
		list := topics[items[i].WorkOrderID]
		if list == nil {
			list = []ProblemTopicBrief{}
		}
		items[i].ProblemTopics = list
		if len(list) > 0 {
			first := list[0]
			id := first.ID
			code := first.Code
			name := first.Name
			items[i].ProblemTopicID = &id
			items[i].ProblemTopicCode = &code
			items[i].ProblemTopicName = &name
		}
	}
}
