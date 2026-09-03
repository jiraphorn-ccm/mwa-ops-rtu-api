package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// CmReportService applies the business rules of rtu.cm_reports across its
// two write paths:
//
//	SaveForWorkOrder / Submit: STANDALONE and PM_ESCALATED origins — a CM
//	  work order's current round, exactly mirroring PmReportService.
//	CreateOnsiteFix: PM_ONSITE_FIX origin — a fix made on the spot during a
//	  PM visit, with no work order of its own.
//	EscalateFromPm: PM_ESCALATED origin opened mid-PM ("Report an issue") —
//	  creates/reuses a CM work order and a pending cm_report linked back to
//	  the PM report.
type CmReportService struct {
	repo          *repository.CmReportRepository
	workOrders    *WorkOrderService
	pmReports     *repository.PmReportRepository
	devices       *repository.PanelDeviceRepository
	activity      *repository.WorkOrderActivityLogRepository
	problemTopics *repository.ProblemTopicRepository
	notify        *NotificationService
}

// CmReportSaveInput is the PUT /work-orders/{id}/cm-report body.
type CmReportSaveInput struct {
	// ReportedBy defaults to the work order's requested_by when omitted.
	ReportedBy       *uuid.UUID `json:"reported_by"`
	PanelDeviceID    *uuid.UUID `json:"panel_device_id"`
	ProblemTopicID   uuid.UUID  `json:"problem_topic_id" validate:"required"`
	TagCode          *string    `json:"tag_code" validate:"omitempty,max=100"`
	ErrorLogs        *string    `json:"error_logs"`
	ProblemDetail    *string    `json:"problem_detail"`
	RootCause        *string    `json:"root_cause"`
	ReferenceInfo    *string    `json:"reference_info"`
	CorrectiveAction *string    `json:"corrective_action"`
	Recommendation   *string    `json:"recommendation"`
	PendingReason    *string    `json:"pending_reason"`
	RepairedBy       *uuid.UUID `json:"repaired_by"`
	ReportedAt       *time.Time `json:"reported_at"`
	StartedAt        *time.Time `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
}

// CmReportEscalateInput is the POST /pm-reports/{id}/escalate body —
// "Report an issue" during a PM visit when the repair cannot be finished
// on the spot (System Design: spawn CM Work Order, Status = Pending).
type CmReportEscalateInput struct {
	PendingReason string      `json:"pending_reason" validate:"required,max=4000"`
	ReportedBy    uuid.UUID   `json:"reported_by" validate:"required"`
	AssignedTo    uuid.UUID   `json:"assigned_to" validate:"required"`
	AssignedBy    uuid.UUID   `json:"assigned_by" validate:"required"`
	PanelDeviceID  *uuid.UUID  `json:"panel_device_id"`
	ProblemTopicID uuid.UUID   `json:"problem_topic_id" validate:"required"`
	TagCode        *string     `json:"tag_code" validate:"omitempty,max=100"`
	ErrorLogs     *string     `json:"error_logs"`
	ProblemDetail *string     `json:"problem_detail"`
	RepairDate    *httpx.Date `json:"repair_date"`
}

// CmReportOnsiteInput is the POST /pm-reports/{id}/onsite-fixes body — a
// repair made during the PM visit itself, before the PM report is even
// submitted.
type CmReportOnsiteInput struct {
	PanelDeviceID    *uuid.UUID `json:"panel_device_id"`
	ReportedBy       uuid.UUID  `json:"reported_by" validate:"required"`
	ProblemTopicID   *uuid.UUID `json:"problem_topic_id"`
	TagCode          *string    `json:"tag_code" validate:"omitempty,max=100"`
	ErrorLogs        *string    `json:"error_logs"`
	ProblemDetail    *string    `json:"problem_detail"`
	RootCause        *string    `json:"root_cause"`
	ReferenceInfo    *string    `json:"reference_info"`
	CorrectiveAction *string    `json:"corrective_action"`
	Recommendation   *string    `json:"recommendation"`
	RepairedBy       *uuid.UUID `json:"repaired_by"`
	StartedAt        *time.Time `json:"started_at"`
	// EndedAt defaults to now: an onsite-fix record only ever gets created
	// once the repair actually succeeded (see rtu.cm_reports note in
	// doc/rtu-full-schema.dbml), so it is implicitly complete on creation.
	EndedAt *time.Time `json:"ended_at"`
}

// CmReportUpdateInput is the PATCH /cm-reports/{id} body — used for any
// origin once the record exists.
type CmReportUpdateInput struct {
	PanelDeviceID    *uuid.UUID `json:"panel_device_id"`
	ProblemTopicID   *uuid.UUID `json:"problem_topic_id"`
	TagCode          *string    `json:"tag_code" validate:"omitempty,max=100"`
	ErrorLogs        *string    `json:"error_logs"`
	ProblemDetail    *string    `json:"problem_detail"`
	RootCause        *string    `json:"root_cause"`
	ReferenceInfo    *string    `json:"reference_info"`
	CorrectiveAction *string    `json:"corrective_action"`
	Recommendation   *string    `json:"recommendation"`
	PendingReason    *string    `json:"pending_reason"`
	RepairedBy       *uuid.UUID `json:"repaired_by"`
	ReportedAt       *time.Time `json:"reported_at"`
	StartedAt        *time.Time `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
}

// CmReportSubmitInput is the POST /work-orders/{id}/cm-report/submit body.
type CmReportSubmitInput struct {
	ActorID uuid.UUID `json:"actor_id" validate:"required"`
}

// SaveForWorkOrder upserts the CM report tied to a work order's current
// round (STANDALONE, or PM_ESCALATED when the work order's
// related_work_order_id points back at the PM it came from).
func (s *CmReportService) SaveForWorkOrder(ctx context.Context, workOrderID uuid.UUID, in CmReportSaveInput) (sqlc.CmReport, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if wo.WorkOrderType != "CM" {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("work_order_id", httpx.IssueInvalid, "A CM report can only be attached to a CM work order.")
	}
	if wo.CurrentRoundID == nil {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to report against.")
	}

	if err := s.checkDeviceInPanel(ctx, wo.PanelID, in.PanelDeviceID); err != nil {
		return sqlc.CmReport{}, err
	}
	problemTopicID, tagCode, err := s.resolveProblemTopic(ctx, &in.ProblemTopicID, in.TagCode)
	if err != nil {
		return sqlc.CmReport{}, err
	}

	var report sqlc.CmReport
	err = s.repo.WithPanelCmLock(ctx, wo.PanelID, func(tx pgx.Tx, q *sqlc.Queries) error {
		if err := repository.EnsureNoOpenCmConflict(ctx, tx, wo.PanelID, repository.OpenCmWorkOrderFilter{
			ProblemTopicID:     problemTopicID,
			ExcludeWorkOrderID: &workOrderID,
		}); err != nil {
			return err
		}

		existing, err := s.repo.FindByRoundQ(ctx, q, *wo.CurrentRoundID)
		if err != nil {
			return err
		}

		if err := s.workOrders.SyncProblemTopicFromReport(ctx, tx, workOrderID, problemTopicID); err != nil {
			return err
		}

		if existing != nil {
			report, err = s.repo.UpdateQ(ctx, q, sqlc.UpdateCmReportParams{
				ID:                       existing.ID,
				PanelDeviceID:            in.PanelDeviceID,
				PanelDeviceIDDoUpdate:    true,
				ProblemTopicID:           problemTopicID,
				ProblemTopicIDDoUpdate:   true,
				TagCode:                  tagCode,
				TagCodeDoUpdate:          true,
				ErrorLogs:                in.ErrorLogs,
				ErrorLogsDoUpdate:        true,
				ProblemDetail:            in.ProblemDetail,
				ProblemDetailDoUpdate:    true,
				RootCause:                in.RootCause,
				RootCauseDoUpdate:        true,
				ReferenceInfo:            in.ReferenceInfo,
				ReferenceInfoDoUpdate:    true,
				CorrectiveAction:         in.CorrectiveAction,
				CorrectiveActionDoUpdate: true,
				Recommendation:           in.Recommendation,
				RecommendationDoUpdate:   true,
				PendingReason:            in.PendingReason,
				PendingReasonDoUpdate:    true,
				RepairedBy:               in.RepairedBy,
				RepairedByDoUpdate:       true,
				ReportedAt:               in.ReportedAt,
				ReportedAtDoUpdate:       true,
				StartedAt:                in.StartedAt,
				StartedAtDoUpdate:        true,
				EndedAt:                  in.EndedAt,
				EndedAtDoUpdate:          true,
			})
			return err
		}

		reportedBy := wo.RequestedBy
		if in.ReportedBy != nil {
			reportedBy = *in.ReportedBy
		}

		report, err = s.repo.CreateQ(ctx, q, sqlc.CreateCmReportParams{
			WorkOrderID:      &workOrderID,
			WorkOrderRoundID: wo.CurrentRoundID,
			PanelID:          wo.PanelID,
			PanelDeviceID:    in.PanelDeviceID,
			ReportedBy:       reportedBy,
			ProblemTopicID:   problemTopicID,
			TagCode:          tagCode,
			ErrorLogs:        in.ErrorLogs,
			ProblemDetail:    in.ProblemDetail,
			RootCause:        in.RootCause,
			ReferenceInfo:    in.ReferenceInfo,
			CorrectiveAction: in.CorrectiveAction,
			Recommendation:   in.Recommendation,
			PendingReason:    in.PendingReason,
			RepairedBy:       in.RepairedBy,
			ReportedAt:       in.ReportedAt,
			StartedAt:        in.StartedAt,
			EndedAt:          in.EndedAt,
		})
		return err
	})
	if err != nil {
		if conflict := openCmConflictError(err); conflict != nil {
			return sqlc.CmReport{}, appErrFromOpenCmConflict(conflict.Conflict)
		}
		return sqlc.CmReport{}, err
	}
	return report, nil
}

// GetForWorkOrder returns the CM report tied to a work order's current round.
func (s *CmReportService) GetForWorkOrder(ctx context.Context, workOrderID uuid.UUID) (sqlc.CmReport, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if wo.CurrentRoundID == nil {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrCmReportNotFnd)
	}
	return s.repo.GetByRound(ctx, *wo.CurrentRoundID)
}

// Submit stamps ended_at (if not already set) and moves the work order to
// PENDING_APPROVAL.
func (s *CmReportService) Submit(ctx context.Context, workOrderID uuid.UUID, actorID uuid.UUID) (sqlc.CmReport, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if wo.CurrentRoundID == nil {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to submit.")
	}

	report, err := s.repo.GetByRound(ctx, *wo.CurrentRoundID)
	if err != nil {
		return sqlc.CmReport{}, err
	}

	now := time.Now()
	if report.EndedAt == nil {
		report, err = s.repo.Update(ctx, sqlc.UpdateCmReportParams{
			ID: report.ID, EndedAt: &now, EndedAtDoUpdate: true,
		})
		if err != nil {
			return sqlc.CmReport{}, err
		}
	}

	if _, err := s.workOrders.MarkSubmitted(ctx, workOrderID, now, actorID); err != nil {
		return sqlc.CmReport{}, err
	}
	return report, nil
}

// CreateOnsiteFix records a repair made on the spot during a PM visit.
func (s *CmReportService) CreateOnsiteFix(ctx context.Context, pmReportID uuid.UUID, in CmReportOnsiteInput) (sqlc.CmReport, error) {
	pmReport, err := s.pmReports.GetDetail(ctx, pmReportID)
	if err != nil {
		return sqlc.CmReport{}, err
	}

	if err := s.checkDeviceInPanel(ctx, pmReport.PanelID, in.PanelDeviceID); err != nil {
		return sqlc.CmReport{}, err
	}
	problemTopicID, tagCode, err := s.resolveProblemTopic(ctx, in.ProblemTopicID, in.TagCode)
	if err != nil {
		return sqlc.CmReport{}, err
	}

	endedAt := in.EndedAt
	if endedAt == nil {
		now := time.Now()
		endedAt = &now
	}

	return s.repo.Create(ctx, sqlc.CreateCmReportParams{
		PmReportID:       &pmReportID,
		PanelID:          pmReport.PanelID,
		PanelDeviceID:    in.PanelDeviceID,
		ReportedBy:       in.ReportedBy,
		ProblemTopicID:   problemTopicID,
		TagCode:          tagCode,
		ErrorLogs:        in.ErrorLogs,
		ProblemDetail:    in.ProblemDetail,
		RootCause:        in.RootCause,
		ReferenceInfo:    in.ReferenceInfo,
		CorrectiveAction: in.CorrectiveAction,
		Recommendation:   in.Recommendation,
		RepairedBy:       in.RepairedBy,
		StartedAt:        in.StartedAt,
		EndedAt:          endedAt,
	})
}

// EscalateFromPm opens (or reuses) a CM work order for a problem found during
// PM that cannot be fixed on the spot — PM_ESCALATED origin. The CM work
// order starts in PENDING; the technician continues the PM checklist.
func (s *CmReportService) EscalateFromPm(ctx context.Context, pmReportID uuid.UUID, in CmReportEscalateInput) (sqlc.CmReport, error) {
	pmReport, err := s.pmReports.GetDetail(ctx, pmReportID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	wo, err := s.workOrders.Get(ctx, pmReport.WorkOrderID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if wo.WorkOrderType != "PM" || (wo.Status != "IN_PROGRESS" && wo.Status != "ASSIGNED" && wo.Status != "PENDING") {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrPmEscalateStatusInvalid)
	}
	if err := s.checkDeviceInPanel(ctx, pmReport.PanelID, in.PanelDeviceID); err != nil {
		return sqlc.CmReport{}, err
	}
	problemTopicID, tagCode, err := s.resolveProblemTopic(ctx, &in.ProblemTopicID, in.TagCode)
	if err != nil {
		return sqlc.CmReport{}, err
	}

	cmID, err := s.resolveOrCreateCM(ctx, wo, in, problemTopicID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	cmWO, err := s.workOrders.Get(ctx, cmID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if cmWO.CurrentRoundID == nil {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "CM work order has no active round.")
	}

	existing, err := s.repo.FindByRound(ctx, *cmWO.CurrentRoundID)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if existing == nil {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrCmReportNotFnd).
			WithField("work_order_id", httpx.IssueInvalid, "CM work order is missing its seeded report.")
	}

	now := time.Now()
	var report sqlc.CmReport
	err = s.repo.WithPanelCmLock(ctx, pmReport.PanelID, func(tx pgx.Tx, q *sqlc.Queries) error {
		if err := s.workOrders.SyncProblemTopicFromReport(ctx, tx, cmID, problemTopicID); err != nil {
			return err
		}
		var err error
		report, err = s.repo.UpdateQ(ctx, q, sqlc.UpdateCmReportParams{
			ID:                       existing.ID,
			PmReportID:               &pmReportID,
			PmReportIDDoUpdate:       true,
			PanelDeviceID:            in.PanelDeviceID,
			PanelDeviceIDDoUpdate:    true,
			ProblemTopicID:           problemTopicID,
			ProblemTopicIDDoUpdate:   true,
			TagCode:                  tagCode,
			TagCodeDoUpdate:          true,
			ErrorLogs:                in.ErrorLogs,
			ErrorLogsDoUpdate:        true,
			ProblemDetail:            in.ProblemDetail,
			ProblemDetailDoUpdate:    true,
			PendingReason:            &in.PendingReason,
			PendingReasonDoUpdate:    true,
			ReportedAt:               &now,
			ReportedAtDoUpdate:       true,
		})
		return err
	})
	if err != nil {
		return sqlc.CmReport{}, err
	}

	if s.activity != nil {
		_, _ = s.activity.Create(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID: pmReport.WorkOrderID,
			Action:      "CM_SPAWNED",
			ActorID:     in.ReportedBy,
			Note:        stringPtr("Escalated issue to CM work order " + cmWO.WorkOrderNo),
		})
	}
	if s.notify != nil {
		_, _ = s.notify.Create(ctx, NotificationCreateInput{
			WorkOrderID: cmID,
			RecipientID: wo.RequestedBy,
			Type:        "CM_PENDING",
			Title:       stringPtr("CM pending from PM"),
			Message:     stringPtr("Issue escalated from " + wo.WorkOrderNo + ": " + in.PendingReason),
		})
	}
	return report, nil
}

func (s *CmReportService) resolveOrCreateCM(ctx context.Context, pmWO repository.WorkOrderView, in CmReportEscalateInput, problemTopicID *uuid.UUID) (uuid.UUID, error) {
	cm, err := s.workOrders.Create(ctx, WorkOrderCreateInput{
		PanelID:            pmWO.PanelID,
		WorkOrderType:      "CM",
		PanelDeviceID:      in.PanelDeviceID,
		Title:              stringPtr("Escalated from " + pmWO.WorkOrderNo),
		Description:        &in.PendingReason,
		RequestedBy:        in.ReportedBy,
		AssignedTo:         in.AssignedTo,
		AssignedBy:         in.AssignedBy,
		RelatedWorkOrderID: &pmWO.ID,
		DueDate:            in.RepairDate,
		ProblemTopicID:     problemTopicID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return cm.ID, nil
}

// ListByPmReport returns every onsite fix recorded during one PM visit.
func (s *CmReportService) ListByPmReport(ctx context.Context, pmReportID uuid.UUID) ([]sqlc.CmReport, error) {
	return s.repo.ListByPmReport(ctx, pmReportID)
}

// Get returns a single CM report.
func (s *CmReportService) Get(ctx context.Context, id uuid.UUID) (sqlc.CmReport, error) {
	return s.repo.Get(ctx, id)
}

// Update applies a partial update to any CM report, regardless of origin.
func (s *CmReportService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in CmReportUpdateInput) (sqlc.CmReport, error) {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return sqlc.CmReport{}, err
	}

	if fields.Has("panel_device_id") {
		if err := s.checkDeviceInPanel(ctx, current.PanelID, in.PanelDeviceID); err != nil {
			return sqlc.CmReport{}, err
		}
	}

	problemTopicID, tagCode, err := s.resolveProblemTopicPatch(ctx, fields, in.ProblemTopicID, in.TagCode)
	if err != nil {
		return sqlc.CmReport{}, err
	}
	if fields.Has("problem_topic_id") && problemTopicID == nil {
		return sqlc.CmReport{}, httpx.Err(httpx.ErrCmProblemTopicRequired).
			WithField("problem_topic_id", httpx.IssueRequired, "Required when work_order_type is CM.")
	}

	params := s.buildCmReportUpdateParams(id, fields, in, problemTopicID, tagCode)

	if fields.Has("problem_topic_id") {
		var updated sqlc.CmReport
		err = s.repo.WithPanelCmLock(ctx, current.PanelID, func(tx pgx.Tx, q *sqlc.Queries) error {
			filter := repository.OpenCmWorkOrderFilter{ProblemTopicID: problemTopicID}
			if current.WorkOrderID != nil {
				filter.ExcludeWorkOrderID = current.WorkOrderID
			}
			if err := repository.EnsureNoOpenCmConflict(ctx, tx, current.PanelID, filter); err != nil {
				return err
			}
			if current.WorkOrderID != nil {
				if err := s.workOrders.SyncProblemTopicFromReport(ctx, tx, *current.WorkOrderID, problemTopicID); err != nil {
					return err
				}
			}
			var err error
			updated, err = s.repo.UpdateQ(ctx, q, params)
			return err
		})
		if err != nil {
			if conflict := openCmConflictError(err); conflict != nil {
				return sqlc.CmReport{}, appErrFromOpenCmConflict(conflict.Conflict)
			}
			return sqlc.CmReport{}, err
		}
		return updated, nil
	}

	return s.repo.Update(ctx, params)
}

func (s *CmReportService) buildCmReportUpdateParams(
	id uuid.UUID,
	fields httpx.FieldSet,
	in CmReportUpdateInput,
	problemTopicID *uuid.UUID,
	tagCode *string,
) sqlc.UpdateCmReportParams {
	params := sqlc.UpdateCmReportParams{ID: id}
	params.PanelDeviceID, params.PanelDeviceIDDoUpdate = patchNullable(fields, "panel_device_id", in.PanelDeviceID)
	if fields.Has("problem_topic_id") {
		params.ProblemTopicID = problemTopicID
		params.ProblemTopicIDDoUpdate = true
		params.TagCode = tagCode
		params.TagCodeDoUpdate = true
	} else if fields.Has("tag_code") {
		params.TagCode, params.TagCodeDoUpdate = patchNullable(fields, "tag_code", in.TagCode)
	}
	params.ErrorLogs, params.ErrorLogsDoUpdate = patchNullable(fields, "error_logs", in.ErrorLogs)
	params.ProblemDetail, params.ProblemDetailDoUpdate = patchNullable(fields, "problem_detail", in.ProblemDetail)
	params.RootCause, params.RootCauseDoUpdate = patchNullable(fields, "root_cause", in.RootCause)
	params.ReferenceInfo, params.ReferenceInfoDoUpdate = patchNullable(fields, "reference_info", in.ReferenceInfo)
	params.CorrectiveAction, params.CorrectiveActionDoUpdate = patchNullable(fields, "corrective_action", in.CorrectiveAction)
	params.Recommendation, params.RecommendationDoUpdate = patchNullable(fields, "recommendation", in.Recommendation)
	params.PendingReason, params.PendingReasonDoUpdate = patchNullable(fields, "pending_reason", in.PendingReason)
	params.RepairedBy, params.RepairedByDoUpdate = patchNullable(fields, "repaired_by", in.RepairedBy)
	params.ReportedAt, params.ReportedAtDoUpdate = patchNullable(fields, "reported_at", in.ReportedAt)
	params.StartedAt, params.StartedAtDoUpdate = patchNullable(fields, "started_at", in.StartedAt)
	params.EndedAt, params.EndedAtDoUpdate = patchNullable(fields, "ended_at", in.EndedAt)
	return params
}

// ListHistoryByPanel returns the repair history of a panel.
func (s *CmReportService) ListHistoryByPanel(ctx context.Context, panelID uuid.UUID, page httpx.Page) ([]repository.CmReportHistoryItem, int64, error) {
	return s.repo.ListByPanel(ctx, panelID, page)
}

// ListHistoryByPanelDevice returns the repair history of a single device.
func (s *CmReportService) ListHistoryByPanelDevice(ctx context.Context, panelDeviceID uuid.UUID, page httpx.Page) ([]repository.CmReportHistoryItem, int64, error) {
	return s.repo.ListByPanelDevice(ctx, panelDeviceID, page)
}

// ListHistoryByWorkOrder returns every CM report across a work order's
// rounds.
func (s *CmReportService) ListHistoryByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]repository.CmReportHistoryItem, error) {
	return s.repo.ListByWorkOrder(ctx, workOrderID)
}

// Delete removes a CM report permanently.
func (s *CmReportService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *CmReportService) checkDeviceInPanel(ctx context.Context, panelID uuid.UUID, deviceID *uuid.UUID) error {
	if deviceID == nil {
		return nil
	}
	device, err := s.devices.Get(ctx, *deviceID)
	if err != nil {
		return err
	}
	if device.PanelID != panelID {
		return httpx.Err(httpx.ErrDeviceNotInPanel).
			WithField("panel_device_id", httpx.IssueInvalid, "Must belong to the panel of this report.")
	}
	return nil
}

// resolveProblemTopic validates an optional master topic and mirrors its code
// into tag_code so legacy clients keep working.
func (s *CmReportService) resolveProblemTopic(ctx context.Context, topicID *uuid.UUID, tagCode *string) (*uuid.UUID, *string, error) {
	if topicID == nil {
		return nil, tagCode, nil
	}
	if s.problemTopics == nil {
		return topicID, tagCode, nil
	}
	topic, err := s.problemTopics.Get(ctx, *topicID)
	if err != nil {
		return nil, nil, err
	}
	if !topic.Active {
		return nil, nil, httpx.Err(httpx.ErrProblemTopicInactive)
	}
	code := topic.Code
	return topicID, &code, nil
}

func (s *CmReportService) resolveProblemTopicPatch(ctx context.Context, fields httpx.FieldSet, topicID *uuid.UUID, tagCode *string) (*uuid.UUID, *string, error) {
	if !fields.Has("problem_topic_id") {
		return nil, nil, nil
	}
	if topicID == nil {
		return nil, tagCode, nil
	}
	return s.resolveProblemTopic(ctx, topicID, tagCode)
}
