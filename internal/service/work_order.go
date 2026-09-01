package service

import (
	"context"
	"errors"
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
	repo          *repository.WorkOrderRepository
	rounds        *repository.WorkOrderRoundRepository
	activity      *repository.WorkOrderActivityLogRepository
	panels        *repository.PanelRepository
	devices       *repository.PanelDeviceRepository
	problemTopics *repository.ProblemTopicRepository
	notify        *NotificationService
}

// WorkOrderCreateInput is the POST /work-orders body. PanelID has no
// `validate:"required"` tag because /panels/{id}/work-orders binds this same
// struct and fills PanelID from the URL after validation runs.
//
// work_order_no is never accepted from clients — the server allocates it on
// create using domain.FormatWorkOrderNo (TYPE-PANEL_CODE-SEQUENCE).
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
	// Required on CM create (client API). Stored on the initial cm_report row.
	ProblemTopicID *uuid.UUID `json:"problem_topic_id"`
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

var workOrderEditableFields = map[string]struct{}{
	"panel_device_id": {},
	"title":           {},
	"description":     {},
	"priority":        {},
	"planned_date":    {},
	"due_date":        {},
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
	if err := checkCmProblemTopic(in.WorkOrderType, in.ProblemTopicID); err != nil {
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

	var initialCmReport *sqlc.CreateCmReportParams
	var openCmDuplicate *repository.OpenCmWorkOrderFilter
	if in.WorkOrderType == "CM" {
		topicID, tagCode, err := s.resolveProblemTopic(ctx, *in.ProblemTopicID)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		openCmDuplicate = &repository.OpenCmWorkOrderFilter{
			ProblemTopicID: topicID,
		}
		initialCmReport = &sqlc.CreateCmReportParams{
			ReportedBy:     in.RequestedBy,
			ProblemTopicID: topicID,
			TagCode:        tagCode,
			PanelDeviceID:  in.PanelDeviceID,
		}
	}

	assignedAt := time.Now()
	wo, _, err := s.repo.CreateWithFirstRound(ctx, sqlc.CreateWorkOrderParams{
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
	}, panel.Code, in.AssignedTo, in.AssignedBy, assignedAt, in.AssignedBy, initialCmReport, openCmDuplicate)
	if err != nil {
		if conflict := openCmConflictError(err); conflict != nil {
			return repository.WorkOrderView{}, appErrFromOpenCmConflict(conflict.Conflict)
		}
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
	if len(fields) > 0 && !fields.HasAny(workOrderEditableFields) {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("body", httpx.IssueInvalid,
				"No editable fields in request. Accepted keys: panel_device_id, title, description, priority, planned_date, due_date.")
	}

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

	if !workOrderUpdateHasChanges(params) {
		return s.repo.GetView(ctx, id)
	}

	if _, err := s.repo.Update(ctx, params); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

func workOrderUpdateHasChanges(p sqlc.UpdateWorkOrderParams) bool {
	return p.PanelDeviceIDDoUpdate || p.TitleDoUpdate || p.DescriptionDoUpdate ||
		p.PriorityDoUpdate || p.PlannedDateDoUpdate || p.DueDateDoUpdate
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

// FindMatchingOpenCm returns an open CM work order on the panel that already
// covers the same problem topic (panel-wide, regardless of device scope).
func (s *WorkOrderService) FindMatchingOpenCm(ctx context.Context, panelID uuid.UUID, problemTopicID *uuid.UUID) (*uuid.UUID, error) {
	if problemTopicID == nil {
		return nil, nil
	}
	items, err := s.ListOpenCmForPanel(ctx, panelID, repository.OpenCmWorkOrderFilter{
		ProblemTopicID: problemTopicID,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	id := items[0].WorkOrderID
	return &id, nil
}

// EnsureNoOpenCmDuplicate rejects creating or saving a CM when another open
// work order on the same panel already covers the same problem topic.
// excludeWorkOrderID skips the caller's own work order during upsert/update.
func (s *WorkOrderService) EnsureNoOpenCmDuplicate(ctx context.Context, panelID uuid.UUID, problemTopicID *uuid.UUID, excludeWorkOrderID *uuid.UUID) error {
	if problemTopicID == nil {
		return nil
	}
	filter := repository.OpenCmWorkOrderFilter{
		ProblemTopicID:     problemTopicID,
		ExcludeWorkOrderID: excludeWorkOrderID,
	}
	items, err := s.ListOpenCmForPanel(ctx, panelID, filter)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return appErrFromOpenCmConflict(items[0])
}

func appErrFromOpenCmConflict(conflict repository.OpenCmWorkOrderSummary) error {
	msg := fmt.Sprintf("Open CM work order %s already covers this problem topic.", conflict.WorkOrderNo)
	if conflict.ProblemTopicName != nil && *conflict.ProblemTopicName != "" {
		msg = fmt.Sprintf("Open CM work order %s already covers problem topic %q.", conflict.WorkOrderNo, *conflict.ProblemTopicName)
	}
	return httpx.Err(httpx.ErrOpenCmDuplicate).
		WithField("problem_topic_id", httpx.IssueDuplicate, msg).
		WithField("work_order_id", httpx.IssueDuplicate, conflict.WorkOrderID.String())
}

func openCmConflictError(err error) *repository.OpenCmConflictError {
	var conflict *repository.OpenCmConflictError
	if errors.As(err, &conflict) {
		return conflict
	}
	return nil
}

// ListOpenCmForPanel returns CM work orders on a panel in ASSIGNED,
// IN_PROGRESS, PENDING, or PENDING_APPROVAL status.
func (s *WorkOrderService) ListOpenCmForPanel(ctx context.Context, panelID uuid.UUID, filter repository.OpenCmWorkOrderFilter) ([]repository.OpenCmWorkOrderSummary, error) {
	if _, err := s.panels.Get(ctx, panelID); err != nil {
		return nil, err
	}
	if filter.PanelDeviceID != nil {
		if err := s.checkDeviceInPanel(ctx, *filter.PanelDeviceID, panelID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListOpenCmForPanel(ctx, panelID, filter)
}

// ListOpenCmForWorkOrder resolves the panel of a work order and lists open CM
// work orders on that panel. When the caller is a CM work order, it is
// excluded from the result set.
func (s *WorkOrderService) ListOpenCmForWorkOrder(ctx context.Context, workOrderID uuid.UUID, filter repository.OpenCmWorkOrderFilter) ([]repository.OpenCmWorkOrderSummary, error) {
	wo, err := s.Get(ctx, workOrderID)
	if err != nil {
		return nil, err
	}
	if wo.WorkOrderType == "CM" {
		filter.ExcludeWorkOrderID = &workOrderID
	}
	return s.ListOpenCmForPanel(ctx, wo.PanelID, filter)
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

func checkCmProblemTopic(workOrderType string, problemTopicID *uuid.UUID) error {
	if workOrderType == "PM" && problemTopicID != nil {
		return httpx.Err(httpx.ErrCmProblemTopicNotAllowed).
			WithField("problem_topic_id", httpx.IssueInvalid, "Must not be set when work_order_type is PM.")
	}
	if workOrderType == "CM" {
		if problemTopicID == nil {
			return httpx.Err(httpx.ErrCmProblemTopicRequired).
				WithField("problem_topic_id", httpx.IssueRequired, "Required when work_order_type is CM.")
		}
		if *problemTopicID == uuid.Nil {
			return httpx.Err(httpx.ErrValidationFailed).
				WithField("problem_topic_id", httpx.IssueInvalid, "Must be a valid UUID.")
		}
	}
	return nil
}

func (s *WorkOrderService) resolveProblemTopic(ctx context.Context, topicID uuid.UUID) (*uuid.UUID, *string, error) {
	if s.problemTopics == nil {
		return &topicID, nil, nil
	}
	topic, err := s.problemTopics.Get(ctx, topicID)
	if err != nil {
		return nil, nil, err
	}
	if !topic.Active {
		return nil, nil, httpx.Err(httpx.ErrProblemTopicInactive)
	}
	code := topic.Code
	return &topicID, &code, nil
}
