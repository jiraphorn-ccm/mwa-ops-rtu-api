package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// ApprovalService applies the business rules of rtu.wo_approvals: it decides
// what happens to a work order after its current round is reviewed.
//
//	APPROVED            -> work order COMPLETED
//	APPROVED_CONDITION  -> work order CONDITIONAL
//	REJECTED, rework    -> same work order opens round_no+1, status PENDING
//	REJECTED, escalate  -> problem is not the PM contractor's responsibility;
//	                       work order goes CONDITIONAL and a CM work order is
//	                       created (or an existing open one reused) — see
//	                       rtu.wo_approvals note in doc/rtu-full-schema.dbml.
type ApprovalService struct {
	repo        *repository.WoApprovalRepository
	workOrders  *WorkOrderService
	activityLog *repository.WorkOrderActivityLogRepository
	panels      *repository.PanelRepository
	notify      *NotificationService
}

// ApprovalDecisionInput is the POST /work-orders/{id}/approvals body.
type ApprovalDecisionInput struct {
	ReviewerID uuid.UUID `json:"reviewer_id" validate:"required"`
	Decision   string    `json:"decision" validate:"required,oneof=APPROVED APPROVED_CONDITION REJECTED"`
	Note       *string   `json:"note" validate:"omitempty,max=4000"`

	// Rework path (REJECTED, Escalate=false/nil): who gets the new round.
	// Omitted = retry with the same assignee as the rejected round.
	ReassignTo *uuid.UUID `json:"reassign_to"`

	// Escalation path (REJECTED, Escalate=true): the issue is outside the PM
	// contractor's responsibility and must become its own CM work order.
	Escalate   *bool       `json:"escalate"`
	RepairDate *httpx.Date `json:"repair_date"`
	// AssignedTo is who a *newly created* CM work order's first round is
	// assigned to. Not needed when an existing open CM work order is reused.
	AssignedTo *uuid.UUID `json:"assigned_to"`
	// ProblemTopicID is required when Escalate is true — the spawned CM work
	// order is created with this topic and an initial cm_report row.
	ProblemTopicID *uuid.UUID `json:"problem_topic_id"`
}

// Decide records a review decision for a work order's current round and
// drives the resulting workflow.
func (s *ApprovalService) Decide(ctx context.Context, workOrderID uuid.UUID, in ApprovalDecisionInput) (repository.WorkOrderView, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return repository.WorkOrderView{}, err
	}
	if wo.CurrentRoundID == nil {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no round to review.")
	}
	if wo.Status != "PENDING_APPROVAL" {
		return repository.WorkOrderView{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "Only a work order in PENDING_APPROVAL can be reviewed.")
	}
	roundID := *wo.CurrentRoundID

	escalate := in.Escalate != nil && *in.Escalate
	var (
		newWorkOrderID *uuid.UUID
		repairDate     *httpx.Date
	)

	if in.Decision == "REJECTED" && escalate {
		if in.RepairDate == nil {
			return repository.WorkOrderView{}, httpx.Err(httpx.ErrApprovalRepairDateRequired).
				WithField("repair_date", httpx.IssueRequired, "Required when escalating to a CM work order.")
		}
		if in.ProblemTopicID == nil || *in.ProblemTopicID == uuid.Nil {
			return repository.WorkOrderView{}, httpx.Err(httpx.ErrCmProblemTopicRequired).
				WithField("problem_topic_id", httpx.IssueRequired, "Required when escalating to a CM work order.")
		}
		repairDate = in.RepairDate

		cmID, err := s.resolveCMWorkOrder(ctx, wo, in)
		if err != nil {
			return repository.WorkOrderView{}, err
		}
		newWorkOrderID = &cmID
	}

	var reassignTo *uuid.UUID
	if in.Decision == "REJECTED" && !escalate {
		reassignTo = wo.CurrentAssignedTo
		if in.ReassignTo != nil {
			reassignTo = in.ReassignTo
		}
		if reassignTo == nil {
			return repository.WorkOrderView{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("reassign_to", httpx.IssueRequired, "This field is required.")
		}
	}

	outcome, notifyRework := s.buildOutcome(wo, in, escalate, newWorkOrderID, reassignTo)

	if _, err := s.repo.DecideAndApply(ctx, sqlc.CreateWoApprovalParams{
		WorkOrderID:      workOrderID,
		WorkOrderRoundID: roundID,
		ReviewerID:       in.ReviewerID,
		Decision:         in.Decision,
		RepairDate:       repairDate,
		NewWorkOrderID:   newWorkOrderID,
		Note:             in.Note,
	}, outcome); err != nil {
		return repository.WorkOrderView{}, err
	}

	switch {
	case in.Decision == "APPROVED", in.Decision == "APPROVED_CONDITION":
		if err := s.syncPanelPmDates(ctx, wo); err != nil {
			return repository.WorkOrderView{}, err
		}
		s.emitCompleted(ctx, wo, in.ReviewerID)
	case in.Decision == "REJECTED" && escalate:
		if err := s.syncPanelPmDates(ctx, wo); err != nil {
			return repository.WorkOrderView{}, err
		}
		if s.notify != nil && newWorkOrderID != nil {
			_, _ = s.notify.Create(ctx, NotificationCreateInput{
				WorkOrderID: *newWorkOrderID,
				RecipientID: in.ReviewerID,
				Type:        "CM_PENDING",
				Title:       stringPtr("CM work order opened from approval"),
			})
		}
	case notifyRework != nil:
		if s.notify != nil {
			_, _ = s.notify.Create(ctx, NotificationCreateInput{
				WorkOrderID: workOrderID,
				RecipientID: *notifyRework,
				Type:        "NEW_ASSIGNMENT",
				Title:       stringPtr("Rework assigned"),
				Message:     stringPtr("Work order " + wo.WorkOrderNo + " was rejected and reassigned."),
			})
		}
	}

	return s.workOrders.Get(ctx, workOrderID)
}

func (s *ApprovalService) buildOutcome(
	wo repository.WorkOrderView,
	in ApprovalDecisionInput,
	escalate bool,
	newWorkOrderID *uuid.UUID,
	reassignTo *uuid.UUID,
) (repository.ApprovalOutcome, *uuid.UUID) {
	switch {
	case in.Decision == "APPROVED":
		return repository.ApprovalOutcome{NewStatus: "COMPLETED", CloseWO: true}, nil
	case in.Decision == "APPROVED_CONDITION":
		return repository.ApprovalOutcome{NewStatus: "CONDITIONAL", CloseWO: true}, nil
	case in.Decision == "REJECTED" && escalate:
		note := "Escalated to CM work order " + newWorkOrderID.String()
		return repository.ApprovalOutcome{
			NewStatus: "CONDITIONAL",
			CloseWO:   true,
			ActivityLog: &sqlc.CreateWorkOrderActivityLogParams{
				WorkOrderID: wo.ID,
				Action:      "CM_SPAWNED",
				ActorID:     in.ReviewerID,
				Note:        &note,
			},
		}, nil
	default:
		now := time.Now()
		return repository.ApprovalOutcome{
			NewStatus: "PENDING",
			Rework: &repository.ApprovalReworkOutcome{
				AssignedTo: *reassignTo,
				AssignedBy: in.ReviewerID,
				AssignedAt: now,
				ActorID:    in.ReviewerID,
				FromStatus: wo.Status,
			},
		}, reassignTo
	}
}

// syncPanelPmDates updates panels.last_pm_date / next_pm_date after a PM
// work order reaches COMPLETED or CONDITIONAL.
func (s *ApprovalService) syncPanelPmDates(ctx context.Context, wo repository.WorkOrderView) error {
	if s.panels == nil || wo.WorkOrderType != "PM" || wo.PmScheduleType == nil {
		return nil
	}
	last := httpx.NewDate(time.Now())
	months := 3
	if *wo.PmScheduleType == "SIX_MONTH" {
		months = 6
	}
	next := httpx.NewDate(last.AddDate(0, months, 0))
	_, err := s.panels.UpdatePmDates(ctx, wo.PanelID, last, next)
	return err
}

func (s *ApprovalService) emitCompleted(ctx context.Context, wo repository.WorkOrderView, reviewerID uuid.UUID) {
	if s.notify == nil {
		return
	}
	recipient := reviewerID
	if wo.CurrentAssignedTo != nil {
		recipient = *wo.CurrentAssignedTo
	}
	_, _ = s.notify.Create(ctx, NotificationCreateInput{
		WorkOrderID: wo.ID,
		RecipientID: recipient,
		Type:        "COMPLETED",
		Title:       stringPtr("Work order completed"),
		Message:     stringPtr("Work order " + wo.WorkOrderNo + " was approved."),
	})
}

// resolveCMWorkOrder finds a reusable CM work order for the panel, or creates
// a new one when none qualifies.
func (s *ApprovalService) resolveCMWorkOrder(ctx context.Context, wo repository.WorkOrderView, in ApprovalDecisionInput) (uuid.UUID, error) {
	if in.ProblemTopicID == nil {
		return uuid.Nil, httpx.Err(httpx.ErrCmProblemTopicRequired).
			WithField("problem_topic_id", httpx.IssueRequired, "Required when escalating to a CM work order.")
	}
	if existing, err := s.workOrders.FindMatchingOpenCm(ctx, wo.PanelID, in.ProblemTopicID); err != nil {
		return uuid.Nil, err
	} else if existing != nil {
		return *existing, nil
	}

	if in.AssignedTo == nil {
		return uuid.Nil, httpx.Err(httpx.ErrApprovalNewWorkOrderRequired).
			WithField("assigned_to", httpx.IssueRequired, "Required to create the new CM work order (no open one exists to reuse).")
	}

	cm, err := s.workOrders.Create(ctx, WorkOrderCreateInput{
		PanelID:            wo.PanelID,
		WorkOrderType:      "CM",
		PanelDeviceID:      wo.PanelDeviceID,
		Title:              stringPtr("Escalated from " + wo.WorkOrderNo),
		RequestedBy:        in.ReviewerID,
		AssignedTo:         *in.AssignedTo,
		AssignedBy:         in.ReviewerID,
		RelatedWorkOrderID: &wo.ID,
		DueDate:            in.RepairDate,
		ProblemTopicID:     in.ProblemTopicID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return cm.ID, nil
}

// ListByWorkOrder returns every approval decision made across all rounds of
// a work order, oldest first.
func (s *ApprovalService) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]sqlc.WoApproval, error) {
	return s.repo.ListByWorkOrder(ctx, workOrderID)
}

func stringPtr(s string) *string { return &s }
