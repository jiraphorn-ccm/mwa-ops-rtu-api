package service

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// List filter types are owned by the service layer so HTTP handlers never
// import repository directly.
type (
	PanelListFilter                 = repository.PanelFilter
	DeviceModelListFilter           = repository.DeviceModelFilter
	PanelDeviceListFilter           = repository.PanelDeviceFilter
	CalibrationInstrumentListFilter = repository.CalibrationInstrumentFilter
	CalibrationListFilter           = repository.CalibrationFilter
	PanelImageListFilter            = repository.PanelImageFilter
	WorkOrderListFilter             = repository.WorkOrderFilter
	EngineerListFilter              = repository.EngineerFilter
	NotificationListFilter          = repository.NotificationFilter
)

// Sortable keys for list endpoints.
var (
	PanelSortable                 = repository.PanelSortable
	DeviceModelSortable           = repository.DeviceModelSortable
	PanelDeviceSortable           = repository.PanelDeviceSortable
	CalibrationInstrumentSortable = repository.CalibrationInstrumentSortable
	CalibrationSortable           = repository.CalibrationSortable
	PanelImageSortable            = repository.PanelImageSortable
	WorkOrderSortable             = repository.WorkOrderSortable
	EngineerSortable              = repository.EngineerSortable
	PmReportHistorySortable       = repository.PmReportHistorySortable
	CmReportHistorySortable       = repository.CmReportHistorySortable
	NotificationSortable          = repository.NotificationSortable
)

// ParsePanelList reads pagination and filter query params for GET /panels.
func ParsePanelList(r *http.Request) (httpx.Page, PanelListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, PanelSortable(), "created_at")
	filter := PanelListFilter{
		Active:      q.Bool("active"),
		HasLocation: q.Bool("has_location"),
		CreatedFrom: q.Time("created_from"),
		CreatedTo:   q.Time("created_to"),
	}
	return page, filter, q.Err()
}

// ParseDeviceModelList reads query params for GET /device-models.
func ParseDeviceModelList(r *http.Request) (httpx.Page, DeviceModelListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, DeviceModelSortable(), "code")
	filter := DeviceModelListFilter{
		Active:        q.Bool("active"),
		Manufacturer:  q.String("manufacturer"),
		EquipmentType: q.String("equipment_type"),
		Brand:         q.String("brand"),
	}
	return page, filter, q.Err()
}

// ParsePanelDeviceList reads query params for GET /panel-devices.
func ParsePanelDeviceList(r *http.Request, panelID *uuid.UUID) (httpx.Page, PanelDeviceListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, PanelDeviceSortable(), "created_at")
	filter := PanelDeviceListFilter{
		PanelID:             panelID,
		EquipmentType:       q.String("equipment_type"),
		Manufacturer:        q.String("manufacturer"),
		Brand:               q.String("brand"),
		Active:              q.Bool("active"),
		CommunicationStatus: q.Enum("communication_status", "ONLINE", "OFFLINE", "DEGRADED", "UNKNOWN"),
		HealthStatus:        q.Enum("health_status", "NORMAL", "WARNING", "CRITICAL", "UNKNOWN"),
		InstalledFrom:       q.Time("installed_from"),
		InstalledTo:         q.Time("installed_to"),
		LastSeenFrom:        q.Time("last_seen_from"),
		LastSeenTo:          q.Time("last_seen_to"),
		NeverSeen:           q.Bool("never_seen"),
	}
	if panelID == nil {
		filter.PanelID = q.UUID("panel_id")
	}
	return page, filter, q.Err()
}

// ParseCalibrationInstrumentList reads query params for GET /calibration-instruments.
func ParseCalibrationInstrumentList(r *http.Request) (httpx.Page, CalibrationInstrumentListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, CalibrationInstrumentSortable(), "name")
	filter := CalibrationInstrumentListFilter{
		Active:         q.Bool("active"),
		Manufacturer:   q.String("manufacturer"),
		EquipmentType:  q.String("equipment_type"),
		Brand:          q.String("brand"),
		Expired:        q.Bool("expired"),
		ExpiringBefore: q.Time("expiring_before"),
	}
	return page, filter, q.Err()
}

// ParseCalibrationList reads query params for GET /calibrations.
func ParseCalibrationList(r *http.Request, deviceID *uuid.UUID) (httpx.Page, CalibrationListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, CalibrationSortable(), "performed_at")
	filter := CalibrationListFilter{
		PanelDeviceID: deviceID,
		PanelID:       q.UUID("panel_id"),
		EquipmentType: q.String("equipment_type"),
		InstrumentID:  q.UUID("instrument_id"),
		Result:        q.Enum("result", "PASS", "FAIL", "ADJUSTED"),
		PerformedBy:   q.String("performed_by"),
		PerformedFrom: q.Time("performed_from"),
		PerformedTo:   q.Time("performed_to"),
	}
	if deviceID == nil {
		filter.PanelDeviceID = q.UUID("panel_device_id")
	}
	return page, filter, q.Err()
}

// ParsePanelImageList reads query params for GET /panels/{id}/images.
func ParsePanelImageList(r *http.Request, panelID uuid.UUID) (httpx.Page, PanelImageListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, PanelImageSortable(), "sort_order")
	filter := PanelImageListFilter{PanelID: panelID}
	if t := q.Enum("image_type", "EXTERIOR", "INTERIOR", "DEVICE"); t != nil {
		filter.ImageType = t
	}
	return page, filter, q.Err()
}

// ParseWorkOrderList reads query params for GET /work-orders.
func ParseWorkOrderList(r *http.Request, panelID, panelDeviceID *uuid.UUID) (httpx.Page, WorkOrderListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, WorkOrderSortable(), "created_at")
	filter := WorkOrderListFilter{
		PanelID:       panelID,
		PanelDeviceID: panelDeviceID,
		WorkOrderType: q.Enum("work_order_type", "PM", "CM"),
		Status:        q.Enum("status", "ASSIGNED", "IN_PROGRESS", "PENDING", "PENDING_APPROVAL", "COMPLETED", "CONDITIONAL", "CANCELLED"),
		Priority:      q.Enum("priority", "HIGH", "MEDIUM", "LOW"),
		Active:        q.Bool("active"),
		AssignedTo:    q.UUID("assigned_to"),
		PlannedFrom:   q.Time("planned_from"),
		PlannedTo:     q.Time("planned_to"),
		DueFrom:       q.Time("due_from"),
		DueTo:         q.Time("due_to"),
	}
	if t := q.Enum("pm_schedule_type", "THREE_MONTH", "SIX_MONTH"); t != nil {
		filter.PmScheduleType = t
	}
	if panelID == nil {
		filter.PanelID = q.UUID("panel_id")
	}
	if panelDeviceID == nil {
		filter.PanelDeviceID = q.UUID("panel_device_id")
	}
	return page, filter, q.Err()
}

// ParseNotificationList reads query params for GET /notifications.
func ParseNotificationList(r *http.Request, recipientID uuid.UUID) (httpx.Page, NotificationListFilter, error) {
	q := httpx.NewQuery(r)
	page := httpx.ParsePage(q, NotificationSortable(), "created_at")
	filter := NotificationListFilter{
		RecipientID: recipientID,
		IsRead:      q.Bool("is_read"),
		Type:        q.Enum("type", "NEW_ASSIGNMENT", "PENDING_WORK", "PENDING_APPROVAL", "COMPLETED", "CM_PENDING"),
	}
	return page, filter, q.Err()
}
