package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// PanelDeviceCmCloseOutcome syncs panel device health when a CM work order
// closes inside the same transaction as wo_approvals.DecideAndApply.
type PanelDeviceCmCloseOutcome struct {
	DeviceID    uuid.UUID
	FinalStatus string // COMPLETED or CONDITIONAL
}

func cmFinalHealthStatus(finalStatus string) (string, bool) {
	switch finalStatus {
	case "COMPLETED":
		return "NORMAL", true
	case "CONDITIONAL":
		return "WARNING", true
	default:
		return "", false
	}
}

func applyPanelDeviceHealthQ(
	ctx context.Context,
	q *sqlc.Queries,
	deviceID uuid.UUID,
	health string,
	updatedBy *uuid.UUID,
) error {
	_, err := q.UpdatePanelDeviceStatus(ctx, sqlc.UpdatePanelDeviceStatusParams{
		ID:                   deviceID,
		HealthStatus:         health,
		HealthStatusDoUpdate: true,
		UpdatedBy:            updatedBy,
	})
	return db.Translate(err, db.WithNotFound(httpx.ErrPanelDeviceNotFnd))
}

// SyncPanelDeviceCmOpenQ marks health WARNING inside the caller's transaction.
func SyncPanelDeviceCmOpenQ(ctx context.Context, q *sqlc.Queries, deviceID uuid.UUID, updatedBy *uuid.UUID) error {
	return applyPanelDeviceHealthQ(ctx, q, deviceID, "WARNING", updatedBy)
}

func hasOpenCmForDeviceTx(ctx context.Context, tx pgx.Tx, deviceID uuid.UUID) (bool, error) {
	var found bool
	if err := tx.QueryRow(ctx, hasOpenCmForDeviceSQL, deviceID).Scan(&found); err != nil {
		return false, db.Translate(err)
	}
	return found, nil
}

func effectivePanelDeviceIDTx(ctx context.Context, tx pgx.Tx, workOrderID uuid.UUID) (*uuid.UUID, error) {
	var deviceID *uuid.UUID
	if err := tx.QueryRow(ctx, effectivePanelDeviceForWorkOrderSQL, workOrderID).Scan(&deviceID); err != nil {
		return nil, db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderNotFnd))
	}
	return deviceID, nil
}

// SyncPanelDeviceAfterCmClosedQ updates device health after a CM work order
// status was persisted in the same transaction. communication_status is unchanged.
func SyncPanelDeviceAfterCmClosedQ(
	ctx context.Context,
	tx pgx.Tx,
	q *sqlc.Queries,
	deviceID uuid.UUID,
	finalStatus string,
	updatedBy *uuid.UUID,
) error {
	health, ok := cmFinalHealthStatus(finalStatus)
	if !ok {
		return nil
	}
	open, err := hasOpenCmForDeviceTx(ctx, tx, deviceID)
	if err != nil {
		return err
	}
	if open {
		return SyncPanelDeviceCmOpenQ(ctx, q, deviceID, updatedBy)
	}
	return applyPanelDeviceHealthQ(ctx, q, deviceID, health, updatedBy)
}

// RecalcPanelDeviceCmStatusQ re-derives health from open CM coverage in-tx.
// Only CM-driven WARNING is cleared to NORMAL when no open CM remains.
func RecalcPanelDeviceCmStatusQ(
	ctx context.Context,
	tx pgx.Tx,
	q *sqlc.Queries,
	deviceID uuid.UUID,
	updatedBy *uuid.UUID,
) error {
	open, err := hasOpenCmForDeviceTx(ctx, tx, deviceID)
	if err != nil {
		return err
	}
	if open {
		return SyncPanelDeviceCmOpenQ(ctx, q, deviceID, updatedBy)
	}
	device, err := q.GetPanelDevice(ctx, deviceID)
	if err != nil {
		return db.Translate(err, db.WithNotFound(httpx.ErrPanelDeviceNotFnd))
	}
	if device.HealthStatus != "WARNING" {
		return nil
	}
	return applyPanelDeviceHealthQ(ctx, q, deviceID, "NORMAL", updatedBy)
}

// SyncPanelDeviceCmReportSaveQ syncs device health after a cm_report save inside
// the panel CM lock transaction.
func SyncPanelDeviceCmReportSaveQ(
	ctx context.Context,
	tx pgx.Tx,
	q *sqlc.Queries,
	openCm bool,
	previousDeviceID *uuid.UUID,
	newDeviceID *uuid.UUID,
) error {
	if !openCm {
		return nil
	}
	updatedBy := updateAudit(ctx)
	if previousDeviceID != nil && (newDeviceID == nil || *previousDeviceID != *newDeviceID) {
		if err := RecalcPanelDeviceCmStatusQ(ctx, tx, q, *previousDeviceID, updatedBy); err != nil {
			return err
		}
	}
	if newDeviceID != nil {
		return SyncPanelDeviceCmOpenQ(ctx, q, *newDeviceID, updatedBy)
	}
	return nil
}

// RecalcPanelDeviceCmStatusForWorkOrderQ recalculates using the effective device id.
func RecalcPanelDeviceCmStatusForWorkOrderQ(
	ctx context.Context,
	tx pgx.Tx,
	q *sqlc.Queries,
	workOrderID uuid.UUID,
	updatedBy *uuid.UUID,
) error {
	deviceID, err := effectivePanelDeviceIDTx(ctx, tx, workOrderID)
	if err != nil || deviceID == nil {
		return err
	}
	return RecalcPanelDeviceCmStatusQ(ctx, tx, q, *deviceID, updatedBy)
}
