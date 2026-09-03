package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	cmReports     *repository.CmReportRepository
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
	// Required on CM create (client API). Stored on work_order_problem_topics
	// and the initial cm_report row (first topic).
	ProblemTopicID  *uuid.UUID  `json:"problem_topic_id"`
	ProblemTopicIDs []uuid.UUID `json:"problem_topic_ids"`
}

// WorkOrderUpdateInput is the PATCH /work-orders/{id} body. Status is not
// editable here — it only ever changes through the check-in/check-out/
// submit/approve actions, which keep the activity log consistent.
type WorkOrderUpdateInput struct {
	PmScheduleType  *string     `json:"pm_schedule_type" validate:"omitempty,oneof=THREE_MONTH SIX_MONTH"`
	PanelDeviceID   *uuid.UUID  `json:"panel_device_id"`
	Title           *string     `json:"title" validate:"omitempty,max=255"`
	Description     *string     `json:"description" validate:"omitempty,max=4000"`
	Priority        *string     `json:"priority" validate:"omitempty,oneof=HIGH MEDIUM LOW"`
	PlannedDate     *httpx.Date `json:"planned_date"`
	DueDate         *httpx.Date `json:"due_date"`
	ProblemTopicID  *uuid.UUID  `json:"problem_topic_id"`
	ProblemTopicIDs []uuid.UUID `json:"problem_topic_ids"`
}

var workOrderEditableFields = map[string]struct{}{
	"pm_schedule_type":  {},
	"panel_device_id":   {},
	"title":             {},
	"description":       {},
	"priority":          {},
	"planned_date":      {},
	"due_date":          {},
	"problem_topic_id":  {},
	"problem_topic_ids": {},
}

var pmScheduleTypeEditableStatuses = map[string]struct{}{
	"ASSIGNED":    {},
	"IN_PROGRESS": {},
	"PENDING":     {},
}

var cmProblemTopicEditableStatuses = map[string]struct{}{
	"ASSIGNED":    {},
	"IN_PROGRESS": {},
	"PENDING":     {},
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
	if err := checkCmProblemTopics(in.WorkOrderType, in.ProblemTopicID, in.ProblemTopicIDs); err != nil {
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
	var openCmDuplicate *repository.OpenCmDuplicateCheck
	var problemTopicIDs []uuid.UUID
	if in.WorkOrderType == "CM" {
		topicIDs, err := normalizeProblemTopicIDs(in.ProblemTopicID, in.ProblemTopicIDs)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		problemTopicIDs = topicIDs
		resolved, firstTag, err := s.resolveProblemTopics(ctx, topicIDs)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		problemTopicIDs = resolved
		openCmDuplicate = &repository.OpenCmDuplicateCheck{
			TopicIDs: problemTopicIDs,
		}
		initialCmReport = &sqlc.CreateCmReportParams{
			ReportedBy:     in.RequestedBy,
			ProblemTopicID: &problemTopicIDs[0],
			TagCode:        firstTag,
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
	}, panel.Code, in.AssignedTo, in.AssignedBy, assignedAt, in.AssignedBy, initialCmReport, openCmDuplicate, problemTopicIDs)
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
	topicField := fields.Has("problem_topic_id") || fields.Has("problem_topic_ids")
	if len(fields) > 0 && !fields.HasAny(workOrderEditableFields) {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("body", httpx.IssueInvalid,
				"No editable fields in request. Accepted keys: pm_schedule_type, panel_device_id, title, description, priority, planned_date, due_date, problem_topic_id, problem_topic_ids.")
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return repository.WorkOrderView{}, err
	}

	params := sqlc.UpdateWorkOrderParams{ID: id}

	if fields.Has("pm_schedule_type") {
		if err := checkPmScheduleTypePatch(current, in.PmScheduleType); err != nil {
			return repository.WorkOrderView{}, err
		}
		schedule, setSchedule, err := patchRequired(fields, "pm_schedule_type", in.PmScheduleType)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		params.PmScheduleType = &schedule
		params.PmScheduleTypeDoUpdate = setSchedule
	}

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

	var newTopicIDs []uuid.UUID
	if topicField {
		if err := checkCmProblemTopics(current.WorkOrderType, in.ProblemTopicID, in.ProblemTopicIDs); err != nil {
			return repository.WorkOrderView{}, err
		}
		if err := checkCmProblemTopicsPatch(current); err != nil {
			return repository.WorkOrderView{}, err
		}
		topicIDs, err := normalizeProblemTopicIDs(in.ProblemTopicID, in.ProblemTopicIDs)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		resolved, _, err := s.resolveProblemTopics(ctx, topicIDs)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		if err := s.ensureCmReportTopicIncluded(ctx, current, resolved); err != nil {
			return repository.WorkOrderView{}, err
		}
		newTopicIDs = resolved
	}

	hasFieldChanges := workOrderUpdateHasChanges(params)
	if !hasFieldChanges && !topicField {
		return s.repo.GetView(ctx, id)
	}

	if topicField {
		var woUpdate *sqlc.UpdateWorkOrderParams
		if hasFieldChanges {
			woUpdate = &params
		}
		if err := s.repo.ReplaceCmProblemTopics(ctx, current.PanelID, id, newTopicIDs, woUpdate); err != nil {
			if conflict := openCmConflictError(err); conflict != nil {
				return repository.WorkOrderView{}, appErrFromOpenCmConflict(conflict.Conflict)
			}
			return repository.WorkOrderView{}, err
		}
		return s.repo.GetView(ctx, id)
	}

	if _, err := s.repo.Update(ctx, params); err != nil {
		return repository.WorkOrderView{}, err
	}
	return s.repo.GetView(ctx, id)
}

func workOrderUpdateHasChanges(p sqlc.UpdateWorkOrderParams) bool {
	return p.PmScheduleTypeDoUpdate || p.PanelDeviceIDDoUpdate || p.TitleDoUpdate || p.DescriptionDoUpdate ||
		p.PriorityDoUpdate || p.PlannedDateDoUpdate || p.DueDateDoUpdate
}

func checkCmProblemTopicsPatch(wo sqlc.WorkOrder) error {
	if wo.WorkOrderType != "CM" {
		return httpx.Err(httpx.ErrCmProblemTopicNotAllowed).
			WithField("problem_topic_id", httpx.IssueInvalid, "Must not be set when work_order_type is PM.")
	}
	if _, ok := cmProblemTopicEditableStatuses[wo.Status]; !ok {
		return httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("problem_topic_id", httpx.IssueInvalid,
				"Can only be changed while status is ASSIGNED, IN_PROGRESS, or PENDING.")
	}
	return nil
}

func (s *WorkOrderService) ensureCmReportTopicIncluded(ctx context.Context, wo sqlc.WorkOrder, topicIDs []uuid.UUID) error {
	if s.cmReports == nil || wo.CurrentRoundID == nil {
		return nil
	}
	report, err := s.cmReports.FindByRound(ctx, *wo.CurrentRoundID)
	if err != nil {
		return err
	}
	if report == nil || report.ProblemTopicID == nil {
		return nil
	}
	for _, id := range topicIDs {
		if id == *report.ProblemTopicID {
			return nil
		}
	}
	return httpx.Err(httpx.ErrValidationFailed).
		WithField("problem_topic_ids", httpx.IssueInvalid,
			"Must include the current CM report problem_topic_id. Change the CM report first or keep that topic in the list.")
}

func checkPmScheduleTypePatch(wo sqlc.WorkOrder, pmScheduleType *string) error {
	if wo.WorkOrderType != "PM" {
		return httpx.Err(httpx.ErrPmScheduleTypeNotAllowed).
			WithField("pm_schedule_type", httpx.IssueInvalid, "Must not be set when work_order_type is CM.")
	}
	if _, ok := pmScheduleTypeEditableStatuses[wo.Status]; !ok {
		return httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("pm_schedule_type", httpx.IssueInvalid,
				"Can only be changed while status is ASSIGNED, IN_PROGRESS, or PENDING.")
	}
	if pmScheduleType == nil {
		return httpx.Err(httpx.ErrPmScheduleTypeRequired).
			WithField("pm_schedule_type", httpx.IssueRequired, "Required when work_order_type is PM.")
	}
	return nil
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

// SyncProblemTopicFromReport keeps junction topics aligned with cm_reports changes.
func (s *WorkOrderService) SyncProblemTopicFromReport(
	ctx context.Context,
	tx pgx.Tx,
	workOrderID uuid.UUID,
	newTopicID *uuid.UUID,
) error {
	return s.repo.SyncProblemTopicFromReport(ctx, tx, workOrderID, newTopicID)
}

// SyncProblemTopicsFromReport adds every report topic to the work order junction.
func (s *WorkOrderService) SyncProblemTopicsFromReport(
	ctx context.Context,
	tx pgx.Tx,
	workOrderID uuid.UUID,
	topicIDs []uuid.UUID,
) error {
	return s.repo.SyncProblemTopicsFromReport(ctx, tx, workOrderID, topicIDs)
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

func normalizeProblemTopicIDs(single *uuid.UUID, many []uuid.UUID) ([]uuid.UUID, error) {
	if single != nil && *single != uuid.Nil {
		many = append([]uuid.UUID{*single}, many...)
	}
	if len(many) == 0 {
		return nil, httpx.Err(httpx.ErrCmProblemTopicRequired).
			WithField("problem_topic_id", httpx.IssueRequired, "Required when work_order_type is CM.")
	}
	seen := make(map[uuid.UUID]struct{}, len(many))
	out := make([]uuid.UUID, 0, len(many))
	for _, id := range many {
		if id == uuid.Nil {
			return nil, httpx.Err(httpx.ErrValidationFailed).
				WithField("problem_topic_id", httpx.IssueInvalid, "Must be a valid UUID.")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func checkCmProblemTopics(workOrderType string, single *uuid.UUID, many []uuid.UUID) error {
	if workOrderType == "PM" && (single != nil || len(many) > 0) {
		return httpx.Err(httpx.ErrCmProblemTopicNotAllowed).
			WithField("problem_topic_id", httpx.IssueInvalid, "Must not be set when work_order_type is PM.")
	}
	if workOrderType == "CM" {
		_, err := normalizeProblemTopicIDs(single, many)
		return err
	}
	return nil
}

func (s *WorkOrderService) resolveProblemTopics(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, *string, error) {
	resolved := make([]uuid.UUID, 0, len(ids))
	var firstTag *string
	for i, id := range ids {
		topicID, tagCode, err := s.resolveProblemTopic(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		resolved = append(resolved, *topicID)
		if i == 0 {
			firstTag = tagCode
		}
	}
	return resolved, firstTag, nil
}

func checkCmProblemTopic(workOrderType string, problemTopicID *uuid.UUID) error {
	return checkCmProblemTopics(workOrderType, problemTopicID, nil)
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
