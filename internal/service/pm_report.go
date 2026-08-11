package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// PmReportService applies the business rules of the PM report aggregate: the
// report itself, its checklist, ground test and power test. One report ties
// to exactly one work order round (rtu.pm_reports.work_order_round_id), so
// every write here is scoped through the work order's *current* round;
// history across rounds (rework after a rejection) is read-only.
type PmReportService struct {
	repo       *repository.PmReportRepository
	workOrders *WorkOrderService
	devices    *repository.PanelDeviceRepository
	notify     *NotificationService
}

// ChecklistResultInput is one line of the checklist submitted with a report.
type ChecklistResultInput struct {
	ChecklistItemID uuid.UUID  `json:"checklist_item_id" validate:"required"`
	PanelDeviceID   *uuid.UUID `json:"panel_device_id"`
	Status          *string    `json:"status" validate:"omitempty,max=20"`
	Value           *string    `json:"value" validate:"omitempty,max=255"`
	MeterNo         *string    `json:"meter_no" validate:"omitempty,max=50"`
	Note            *string    `json:"note" validate:"omitempty,max=4000"`
	CheckedBy       *uuid.UUID `json:"checked_by"`
	CheckedAt       *time.Time `json:"checked_at"`
}

// GroundTestInput is the optional ground resistance/voltage test.
type GroundTestInput struct {
	ResistanceLg *decimal.Decimal `json:"resistance_lg"`
	ResistanceNg *decimal.Decimal `json:"resistance_ng"`
	VoltageLg    *decimal.Decimal `json:"voltage_lg"`
	VoltageNg    *decimal.Decimal `json:"voltage_ng"`
	Result       *string          `json:"result" validate:"omitempty,oneof=PASS FAIL"`
	Note         *string          `json:"note" validate:"omitempty,max=4000"`
	MeasuredBy   *uuid.UUID       `json:"measured_by"`
	MeasuredAt   *time.Time       `json:"measured_at"`
}

// PowerTestPointInput is one equipment row (breaker or DC supply) of the
// power test.
type PowerTestPointInput struct {
	EquipmentRole     string           `json:"equipment_role" validate:"required,oneof=CIRCUIT_BREAKER DC_POWER_SUPPLY"`
	Brand             *string          `json:"brand" validate:"omitempty,max=255"`
	Model             *string          `json:"model" validate:"omitempty,max=255"`
	InputAcceptRange  *string          `json:"input_accept_range" validate:"omitempty,max=100"`
	InputResultValue  *decimal.Decimal `json:"input_result_value"`
	InputUnit         *string          `json:"input_unit" validate:"omitempty,max=20"`
	OutputAcceptRange *string          `json:"output_accept_range" validate:"omitempty,max=100"`
	OutputResultValue *decimal.Decimal `json:"output_result_value"`
	OutputUnit        *string          `json:"output_unit" validate:"omitempty,max=20"`
	Result            *string          `json:"result" validate:"omitempty,oneof=ACCEPT NOT_ACCEPTED"`
	CorrectiveAction  *string          `json:"corrective_action" validate:"omitempty,max=4000"`
}

// PowerTestInput is the optional power supply test, with its equipment
// points.
type PowerTestInput struct {
	InstrumentID *uuid.UUID            `json:"instrument_id"`
	TestedBy     *uuid.UUID            `json:"tested_by"`
	TestedAt     *time.Time            `json:"tested_at"`
	Points       []PowerTestPointInput `json:"points"`
}

// PmReportSaveInput is the PUT /work-orders/{id}/pm-report body. It replaces
// the report's meta fields and its whole child set in one call — safe to
// send repeatedly while the technician is filling the form; every field
// (including nil) is authoritative, there is no partial-update semantics.
type PmReportSaveInput struct {
	EngineerID *uuid.UUID             `json:"engineer_id"`
	Note       *string                `json:"note" validate:"omitempty,max=4000"`
	ReportDate *time.Time             `json:"report_date"`
	Checklist  []ChecklistResultInput `json:"checklist_results"`
	Ground     *GroundTestInput       `json:"ground_test"`
	Power      *PowerTestInput        `json:"power_test"`
}

// PmReportSubmitInput is the POST /work-orders/{id}/pm-report/submit body.
type PmReportSubmitInput struct {
	ActorID uuid.UUID `json:"actor_id" validate:"required"`
}

// SaveForWorkOrder upserts the PM report tied to a work order's current
// round.
func (s *PmReportService) SaveForWorkOrder(ctx context.Context, workOrderID uuid.UUID, in PmReportSaveInput) (repository.PmReportDetail, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return repository.PmReportDetail{}, err
	}
	if wo.WorkOrderType != "PM" {
		return repository.PmReportDetail{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("work_order_id", httpx.IssueInvalid, "A PM report can only be attached to a PM work order.")
	}
	if wo.CurrentRoundID == nil {
		return repository.PmReportDetail{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to report against.")
	}

	if err := s.checkDevicesInPanel(ctx, wo.PanelID, in.Checklist); err != nil {
		return repository.PmReportDetail{}, err
	}

	save := repository.SaveInput{
		EngineerID: in.EngineerID,
		Note:       in.Note,
		ReportDate: in.ReportDate,
		Checklist:  toChecklistResultInputs(in.Checklist),
		Ground:     toGroundTestInput(in.Ground),
		Power:      toPowerTestInput(in.Power),
	}

	return s.repo.Save(ctx, workOrderID, *wo.CurrentRoundID, wo.PanelID, save)
}

// GetForWorkOrder returns the report tied to a work order's current round.
func (s *PmReportService) GetForWorkOrder(ctx context.Context, workOrderID uuid.UUID) (repository.PmReportDetail, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return repository.PmReportDetail{}, err
	}
	if wo.CurrentRoundID == nil {
		return repository.PmReportDetail{}, httpx.Err(httpx.ErrPmReportNotFnd)
	}
	return s.repo.GetDetailByRound(ctx, *wo.CurrentRoundID)
}

// Get returns a single report by id.
func (s *PmReportService) Get(ctx context.Context, id uuid.UUID) (repository.PmReportDetail, error) {
	return s.repo.GetDetail(ctx, id)
}

// Submit finalizes the PM report of a work order's current round and moves
// the work order to PENDING_APPROVAL.
func (s *PmReportService) Submit(ctx context.Context, workOrderID uuid.UUID, actorID uuid.UUID) (repository.PmReportDetail, error) {
	wo, err := s.workOrders.Get(ctx, workOrderID)
	if err != nil {
		return repository.PmReportDetail{}, err
	}
	if wo.CurrentRoundID == nil {
		return repository.PmReportDetail{}, httpx.Err(httpx.ErrWorkOrderStatusInvalid).
			WithField("id", httpx.IssueInvalid, "This work order has no active round to submit.")
	}

	report, err := s.repo.GetDetailByRound(ctx, *wo.CurrentRoundID)
	if err != nil {
		return repository.PmReportDetail{}, err
	}
	if report.Status != "DRAFT" {
		return repository.PmReportDetail{}, httpx.Err(httpx.ErrPmReportNotDraft)
	}
	if err := s.checkScheduleRequirements(wo, report); err != nil {
		return repository.PmReportDetail{}, err
	}

	now := time.Now()
	if _, err := s.repo.Submit(ctx, report.ID, actorID, now); err != nil {
		return repository.PmReportDetail{}, err
	}
	if _, err := s.workOrders.MarkSubmitted(ctx, workOrderID, now, actorID); err != nil {
		return repository.PmReportDetail{}, err
	}
	if s.notify != nil && wo.RequestedBy != uuid.Nil {
		_, _ = s.notify.Create(ctx, NotificationCreateInput{
			WorkOrderID: workOrderID,
			RecipientID: wo.RequestedBy,
			Type:        "PENDING_APPROVAL",
			Title:       stringPtr("PM report submitted"),
			Message:     stringPtr("Work order " + wo.WorkOrderNo + " is waiting for approval."),
		})
	}

	return s.repo.GetDetail(ctx, report.ID)
}

// checkScheduleRequirements enforces the 3-month / 6-month section rules from
// System Design Screen 03 before a PM report can be submitted.
func (s *PmReportService) checkScheduleRequirements(wo repository.WorkOrderView, report repository.PmReportDetail) error {
	if wo.PmScheduleType == nil {
		return nil
	}
	switch *wo.PmScheduleType {
	case "THREE_MONTH":
		if report.PowerTest == nil {
			return httpx.Err(httpx.ErrPmPowerTestRequired).
				WithField("power_test", httpx.IssueRequired, "Required for a 3-month PM.")
		}
	case "SIX_MONTH":
		if len(report.Calibrations) == 0 {
			return httpx.Err(httpx.ErrPmCalibrationRequired).
				WithField("calibrations", httpx.IssueRequired, "Required for a 6-month PM.")
		}
	}
	return nil
}

// ListHistoryByWorkOrder returns every report of a work order across its
// rounds (i.e. what happened on each rework attempt).
func (s *PmReportService) ListHistoryByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]repository.PmReportHistoryItem, error) {
	return s.repo.ListByWorkOrder(ctx, workOrderID)
}

// ListHistoryByPanel returns the PM report history of a panel.
func (s *PmReportService) ListHistoryByPanel(ctx context.Context, panelID uuid.UUID, page httpx.Page) ([]repository.PmReportHistoryItem, int64, error) {
	return s.repo.ListByPanel(ctx, panelID, page)
}

// Delete removes a report while it is still a DRAFT.
func (s *PmReportService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *PmReportService) checkDevicesInPanel(ctx context.Context, panelID uuid.UUID, checklist []ChecklistResultInput) error {
	seen := make(map[uuid.UUID]bool)
	for _, item := range checklist {
		if item.PanelDeviceID == nil || seen[*item.PanelDeviceID] {
			continue
		}
		seen[*item.PanelDeviceID] = true

		device, err := s.devices.Get(ctx, *item.PanelDeviceID)
		if err != nil {
			return err
		}
		if device.PanelID != panelID {
			return httpx.Err(httpx.ErrDeviceNotInPanel).
				WithField("panel_device_id", httpx.IssueInvalid, "Must belong to the panel of this work order.")
		}
	}
	return nil
}

func toChecklistResultInputs(items []ChecklistResultInput) []repository.ChecklistResultInput {
	out := make([]repository.ChecklistResultInput, len(items))
	for i, it := range items {
		out[i] = repository.ChecklistResultInput{
			ChecklistItemID: it.ChecklistItemID,
			PanelDeviceID:   it.PanelDeviceID,
			Status:          it.Status,
			Value:           it.Value,
			MeterNo:         it.MeterNo,
			Note:            it.Note,
			CheckedBy:       it.CheckedBy,
			CheckedAt:       it.CheckedAt,
		}
	}
	return out
}

func toGroundTestInput(in *GroundTestInput) *repository.GroundTestInput {
	if in == nil {
		return nil
	}
	return &repository.GroundTestInput{
		ResistanceLg: in.ResistanceLg,
		ResistanceNg: in.ResistanceNg,
		VoltageLg:    in.VoltageLg,
		VoltageNg:    in.VoltageNg,
		Result:       in.Result,
		Note:         in.Note,
		MeasuredBy:   in.MeasuredBy,
		MeasuredAt:   in.MeasuredAt,
	}
}

func toPowerTestInput(in *PowerTestInput) *repository.PowerTestInput {
	if in == nil {
		return nil
	}
	points := make([]repository.PowerTestPointInput, len(in.Points))
	for i, pt := range in.Points {
		points[i] = repository.PowerTestPointInput{
			EquipmentRole:     pt.EquipmentRole,
			Brand:             pt.Brand,
			Model:             pt.Model,
			InputAcceptRange:  pt.InputAcceptRange,
			InputResultValue:  pt.InputResultValue,
			InputUnit:         pt.InputUnit,
			OutputAcceptRange: pt.OutputAcceptRange,
			OutputResultValue: pt.OutputResultValue,
			OutputUnit:        pt.OutputUnit,
			Result:            pt.Result,
			CorrectiveAction:  pt.CorrectiveAction,
		}
	}
	return &repository.PowerTestInput{
		InstrumentID: in.InstrumentID,
		TestedBy:     in.TestedBy,
		TestedAt:     in.TestedAt,
		Points:       points,
	}
}
