package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// WorkOrderService applies the business rules of rtu.work_orders and the
// first round of every work order it creates. Reassignment, check-in/out and
// submission of the *current* round also live here, since every one of
// those actions reads and mutates the work order's status alongside the
// round; see ApprovalService for what happens after a round is reviewed.
type WorkOrderService struct {
	repo     *repository.WorkOrderRepository
	rounds   *repository.WorkOrderRoundRepository
	activity *repository.WorkOrderActivityLogRepository
	panels   *repository.PanelRepository
	devices  *repository.PanelDeviceRepository
	notify   *NotificationService
}

// WorkOrderCreateInput is the POST /work-orders body. PanelID has no
// `validate:"required"` tag because /panels/{id}/work-orders binds this same
// struct and fills PanelID from the URL after validation runs.
type WorkOrderCreateInput struct {
	PanelID            uuid.UUID   `json:"panel_id"`
	WorkOrderType      string      `json:"work_order_type" validate:"required,oneof=PM CM"`
	PmScheduleType     *string     `json:"pm_schedule_type" validate:"omitempty,oneof=THREE_MONTH SIX_MONTH"`
	PanelDeviceID      *uuid.UUID  `json:"panel_device_id"`
	Title              *string     `json:"title" validate:"omitempty,max=255"`
	Description        *string     `json:"description" validate:"omitempty,max=4000"`
	Priority           *string     `json:"priority" validate:"omitempty,oneof=HIGH MEDIUM LOW"`
	Source             *string     `json:"source" validate:"omitempty,oneof=WORKFLOW LEGACY_IMPORT"`
	RequestedBy        uuid.UUID   `json:"requested_by" validate:"required"`
	AssignedTo         uuid.UUID   `json:"assigned_to" validate:"required"`
	AssignedBy         uuid.UUID   `json:"assigned_by" validate:"required"`
	RelatedWorkOrderID *uuid.UUID  `json:"related_work_order_id"`
	PlannedDate        *httpx.Date `json:"planned_date"`
	DueDate            *httpx.Date `json:"due_date"`
}

// WorkOrderUpdateInput is the PATCH /work-orders/{id} body. Status is not
// editable here — it only ever changes through the check-in/check-out/
// submit/approve actions, which keep the activity log consistent.
type WorkOrderUpdateInput struct {
	PanelDeviceID *uuid.UUID  `json:"panel_device_id"`
	Title         *string     `json:"title" validate:"omitempty,max=255"`
	Description   *string     `json:"description" validate:"omitempty,max=4000"`
	Priority      *string     `json:"priority" validate:"omitempty,oneof=HIGH MEDIUM LOW"`
	PlannedDate   *httpx.Date `json:"planned_date"`
	DueDate       *httpx.Date `json:"due_date"`
}

// ReassignInput is the POST /work-orders/{id}/reassign body. Only valid
// before the current round has checked in.
type ReassignInput struct {
	AssignedTo uuid.UUID `json:"assigned_to" validate:"required"`
	ActorID    uuid.UUID `json:"actor_id" validate:"required"`
}

// CheckInInput is the POST /work-orders/{id}/check-in body.
type CheckInInput struct {
	CheckInAt *time.Time       `json:"check_in_at"`
	Lat       *decimal.Decimal `json:"lat"`
	Lng       *decimal.Decimal `json:"lng"`
}

// CheckOutInput is the POST /work-orders/{id}/check-out body.
type CheckOutInput struct {
	CheckOutAt *time.Time       `json:"check_out_at"`
	Lat        *decimal.Decimal `json:"lat"`
	Lng        *decimal.Decimal `json:"lng"`
}

// List returns one page of work orders.
func (s *WorkOrderService) List(ctx context.Context, page httpx.Page, filter repository.WorkOrderFilter) ([]repository.WorkOrderView, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single work order with its joined context.
func (s *WorkOrderService) Get(ctx context.Context, id uuid.UUID) (repository.WorkOrderView, error) {
	return s.repo.GetView(ctx, id)
}

// Create opens a work order and its first round (round_no = 1) atomically.
func (s *WorkOrderService) Create(ctx context.Context, in WorkOrderCreateInput) (repository.WorkOrderView, error) {
	if err := checkPmScheduleType(in.WorkOrderType, in.PmScheduleType); err != nil {
		return repository.WorkOrderView{}, err
	}

	panel, err := s.panels.Get(ctx, in.PanelID)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if !panel.Active {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrPanelInactive).
			WithField("panel_id", httpx.IssueInvalid, "The panel is deactivated.")
	}

	if in.PanelDeviceID != nil {
		if err := s.checkDeviceInPanel(ctx, *in.PanelDeviceID, in.PanelID); err != nil {
			return repository.WorkOrderView{}, err
		}
	}

	total, err := s.repo.CountByPanelAndType(ctx, in.PanelID, in.WorkOrderType)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	workOrderNo := fmt.Sprintf("%s-%s-%d", in.WorkOrderType, panel.Code, total+1)

	assignedAt := time.Now()
	wo, _, err := s.repo.CreateWithFirstRound(ctx, sqlc.CreateWorkOrderParams{
		WorkOrderNo:        workOrderNo,
		WorkOrderType:      in.WorkOrderType,
		PmScheduleType:     in.PmScheduleType,
		PanelID:            in.PanelID,
		PanelDeviceID:      in.PanelDeviceID,
		Title:              in.Title,
		Description:        in.Description,
		Priority:           in.Priority,
		Source:             in.Source,
		RequestedBy:        in.RequestedBy,
		RelatedWorkOrderID: in.RelatedWorkOrderID,
		PlannedDate:        in.PlannedDate,
		DueDate:            in.DueDate,
	}, in.AssignedTo, in.AssignedBy, assignedAt, in.AssignedBy)
	if err != nil {
		return repository.WorkOrderView{}, err
	}

	view, err := s.repo.GetView(ctx, wo.ID)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if s.notify != nil {
		_, _ = s.notify.Create(ctx, NotificationCreateInput{
			WorkOrderID: wo.ID,
			RecipientID: in.AssignedTo,
			Type:        "NEW_ASSIGNMENT",
			Title:       stringPtr("New work order assigned"),
			Message:     stringPtr("You were assigned " + wo.WorkOrderNo + "."),
		})
	}
	return view, nil
}

// Update applies a partial update to a work order's editable fields.
func (s *WorkOrderService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in WorkOrderUpdateInput) (repository.WorkOrderView, error) {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}

	params := sqlc.UpdateWorkOrderParams{ID: id}
	params.PanelDeviceID, params.PanelDeviceIDDoUpdate = patchNullable(fields, "panel_device_id", in.PanelDeviceID)
	if params.PanelDeviceIDDoUpdate && params.PanelDeviceID != nil {
		if err := s.checkDeviceInPanel(ctx, *params.PanelDeviceID, current.PanelID); err != nil {
			return repository.WorkOrderView{}, err
		}
	}

	params.Title, params.TitleDoUpdate = patchNullable(fields, "title", in.Title)
	params.Description, params.DescriptionDoUpdate = patchNullable(fields, "description", in.Description)
	params.PlannedDate, params.PlannedDateDoUpdate = patchNullable(fields, "planned_date", in.PlannedDate)
	params.DueDate, params.DueDateDoUpdate = patchNullable(fields, "due_date", in.DueDate)

	priority, setPriority, err := patchRequired(fields, "priority", in.Priority)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	params.Priority, params.PriorityDoUpdate = priority, setPriority

	if _, err := s.repo.Update(ctx, params); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// Reassign changes the assignee of the round currently in progress, before
// it has checked in. After check-in a rejection must open a new round
// instead (see ApprovalService).
func (s *WorkOrderService) Reassign(ctx context.Context, id uuid.UUID, in ReassignInput) (repository.WorkOrderView, error) {
	wo, err := s.repo.GetView(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if wo.CurrentRoundID == nil {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to reassign.")
	}

	var fromAssignee uuid.UUID
	if wo.CurrentAssignedTo != nil {
		fromAssignee = *wo.CurrentAssignedTo
	}

	if _, err := s.rounds.Reassign(ctx, id, *wo.CurrentRoundID, fromAssignee, in.AssignedTo, in.ActorID, time.Now(), in.ActorID); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// CheckIn records the mobile check-in of the current round and moves the
// work order to IN_PROGRESS.
func (s *WorkOrderService) CheckIn(ctx context.Context, id uuid.UUID, in CheckInInput) (repository.WorkOrderView, error) {
	wo, err := s.repo.GetView(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if wo.CurrentRoundID == nil || wo.CurrentAssignedTo == nil {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to check into.")
	}

	checkInAt := time.Now()
	if in.CheckInAt != nil {
		checkInAt = *in.CheckInAt
	}

	if _, _, err := s.rounds.CheckIn(ctx, id, *wo.CurrentRoundID, checkInAt, in.Lat, in.Lng, wo.Status, *wo.CurrentAssignedTo); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// CheckOut records the mobile check-out of the current round. The work order
// stays IN_PROGRESS until its PM/CM report is submitted.
func (s *WorkOrderService) CheckOut(ctx context.Context, id uuid.UUID, in CheckOutInput) (repository.WorkOrderView, error) {
	wo, err := s.repo.GetView(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if wo.CurrentRoundID == nil || wo.CurrentAssignedTo == nil {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to check out of.")
	}

	checkOutAt := time.Now()
	if in.CheckOutAt != nil {
		checkOutAt = *in.CheckOutAt
	}

	if _, err := s.rounds.CheckOut(ctx, id, *wo.CurrentRoundID, checkOutAt, in.Lat, in.Lng, *wo.CurrentAssignedTo); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// MarkSubmitted stamps the current round's submitted_at and moves the work
// order to PENDING_APPROVAL. Called by PmReportService/CmReportService once
// a report is submitted.
func (s *WorkOrderService) MarkSubmitted(ctx context.Context, id uuid.UUID, submittedAt time.Time, actorID uuid.UUID) (repository.WorkOrderView, error) {
	wo, err := s.repo.GetView(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if wo.CurrentRoundID == nil {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to submit.")
	}

	if _, _, err := s.rounds.MarkSubmitted(ctx, id, *wo.CurrentRoundID, submittedAt, wo.Status, actorID); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// OpenNewRound reopens a work order for rework after a round was rejected,
// per rtu.wo_approvals: the same work order gets round_no+1, newStatus is
// typically PENDING. Exposed for ApprovalService.
func (s *WorkOrderService) OpenNewRound(
	ctx context.Context, id uuid.UUID, assignedTo, assignedBy uuid.UUID, newStatus string, actorID uuid.UUID,
) (repository.WorkOrderView, error) {
	wo, err := s.repo.Get(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}

	if _, _, err := s.repo.OpenNewRound(ctx, id, assignedTo, assignedBy, time.Now(), newStatus, actorID, wo.Status); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// UpdateStatus is a direct status transition with no round involved (used by
// ApprovalService for APPROVED -> COMPLETED / APPROVED_CONDITION -> CONDITIONAL,
// and by SoftDelete/Restore for CANCELLED).
func (s *WorkOrderService) UpdateStatus(ctx context.Context, id uuid.UUID, status string, closeIt bool) (repository.WorkOrderView, error) {
	params := sqlc.UpdateWorkOrderStatusParams{ID: id, Status: status}
	if closeIt {
		now := time.Now()
		params.ClosedAt, params.ClosedAtDoUpdate = &now, true
	}
	if _, err := s.repo.UpdateStatus(ctx, params); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// SoftDelete cancels a work order without losing its history.
func (s *WorkOrderService) SoftDelete(ctx context.Context, id uuid.UUID) (repository.WorkOrderView, error) {
	if _, err := s.repo.SetActive(ctx, id, false); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// Restore reactivates a cancelled work order.
func (s *WorkOrderService) Restore(ctx context.Context, id uuid.UUID) (repository.WorkOrderView, error) {
	if _, err := s.repo.SetActive(ctx, id, true); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// ListRounds returns the full round-by-round history of a work order — every
// visit/rework attempt, who did it and when.
func (s *WorkOrderService) ListRounds(ctx context.Context, id uuid.UUID) ([]sqlc.WorkOrderRound, error) {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.rounds.ListByWorkOrder(ctx, id)
}

// FindReusableCMWorkOrder returns a PENDING CM on the panel whose current
// round has no report yet — safe for PM escalation reuse.
func (s *WorkOrderService) FindReusableCMWorkOrder(ctx context.Context, panelID uuid.UUID) (*uuid.UUID, error) {
	return s.repo.FindReusableCMForPanel(ctx, panelID)
}

// FindOpenWorkOrder returns the id of an open (not completed/cancelled) work
// order of the given type for a panel, if any. Used by ApprovalService to
// decide whether escalating a rejected PM to CM should reuse an existing CM
// work order or create a new one.
func (s *WorkOrderService) FindOpenWorkOrder(ctx context.Context, panelID uuid.UUID, workOrderType string) (*uuid.UUID, error) {
	return s.repo.FindOpenForPanel(ctx, panelID, workOrderType)
}

// ListActivity returns the full status/assignment timeline of a work order —
// when it was opened, assigned, started, sent for approval and rejected.
func (s *WorkOrderService) ListActivity(ctx context.Context, id uuid.UUID) ([]sqlc.WorkOrderActivityLog, error) {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.activity.ListByWorkOrder(ctx, id)
}

func (s *WorkOrderService) checkDeviceInPanel(ctx context.Context, deviceID, panelID uuid.UUID) error {
	device, err := s.devices.Get(ctx, deviceID)
	if err != nil {
		return err
	}
	if device.PanelID != panelID {
		return httpx.Err(httpx.ErrDeviceNotInPanel).
			WithField("panel_device_id", httpx.IssueInvalid, "Must belong to the given panel.")
	}
	return nil
}

func checkPmScheduleType(workOrderType string, pmScheduleType *string) error {
	if workOrderType == "PM" && pmScheduleType == nil {
		return httpx.Err(httpx.ErrPmScheduleTypeRequired).
			WithField("pm_schedule_type", httpx.IssueRequired, "Required when work_order_type is PM.")
	}
	if workOrderType == "CM" && pmScheduleType != nil {
		return httpx.Err(httpx.ErrPmScheduleTypeNotAllowed).
			WithField("pm_schedule_type", httpx.IssueInvalid, "Must not be set when work_order_type is CM.")
	}
	return nil
}
