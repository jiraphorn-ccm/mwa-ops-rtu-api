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

// clockSkew is the tolerance allowed on performed_at so a field tablet whose
// clock runs slightly ahead is not rejected.
const clockSkew = 5 * time.Minute

// maxReadingsPerCalibration bounds a single measurement sheet.
const maxReadingsPerCalibration = 500

// CalibrationService applies the business rules of a calibration event and its
// measurement sheet.
type CalibrationService struct {
	repo        *repository.CalibrationRepository
	readings    *repository.CalibrationReadingRepository
	devices     *repository.PanelDeviceRepository
	instruments *repository.CalibrationInstrumentRepository
	workOrders  *WorkOrderService
	pmReports   *repository.PmReportRepository
}

// CalibrationReadingInput is one row of a measurement sheet.
type CalibrationReadingInput struct {
	Sequence     *int16           `json:"sequence" validate:"omitempty,gte=1"`
	ItemLabel    *string          `json:"item_label" validate:"omitempty,max=150"`
	ParameterKey string           `json:"parameter_key" validate:"required,max=50"`
	Value        *decimal.Decimal `json:"value"`
	Unit         *string          `json:"unit" validate:"omitempty,max=20"`
}

// CalibrationCreateInput is the POST /calibrations body. Readings may be sent
// inline; the whole sheet is then written in one transaction. PanelDeviceID
// has no `validate:"required"` tag because /panel-devices/{id}/calibrations
// binds this same struct and fills PanelDeviceID from the URL *after*
// validation runs; the flat /calibrations handler enforces presence
// explicitly instead.
type CalibrationCreateInput struct {
	PanelDeviceID uuid.UUID                 `json:"panel_device_id"`
	InstrumentID  uuid.UUID                 `json:"instrument_id" validate:"required"`
	PerformedBy   *string                   `json:"performed_by" validate:"omitempty,max=100"`
	PerformedAt   time.Time                 `json:"performed_at" validate:"required"`
	Result        string                    `json:"result" validate:"required,oneof=PASS FAIL ADJUSTED"`
	Remark        *string                   `json:"remark" validate:"omitempty,max=4000"`
	WorkOrderID   *uuid.UUID                `json:"work_order_id"`
	PmReportID    *uuid.UUID                `json:"pm_report_id"`
	ChannelType   *string                   `json:"channel_type" validate:"omitempty,oneof=PRESSURE FLOW LEVEL RTU_READBACK"`
	EutManufacturer  *string                `json:"eut_manufacturer" validate:"omitempty,max=255"`
	EutModel         *string                `json:"eut_model" validate:"omitempty,max=255"`
	EutSerialNo      *string                `json:"eut_serial_no" validate:"omitempty,max=255"`
	EutInputRange    *string                `json:"eut_input_range" validate:"omitempty,max=100"`
	EutAccuracyClass *string                `json:"eut_accuracy_class" validate:"omitempty,max=100"`
	EutPowerSupply   *string                `json:"eut_power_supply" validate:"omitempty,max=100"`
	EutOutputRange   *string                `json:"eut_output_range" validate:"omitempty,max=100"`
	ResultType       *string                `json:"result_type" validate:"omitempty,oneof=TESTED CALIBRATED_AND_TESTED OTHER"`
	ResultOtherText  *string                `json:"result_other_text" validate:"omitempty,max=255"`
	Readings      []CalibrationReadingInput `json:"readings" validate:"omitempty,max=500,dive"`
}

// CalibrationUpdateInput is the PATCH /calibrations/{id} body.
type CalibrationUpdateInput struct {
	PanelDeviceID    *uuid.UUID `json:"panel_device_id"`
	InstrumentID     *uuid.UUID `json:"instrument_id"`
	PerformedBy      *string    `json:"performed_by" validate:"omitempty,max=100"`
	PerformedAt      *time.Time `json:"performed_at"`
	Result           *string    `json:"result" validate:"omitempty,oneof=PASS FAIL ADJUSTED"`
	Remark           *string    `json:"remark" validate:"omitempty,max=4000"`
	WorkOrderID      *uuid.UUID `json:"work_order_id"`
	PmReportID       *uuid.UUID `json:"pm_report_id"`
	ChannelType      *string    `json:"channel_type" validate:"omitempty,oneof=PRESSURE FLOW LEVEL RTU_READBACK"`
	EutManufacturer  *string    `json:"eut_manufacturer" validate:"omitempty,max=255"`
	EutModel         *string    `json:"eut_model" validate:"omitempty,max=255"`
	EutSerialNo      *string    `json:"eut_serial_no" validate:"omitempty,max=255"`
	EutInputRange    *string    `json:"eut_input_range" validate:"omitempty,max=100"`
	EutAccuracyClass *string    `json:"eut_accuracy_class" validate:"omitempty,max=100"`
	EutPowerSupply   *string    `json:"eut_power_supply" validate:"omitempty,max=100"`
	EutOutputRange   *string    `json:"eut_output_range" validate:"omitempty,max=100"`
	ResultType       *string    `json:"result_type" validate:"omitempty,oneof=TESTED CALIBRATED_AND_TESTED OTHER"`
	ResultOtherText  *string    `json:"result_other_text" validate:"omitempty,max=255"`
}

// ReadingSheetInput is the PUT /calibrations/{id}/readings body, which replaces
// the whole sheet at once.
type ReadingSheetInput struct {
	Readings []CalibrationReadingInput `json:"readings" validate:"max=500,dive"`
}

// ReadingUpdateInput is the PATCH /calibration-readings/{id} body.
type ReadingUpdateInput struct {
	Sequence     *int16           `json:"sequence" validate:"omitempty,gte=1"`
	ItemLabel    *string          `json:"item_label" validate:"omitempty,max=150"`
	ParameterKey *string          `json:"parameter_key" validate:"omitempty,max=50"`
	Value        *decimal.Decimal `json:"value"`
	Unit         *string          `json:"unit" validate:"omitempty,max=20"`
}

// List returns one page of calibrations.
func (s *CalibrationService) List(ctx context.Context, page httpx.Page, filter repository.CalibrationFilter) ([]repository.CalibrationView, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a calibration together with its readings.
func (s *CalibrationService) Get(ctx context.Context, id uuid.UUID) (repository.CalibrationDetail, error) {
	view, err := s.repo.GetView(ctx, id)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}

	readings, err := s.readings.ListByCalibration(ctx, id)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}

	return repository.CalibrationDetail{CalibrationView: view, Readings: readings}, nil
}

// Create records a calibration event and, when supplied, its readings.
func (s *CalibrationService) Create(ctx context.Context, in CalibrationCreateInput) (repository.CalibrationDetail, error) {
	if err := s.checkPerformedAt(in.PerformedAt); err != nil {
		return repository.CalibrationDetail{}, err
	}
	if err := s.checkDeviceUsable(ctx, in.PanelDeviceID); err != nil {
		return repository.CalibrationDetail{}, err
	}
	if err := s.checkInstrumentUsable(ctx, in.InstrumentID, in.PerformedAt); err != nil {
		return repository.CalibrationDetail{}, err
	}
	if err := s.checkPmLink(ctx, in.WorkOrderID, in.PmReportID); err != nil {
		return repository.CalibrationDetail{}, err
	}

	readings, err := buildReadingRows(in.Readings, 0)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}

	calibration, saved, err := s.repo.CreateWithReadings(ctx, sqlc.CreateCalibrationParams{
		PanelDeviceID:    in.PanelDeviceID,
		InstrumentID:     in.InstrumentID,
		PerformedBy:      in.PerformedBy,
		PerformedAt:      in.PerformedAt,
		Result:           in.Result,
		Remark:           in.Remark,
		WorkOrderID:      in.WorkOrderID,
		PmReportID:       in.PmReportID,
		ChannelType:      in.ChannelType,
		EutManufacturer:  in.EutManufacturer,
		EutModel:         in.EutModel,
		EutSerialNo:      in.EutSerialNo,
		EutInputRange:    in.EutInputRange,
		EutAccuracyClass: in.EutAccuracyClass,
		EutPowerSupply:   in.EutPowerSupply,
		EutOutputRange:   in.EutOutputRange,
		ResultType:       in.ResultType,
		ResultOtherText:  in.ResultOtherText,
	}, readings)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}

	view, err := s.repo.GetView(ctx, calibration.ID)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}
	return repository.CalibrationDetail{CalibrationView: view, Readings: saved}, nil
}

// Update applies a partial update to a calibration event.
func (s *CalibrationService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in CalibrationUpdateInput) (repository.CalibrationDetail, error) {
	params := sqlc.UpdateCalibrationParams{ID: id}

	deviceID, setDevice, err := patchRequired(fields, "panel_device_id", in.PanelDeviceID)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}
	params.PanelDeviceID, params.PanelDeviceIDDoUpdate = deviceID, setDevice

	instrumentID, setInstrument, err := patchRequired(fields, "instrument_id", in.InstrumentID)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}
	params.InstrumentID, params.InstrumentIDDoUpdate = instrumentID, setInstrument

	performedAt, setPerformedAt, err := patchRequired(fields, "performed_at", in.PerformedAt)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}
	params.PerformedAt, params.PerformedAtDoUpdate = performedAt, setPerformedAt

	result, setResult, err := patchRequired(fields, "result", in.Result)
	if err != nil {
		return repository.CalibrationDetail{}, err
	}
	params.Result, params.ResultDoUpdate = result, setResult

	params.PerformedBy, params.PerformedByDoUpdate = patchNullable(fields, "performed_by", in.PerformedBy)
	params.Remark, params.RemarkDoUpdate = patchNullable(fields, "remark", in.Remark)
	params.WorkOrderID, params.WorkOrderIDDoUpdate = patchNullable(fields, "work_order_id", in.WorkOrderID)
	params.PmReportID, params.PmReportIDDoUpdate = patchNullable(fields, "pm_report_id", in.PmReportID)
	params.ChannelType, params.ChannelTypeDoUpdate = patchNullable(fields, "channel_type", in.ChannelType)
	params.EutManufacturer, params.EutManufacturerDoUpdate = patchNullable(fields, "eut_manufacturer", in.EutManufacturer)
	params.EutModel, params.EutModelDoUpdate = patchNullable(fields, "eut_model", in.EutModel)
	params.EutSerialNo, params.EutSerialNoDoUpdate = patchNullable(fields, "eut_serial_no", in.EutSerialNo)
	params.EutInputRange, params.EutInputRangeDoUpdate = patchNullable(fields, "eut_input_range", in.EutInputRange)
	params.EutAccuracyClass, params.EutAccuracyClassDoUpdate = patchNullable(fields, "eut_accuracy_class", in.EutAccuracyClass)
	params.EutPowerSupply, params.EutPowerSupplyDoUpdate = patchNullable(fields, "eut_power_supply", in.EutPowerSupply)
	params.EutOutputRange, params.EutOutputRangeDoUpdate = patchNullable(fields, "eut_output_range", in.EutOutputRange)
	params.ResultType, params.ResultTypeDoUpdate = patchNullable(fields, "result_type", in.ResultType)
	params.ResultOtherText, params.ResultOtherTextDoUpdate = patchNullable(fields, "result_other_text", in.ResultOtherText)

	if params.WorkOrderIDDoUpdate || params.PmReportIDDoUpdate {
		current, err := s.repo.Get(ctx, id)
		if err != nil {
			return repository.CalibrationDetail{}, err
		}
		woID, pmID := current.WorkOrderID, current.PmReportID
		if params.WorkOrderIDDoUpdate {
			woID = params.WorkOrderID
		}
		if params.PmReportIDDoUpdate {
			pmID = params.PmReportID
		}
		if err := s.checkPmLink(ctx, woID, pmID); err != nil {
			return repository.CalibrationDetail{}, err
		}
	}

	if setPerformedAt {
		if err := s.checkPerformedAt(performedAt); err != nil {
			return repository.CalibrationDetail{}, err
		}
	}
	if setDevice {
		if err := s.checkDeviceUsable(ctx, deviceID); err != nil {
			return repository.CalibrationDetail{}, err
		}
	}
	if setPerformedAt || setInstrument {
		current, err := s.repo.Get(ctx, id)
		if err != nil {
			return repository.CalibrationDetail{}, err
		}
		effectiveInstrument := current.InstrumentID
		if setInstrument {
			effectiveInstrument = instrumentID
		}
		effectiveAt := current.PerformedAt
		if setPerformedAt {
			effectiveAt = performedAt
		}
		if err := s.checkInstrumentUsable(ctx, effectiveInstrument, effectiveAt); err != nil {
			return repository.CalibrationDetail{}, err
		}
	}

	if _, err := s.repo.Update(ctx, params); err != nil {
		return repository.CalibrationDetail{}, err
	}
	return s.Get(ctx, id)
}

// Delete removes a calibration; PostgreSQL cascades its readings.
func (s *CalibrationService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ResultSummary counts calibrations per result over an optional device and
// period.
func (s *CalibrationService) ResultSummary(ctx context.Context, deviceID *uuid.UUID, from, to *time.Time) (map[string]int64, error) {
	if from != nil && to != nil && to.Before(*from) {
		return nil, httpx.Err(httpx.ErrDateRangeInvalid).
			WithField("performed_to", httpx.IssueInvalid, "Must be later than performed_from.")
	}

	rows, err := s.repo.ResultSummary(ctx, sqlc.CountCalibrationsByResultParams{
		PanelDeviceID: deviceID,
		PerformedFrom: from,
		PerformedTo:   to,
	})
	if err != nil {
		return nil, err
	}

	summary := map[string]int64{"PASS": 0, "FAIL": 0, "ADJUSTED": 0}
	for _, row := range rows {
		summary[row.Result] = row.Total
	}
	return summary, nil
}

// ListReadings returns the measurement sheet of a calibration.
func (s *CalibrationService) ListReadings(ctx context.Context, calibrationID uuid.UUID) ([]sqlc.CalibrationReading, error) {
	if _, err := s.repo.Get(ctx, calibrationID); err != nil {
		return nil, err
	}
	return s.readings.ListByCalibration(ctx, calibrationID)
}

// AddReading appends one reading, assigning the next free sequence when the
// client does not choose one.
func (s *CalibrationService) AddReading(ctx context.Context, calibrationID uuid.UUID, in CalibrationReadingInput) (sqlc.CalibrationReading, error) {
	if _, err := s.repo.Get(ctx, calibrationID); err != nil {
		return sqlc.CalibrationReading{}, err
	}

	sequence := int16(0)
	if in.Sequence != nil {
		sequence = *in.Sequence
	} else {
		next, err := s.readings.NextSequence(ctx, calibrationID)
		if err != nil {
			return sqlc.CalibrationReading{}, err
		}
		sequence = next
	}

	return s.readings.Create(ctx, sqlc.CreateCalibrationReadingParams{
		CalibrationID: calibrationID,
		Sequence:      sequence,
		ItemLabel:     in.ItemLabel,
		ParameterKey:  in.ParameterKey,
		Value:         in.Value,
		Unit:          in.Unit,
	})
}

// ReplaceReadings swaps the whole measurement sheet of a calibration.
func (s *CalibrationService) ReplaceReadings(ctx context.Context, calibrationID uuid.UUID, in ReadingSheetInput) ([]sqlc.CalibrationReading, error) {
	rows, err := buildReadingRows(in.Readings, 0)
	if err != nil {
		return nil, err
	}
	return s.repo.ReplaceReadings(ctx, calibrationID, rows)
}

// GetReading returns a single reading.
func (s *CalibrationService) GetReading(ctx context.Context, id uuid.UUID) (sqlc.CalibrationReading, error) {
	return s.readings.Get(ctx, id)
}

// GetReadingForCalibration returns a reading under a specific calibration.
func (s *CalibrationService) GetReadingForCalibration(ctx context.Context, calibrationID, id uuid.UUID) (sqlc.CalibrationReading, error) {
	if _, err := s.repo.Get(ctx, calibrationID); err != nil {
		return sqlc.CalibrationReading{}, err
	}
	return s.readings.GetForCalibration(ctx, calibrationID, id)
}

// UpdateReadingForCalibration updates a reading scoped to its parent calibration.
func (s *CalibrationService) UpdateReadingForCalibration(ctx context.Context, calibrationID, id uuid.UUID, fields httpx.FieldSet, in ReadingUpdateInput) (sqlc.CalibrationReading, error) {
	if _, err := s.GetReadingForCalibration(ctx, calibrationID, id); err != nil {
		return sqlc.CalibrationReading{}, err
	}
	return s.UpdateReading(ctx, id, fields, in)
}

// DeleteReadingForCalibration removes a reading scoped to its parent calibration.
func (s *CalibrationService) DeleteReadingForCalibration(ctx context.Context, calibrationID, id uuid.UUID) error {
	if _, err := s.GetReadingForCalibration(ctx, calibrationID, id); err != nil {
		return err
	}
	return s.DeleteReading(ctx, id)
}

// UpdateReading applies a partial update to a reading.
func (s *CalibrationService) UpdateReading(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in ReadingUpdateInput) (sqlc.CalibrationReading, error) {
	params := sqlc.UpdateCalibrationReadingParams{ID: id}

	sequence, setSequence, err := patchRequired(fields, "sequence", in.Sequence)
	if err != nil {
		return sqlc.CalibrationReading{}, err
	}
	params.Sequence, params.SequenceDoUpdate = sequence, setSequence

	parameterKey, setKey, err := patchRequired(fields, "parameter_key", in.ParameterKey)
	if err != nil {
		return sqlc.CalibrationReading{}, err
	}
	params.ParameterKey, params.ParameterKeyDoUpdate = parameterKey, setKey

	params.ItemLabel, params.ItemLabelDoUpdate = patchNullable(fields, "item_label", in.ItemLabel)
	params.Value, params.ValueDoUpdate = patchNullable(fields, "value", in.Value)
	params.Unit, params.UnitDoUpdate = patchNullable(fields, "unit", in.Unit)

	return s.readings.Update(ctx, params)
}

// DeleteReading removes a single reading.
func (s *CalibrationService) DeleteReading(ctx context.Context, id uuid.UUID) error {
	return s.readings.Delete(ctx, id)
}

func (s *CalibrationService) checkPerformedAt(performedAt time.Time) error {
	if performedAt.After(time.Now().Add(clockSkew)) {
		return httpx.Err(httpx.ErrPerformedAtFuture).
			WithField("performed_at", httpx.IssueInvalid, "Must not be in the future.")
	}
	return nil
}

func (s *CalibrationService) checkDeviceUsable(ctx context.Context, deviceID uuid.UUID) error {
	active, err := s.devices.IsActive(ctx, deviceID)
	if err != nil {
		return err
	}
	if !active {
		return httpx.Err(httpx.ErrDeviceInactive).
			WithField("panel_device_id", httpx.IssueInvalid, "The device is deactivated and cannot be calibrated.")
	}
	return nil
}

func (s *CalibrationService) checkInstrumentUsable(ctx context.Context, instrumentID uuid.UUID, performedAt time.Time) error {
	active, expireDate, err := s.instruments.Usability(ctx, instrumentID)
	if err != nil {
		return err
	}
	if !active {
		return httpx.Err(httpx.ErrInstrumentInactive).
			WithField("instrument_id", httpx.IssueInvalid, "The instrument is deactivated.")
	}
	if expireDate != nil {
		exp := expireDate.AsTime()
		// The certificate stays valid through the whole of its expiry date.
		validUntil := exp.AddDate(0, 0, 1)
		if !performedAt.UTC().Before(validUntil) {
			return httpx.Errf(httpx.ErrInstrumentExpired,
				"Calibration instrument certificate expired on %s.", exp.Format(time.DateOnly)).
				WithField("instrument_id", httpx.IssueInvalid, "The certificate had expired at performed_at.")
		}
	}
	return nil
}

// checkPmLink enforces that calibrations attached to a work order / PM report
// only ever belong to a SIX_MONTH PM (System Design: Calibration Section shows
// only for 6-month PM). Standalone calibrations (both nil) are still allowed.
func (s *CalibrationService) checkPmLink(ctx context.Context, workOrderID, pmReportID *uuid.UUID) error {
	if workOrderID == nil && pmReportID == nil {
		return nil
	}
	if s.workOrders == nil {
		return nil
	}

	var woID uuid.UUID
	switch {
	case workOrderID != nil && pmReportID != nil:
		if s.pmReports == nil {
			return nil
		}
		report, err := s.pmReports.GetDetail(ctx, *pmReportID)
		if err != nil {
			return err
		}
		if report.WorkOrderID != *workOrderID {
			return httpx.Err(httpx.ErrCalibrationLinkPmOnly).
				WithField("pm_report_id", httpx.IssueInvalid, "Must belong to the given work order.")
		}
		woID = *workOrderID
	case workOrderID != nil:
		woID = *workOrderID
	case pmReportID != nil && s.pmReports != nil:
		report, err := s.pmReports.GetDetail(ctx, *pmReportID)
		if err != nil {
			return err
		}
		woID = report.WorkOrderID
	default:
		return nil
	}

	wo, err := s.workOrders.Get(ctx, woID)
	if err != nil {
		return err
	}
	if wo.WorkOrderType != "PM" || wo.PmScheduleType == nil || *wo.PmScheduleType != "SIX_MONTH" {
		return httpx.Err(httpx.ErrCalibrationLinkPmOnly).
			WithField("work_order_id", httpx.IssueInvalid, "Must reference a 6-month PM work order.")
	}
	return nil
}

// buildReadingRows normalises a measurement sheet: sequences are either all
// supplied by the client or all assigned here, and they must be unique.
func buildReadingRows(inputs []CalibrationReadingInput, startAt int16) ([]sqlc.BulkCreateCalibrationReadingsParams, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) > maxReadingsPerCalibration {
		return nil, httpx.Errf(httpx.ErrValidationFailed,
			"A calibration accepts at most %d readings.", maxReadingsPerCalibration).
			WithField("readings", httpx.IssueOutOfRange,
				fmt.Sprintf("Received %d rows.", len(inputs)))
	}

	supplied := 0
	for _, in := range inputs {
		if in.Sequence != nil {
			supplied++
		}
	}
	if supplied != 0 && supplied != len(inputs) {
		return nil, httpx.Err(httpx.ErrValidationFailed).
			WithField("readings[].sequence", httpx.IssueInvalid,
				"Provide a sequence for every reading or for none of them.")
	}

	rows := make([]sqlc.BulkCreateCalibrationReadingsParams, 0, len(inputs))
	seen := make(map[int16]int, len(inputs))

	for i, in := range inputs {
		sequence := startAt + int16(i) + 1
		if in.Sequence != nil {
			sequence = *in.Sequence
		}
		if first, duplicate := seen[sequence]; duplicate {
			return nil, httpx.Errf(httpx.ErrReadingSeqDup,
				"Sequence %d is used by both reading %d and reading %d.", sequence, first, i).
				WithField(fmt.Sprintf("readings[%d].sequence", i), httpx.IssueDuplicate,
					"Sequence numbers must be unique within a calibration.")
		}
		seen[sequence] = i

		rows = append(rows, sqlc.BulkCreateCalibrationReadingsParams{
			Sequence:     sequence,
			ItemLabel:    in.ItemLabel,
			ParameterKey: in.ParameterKey,
			Value:        in.Value,
			Unit:         in.Unit,
		})
	}

	return rows, nil
}
