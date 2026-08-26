package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// ProblemTopicRepository reads and writes rtu.problem_topics — the master
// list of CM issue categories shown as pills in the UI.
type ProblemTopicRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var problemTopicConstraints = db.Constraints{
	"uk_problem_topics_code": httpx.ErrProblemTopicCodeDup,
}

var problemTopicDeleteConstraints = db.Constraints{
	"fk_cm_reports_problem_topic": httpx.ErrProblemTopicInUse,
}

// List returns every problem topic, ordered by sort_order.
func (r *ProblemTopicRepository) List(ctx context.Context, active *bool) ([]sqlc.RtuProblemTopic, error) {
	items, err := r.q.ListProblemTopics(ctx, active)
	if err != nil {
		return nil, db.Translate(err)
	}
	return items, nil
}

// Get returns a single problem topic by id.
func (r *ProblemTopicRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.RtuProblemTopic, error) {
	item, err := r.q.GetProblemTopic(ctx, id)
	if err != nil {
		return sqlc.RtuProblemTopic{}, db.Translate(err, db.WithNotFound(httpx.ErrProblemTopicNotFnd))
	}
	return item, nil
}

// Create inserts a problem topic.
func (r *ProblemTopicRepository) Create(ctx context.Context, arg sqlc.CreateProblemTopicParams) (sqlc.RtuProblemTopic, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	item, err := r.q.CreateProblemTopic(ctx, arg)
	if err != nil {
		return sqlc.RtuProblemTopic{}, db.Translate(err, db.Options{Constraints: problemTopicConstraints})
	}
	return item, nil
}

// Update applies a partial update.
func (r *ProblemTopicRepository) Update(ctx context.Context, arg sqlc.UpdateProblemTopicParams) (sqlc.RtuProblemTopic, error) {
	arg.UpdatedBy = updateAudit(ctx)
	item, err := r.q.UpdateProblemTopic(ctx, arg)
	if err != nil {
		return sqlc.RtuProblemTopic{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrProblemTopicNotFnd,
			Constraints: problemTopicConstraints,
		})
	}
	return item, nil
}

// SetActive soft-deletes or restores a problem topic.
func (r *ProblemTopicRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.RtuProblemTopic, error) {
	item, err := r.q.SetProblemTopicActive(ctx, sqlc.SetProblemTopicActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.RtuProblemTopic{}, db.Translate(err, db.WithNotFound(httpx.ErrProblemTopicNotFnd))
	}
	return item, nil
}

// Delete removes a problem topic permanently.
func (r *ProblemTopicRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteProblemTopic(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: problemTopicDeleteConstraints})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrProblemTopicNotFnd)
	}
	return nil
}

// Usability returns whether a topic may be selected on a new CM report.
func (r *ProblemTopicRepository) Usability(ctx context.Context, id uuid.UUID) (active bool, err error) {
	row, err := r.q.GetProblemTopicUsability(ctx, id)
	if err != nil {
		return false, db.Translate(err, db.WithNotFound(httpx.ErrProblemTopicNotFnd))
	}
	return row, nil
}
