package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// ProblemTopicService applies the business rules of rtu.problem_topics.
type ProblemTopicService struct {
	repo *repository.ProblemTopicRepository
}

// ProblemTopicCreateInput is the POST /problem-topics body.
type ProblemTopicCreateInput struct {
	Code      string `json:"code" validate:"required,max=30"`
	Name      string `json:"name" validate:"required,max=255"`
	SortOrder int16  `json:"sort_order" validate:"gte=0"`
	Active    *bool  `json:"active"`
}

// ProblemTopicUpdateInput is the PATCH /problem-topics/{id} body.
type ProblemTopicUpdateInput struct {
	Code      *string `json:"code" validate:"omitempty,max=30"`
	Name      *string `json:"name" validate:"omitempty,max=255"`
	SortOrder *int16  `json:"sort_order" validate:"omitempty,gte=0"`
	Active    *bool   `json:"active"`
}

// List returns every problem topic, ordered by sort_order.
func (s *ProblemTopicService) List(ctx context.Context, active *bool) ([]sqlc.RtuProblemTopic, error) {
	return s.repo.List(ctx, active)
}

// Get returns a single problem topic.
func (s *ProblemTopicService) Get(ctx context.Context, id uuid.UUID) (sqlc.RtuProblemTopic, error) {
	return s.repo.Get(ctx, id)
}

// Create registers a new problem topic.
func (s *ProblemTopicService) Create(ctx context.Context, in ProblemTopicCreateInput) (sqlc.RtuProblemTopic, error) {
	return s.repo.Create(ctx, sqlc.CreateProblemTopicParams{
		Code:      in.Code,
		Name:      in.Name,
		SortOrder: in.SortOrder,
		Active:    in.Active,
	})
}

// Update applies a partial update to a problem topic.
func (s *ProblemTopicService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in ProblemTopicUpdateInput) (sqlc.RtuProblemTopic, error) {
	params := sqlc.UpdateProblemTopicParams{ID: id}

	code, setCode, err := patchRequired(fields, "code", in.Code)
	if err != nil {
		return sqlc.RtuProblemTopic{}, err
	}
	params.Code, params.CodeDoUpdate = code, setCode

	name, setName, err := patchRequired(fields, "name", in.Name)
	if err != nil {
		return sqlc.RtuProblemTopic{}, err
	}
	params.Name, params.NameDoUpdate = name, setName

	sortOrder, setSortOrder, err := patchRequired(fields, "sort_order", in.SortOrder)
	if err != nil {
		return sqlc.RtuProblemTopic{}, err
	}
	params.SortOrder, params.SortOrderDoUpdate = sortOrder, setSortOrder

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return sqlc.RtuProblemTopic{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates a problem topic.
func (s *ProblemTopicService) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.RtuProblemTopic, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a deactivated problem topic.
func (s *ProblemTopicService) Restore(ctx context.Context, id uuid.UUID) (sqlc.RtuProblemTopic, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes a problem topic permanently.
func (s *ProblemTopicService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// RequireUsable rejects inactive or missing topics before they are linked
// from a CM report.
func (s *ProblemTopicService) RequireUsable(ctx context.Context, id uuid.UUID) (sqlc.RtuProblemTopic, error) {
	topic, err := s.repo.Get(ctx, id)
	if err != nil {
		return sqlc.RtuProblemTopic{}, err
	}
	if !topic.Active {
		return sqlc.RtuProblemTopic{}, httpx.Err(httpx.ErrProblemTopicInactive)
	}
	return topic, nil
}
