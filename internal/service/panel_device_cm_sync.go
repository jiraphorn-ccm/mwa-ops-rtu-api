package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/repository"
)

// openCmWorkOrderStatuses are CM work orders still in flight (not closed).
var openCmWorkOrderStatuses = map[string]struct{}{
	"ASSIGNED":         {},
	"IN_PROGRESS":      {},
	"PENDING":          {},
	"PENDING_APPROVAL": {},
}

func isOpenCmWorkOrder(woType, status string) bool {
	if woType != "CM" {
		return false
	}
	_, ok := openCmWorkOrderStatuses[status]
	return ok
}

// cmClosedHealthStatus maps a closed CM work order status to the health_status
// written on the panel device when no other open CM covers it.
func cmClosedHealthStatus(finalStatus string) (string, bool) {
	switch finalStatus {
	case "COMPLETED":
		return "NORMAL", true
	case "CONDITIONAL":
		return "WARNING", true
	default:
		return "", false
	}
}

// shouldClearCmHealthOnRecalc reports whether recalc may reset health to NORMAL
// when no open CM remains. Only CM-driven WARNING is cleared; CRITICAL and other
// states are left to telemetry / manual updates.
func shouldClearCmHealthOnRecalc(currentHealth string) bool {
	return currentHealth == "WARNING"
}

// RunPanelDeviceCmSync runs a sync side effect and logs failures without
// failing the caller's primary operation (CM workflow response stays success).
func RunPanelDeviceCmSync(ctx context.Context, op string, fn func() error, attrs ...any) {
	if err := fn(); err != nil {
		args := append([]any{"op", op, "error", err}, attrs...)
		slog.WarnContext(ctx, "panel device CM status sync failed", args...)
	}
}

func RunSyncPanelDeviceForCmOpen(ctx context.Context, devices *repository.PanelDeviceRepository, deviceID uuid.UUID) {
	RunPanelDeviceCmSync(ctx, "cm_open", func() error {
		return SyncPanelDeviceForCmOpen(ctx, devices, deviceID)
	}, "panel_device_id", deviceID)
}

func RunSyncPanelDeviceAfterCmClosed(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	deviceID uuid.UUID,
	finalStatus string,
) {
	RunPanelDeviceCmSync(ctx, "cm_closed", func() error {
		return SyncPanelDeviceAfterCmClosed(ctx, devices, workOrders, deviceID, finalStatus)
	}, "panel_device_id", deviceID, "final_status", finalStatus)
}

func RunSyncPanelDeviceAfterCmWorkOrderClosed(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	workOrderID uuid.UUID,
	finalStatus string,
) {
	RunPanelDeviceCmSync(ctx, "cm_work_order_closed", func() error {
		return SyncPanelDeviceAfterCmWorkOrderClosed(ctx, devices, workOrders, workOrderID, finalStatus)
	}, "work_order_id", workOrderID, "final_status", finalStatus)
}

func RunRecalcPanelDeviceCmStatus(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	deviceID uuid.UUID,
) {
	RunPanelDeviceCmSync(ctx, "cm_recalc", func() error {
		return RecalcPanelDeviceCmStatus(ctx, devices, workOrders, deviceID)
	}, "panel_device_id", deviceID)
}

func RunRecalcPanelDeviceCmStatusForWorkOrder(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	workOrderID uuid.UUID,
) {
	RunPanelDeviceCmSync(ctx, "cm_work_order_recalc", func() error {
		return RecalcPanelDeviceCmStatusForWorkOrder(ctx, devices, workOrders, workOrderID)
	}, "work_order_id", workOrderID)
}

// SyncPanelDeviceForCmOpen marks a device MONITORING (health WARNING) when a
// CM work order covering it is opened. communication_status is not changed.
func SyncPanelDeviceForCmOpen(ctx context.Context, devices *repository.PanelDeviceRepository, deviceID uuid.UUID) error {
	if devices == nil {
		return nil
	}
	_, err := devices.UpdateStatus(ctx, sqlc.UpdatePanelDeviceStatusParams{
		ID:                   deviceID,
		HealthStatus:         "WARNING",
		HealthStatusDoUpdate: true,
	})
	return err
}

// SyncPanelDeviceAfterCmClosed updates health_status when a CM work order is
// approved to COMPLETED or CONDITIONAL. If another open CM still covers the
// device it stays MONITORING; otherwise COMPLETED → health NORMAL and
// CONDITIONAL → health WARNING. communication_status is never changed here.
func SyncPanelDeviceAfterCmClosed(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	deviceID uuid.UUID,
	finalStatus string,
) error {
	if devices == nil || workOrders == nil {
		return nil
	}
	health, ok := cmClosedHealthStatus(finalStatus)
	if !ok {
		return nil
	}

	open, err := workOrders.HasOpenCmForDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if open {
		return SyncPanelDeviceForCmOpen(ctx, devices, deviceID)
	}

	_, err = devices.UpdateStatus(ctx, sqlc.UpdatePanelDeviceStatusParams{
		ID:                   deviceID,
		HealthStatus:         health,
		HealthStatusDoUpdate: true,
	})
	return err
}

// SyncPanelDeviceAfterCmWorkOrderClosed resolves the effective panel_device_id
// for a CM work order (cm_report overrides work_order) then syncs device health.
func SyncPanelDeviceAfterCmWorkOrderClosed(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	workOrderID uuid.UUID,
	finalStatus string,
) error {
	if devices == nil || workOrders == nil {
		return nil
	}
	deviceID, err := workOrders.EffectivePanelDeviceID(ctx, workOrderID)
	if err != nil || deviceID == nil {
		return err
	}
	return SyncPanelDeviceAfterCmClosed(ctx, devices, workOrders, *deviceID, finalStatus)
}

// RecalcPanelDeviceCmStatus re-derives device health from open CM coverage.
// When no open CM remains, only CM-driven WARNING is cleared to NORMAL.
func RecalcPanelDeviceCmStatus(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	deviceID uuid.UUID,
) error {
	if devices == nil || workOrders == nil {
		return nil
	}
	open, err := workOrders.HasOpenCmForDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if open {
		return SyncPanelDeviceForCmOpen(ctx, devices, deviceID)
	}

	device, err := devices.Get(ctx, deviceID)
	if err != nil {
		return err
	}
	if !shouldClearCmHealthOnRecalc(device.HealthStatus) {
		return nil
	}

	_, err = devices.UpdateStatus(ctx, sqlc.UpdatePanelDeviceStatusParams{
		ID:                   deviceID,
		HealthStatus:         "NORMAL",
		HealthStatusDoUpdate: true,
	})
	return err
}

// RecalcPanelDeviceCmStatusForWorkOrder recalculates health for the effective
// device linked to a CM work order.
func RecalcPanelDeviceCmStatusForWorkOrder(
	ctx context.Context,
	devices *repository.PanelDeviceRepository,
	workOrders *repository.WorkOrderRepository,
	workOrderID uuid.UUID,
) error {
	if devices == nil || workOrders == nil {
		return nil
	}
	deviceID, err := workOrders.EffectivePanelDeviceID(ctx, workOrderID)
	if err != nil || deviceID == nil {
		return err
	}
	return RecalcPanelDeviceCmStatus(ctx, devices, workOrders, *deviceID)
}
