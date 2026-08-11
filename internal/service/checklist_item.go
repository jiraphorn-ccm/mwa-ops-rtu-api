package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// ChecklistItemService applies the business rules of rtu.checklist_items —
// the master list of the standard PM checklist (System Design Screen 03).
type ChecklistItemService struct {
	repo *repository.ChecklistItemRepository
}

// ChecklistItemCreateInput is the POST /checklist-items body.
type ChecklistItemCreateInput struct {
	Code         string  `json:"code" validate:"required,max=20"`
	Name         string  `json:"name" validate:"required,max=255"`
	ActionType   string  `json:"action_type" validate:"required,oneof=MAINTENANCE MEASUREMENT VISUAL_INSPECTION"`
	ApplicablePm *string `json:"applicable_pm" validate:"omitempty,oneof=PM3 PM6 BOTH"`
	SortOrder    int16   `json:"sort_order" validate:"gte=0"`
	Active       *bool   `json:"active"`
}

// ChecklistItemUpdateInput is the PATCH /checklist-items/{id} body.
type ChecklistItemUpdateInput struct {
	Code         *string `json:"code" validate:"omitempty,max=20"`
	Name         *string `json:"name" validate:"omitempty,max=255"`
	ActionType   *string `json:"action_type" validate:"omitempty,oneof=MAINTENANCE MEASUREMENT VISUAL_INSPECTION"`
	ApplicablePm *string `json:"applicable_pm" validate:"omitempty,oneof=PM3 PM6 BOTH"`
	SortOrder    *int16  `json:"sort_order" validate:"omitempty,gte=0"`
	Active       *bool   `json:"active"`
}

// List returns every checklist item, ordered by sort_order.
func (s *ChecklistItemService) List(ctx context.Context, active *bool) ([]sqlc.ChecklistItem, error) {
	return s.repo.List(ctx, active)
}

// Get returns a single checklist item.
func (s *ChecklistItemService) Get(ctx context.Context, id uuid.UUID) (sqlc.ChecklistItem, error) {
	return s.repo.Get(ctx, id)
}

// Create registers a new checklist item.
func (s *ChecklistItemService) Create(ctx context.Context, in ChecklistItemCreateInput) (sqlc.ChecklistItem, error) {
	return s.repo.Create(ctx, sqlc.CreateChecklistItemParams{
		Code:         in.Code,
		Name:         in.Name,
		ActionType:   in.ActionType,
		ApplicablePm: in.ApplicablePm,
		SortOrder:    in.SortOrder,
		Active:       in.Active,
	})
}

// Update applies a partial update to a checklist item.
func (s *ChecklistItemService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in ChecklistItemUpdateInput) (sqlc.ChecklistItem, error) {
	params := sqlc.UpdateChecklistItemParams{ID: id}

	code, setCode, err := patchRequired(fields, "code", in.Code)
	if err != nil {
		return sqlc.ChecklistItem{}, err
	}
	params.Code, params.CodeDoUpdate = code, setCode

	name, setName, err := patchRequired(fields, "name", in.Name)
	if err != nil {
		return sqlc.ChecklistItem{}, err
	}
	params.Name, params.NameDoUpdate = name, setName

	actionType, setActionType, err := patchRequired(fields, "action_type", in.ActionType)
	if err != nil {
		return sqlc.ChecklistItem{}, err
	}
	params.ActionType, params.ActionTypeDoUpdate = actionType, setActionType

	applicablePm, setApplicablePm, err := patchRequired(fields, "applicable_pm", in.ApplicablePm)
	if err != nil {
		return sqlc.ChecklistItem{}, err
	}
	params.ApplicablePm, params.ApplicablePmDoUpdate = applicablePm, setApplicablePm

	sortOrder, setSortOrder, err := patchRequired(fields, "sort_order", in.SortOrder)
	if err != nil {
		return sqlc.ChecklistItem{}, err
	}
	params.SortOrder, params.SortOrderDoUpdate = sortOrder, setSortOrder

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return sqlc.ChecklistItem{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates a checklist item.
func (s *ChecklistItemService) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.ChecklistItem, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a deactivated checklist item.
func (s *ChecklistItemService) Restore(ctx context.Context, id uuid.UUID) (sqlc.ChecklistItem, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes a checklist item permanently. It fails while checklist
// results reference it.
func (s *ChecklistItemService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
