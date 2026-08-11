// Package service holds the business rules that sit between the HTTP handlers
// and the repositories: request shapes, cross-entity checks and the
// orchestration of multi-statement writes.
package service

import (
	"github.com/rtu-api/internal/repository"
	"github.com/rtu-api/internal/storage"
)

// Services bundles every service behind a single dependency.
type Services struct {
	Panels                 *PanelService
	DeviceModels           *DeviceModelService
	PanelDevices           *PanelDeviceService
	CalibrationInstruments *CalibrationInstrumentService
	Calibrations           *CalibrationService
	PanelImages            *PanelImageService
	WorkOrders             *WorkOrderService
	Approvals              *ApprovalService
	Engineers              *EngineerService
	ChecklistItems         *ChecklistItemService
	PmReports              *PmReportService
	CmReports              *CmReportService
	Attachments            *AttachmentService
	Notifications          *NotificationService
}

// New wires the services onto the repository store.
func New(store *repository.Store, s3 *storage.S3Client, appPrefix string) *Services {
	workOrders := &WorkOrderService{
		repo:     store.WorkOrders,
		rounds:   store.WorkOrderRounds,
		activity: store.WorkOrderActivityLogs,
		panels:   store.Panels,
		devices:  store.PanelDevices,
	}
	notifications := &NotificationService{
		repo:       store.Notifications,
		workOrders: store.WorkOrders,
	}
	workOrders.notify = notifications

	return &Services{
		Panels:                 &PanelService{repo: store.Panels},
		DeviceModels:           &DeviceModelService{repo: store.DeviceModels},
		PanelDevices:           &PanelDeviceService{repo: store.PanelDevices, panels: store.Panels, models: store.DeviceModels},
		CalibrationInstruments: &CalibrationInstrumentService{repo: store.CalibrationInstruments},
		Calibrations: &CalibrationService{
			repo:        store.Calibrations,
			readings:    store.CalibrationReadings,
			devices:     store.PanelDevices,
			instruments: store.CalibrationInstruments,
			workOrders:  workOrders,
			pmReports:   store.PmReports,
		},
		PanelImages: &PanelImageService{
			repo:      store.PanelImages,
			panels:    store.Panels,
			s3:        s3,
			appPrefix: appPrefix,
		},
		WorkOrders: workOrders,
		Approvals: &ApprovalService{
			repo:        store.WoApprovals,
			workOrders:  workOrders,
			activityLog: store.WorkOrderActivityLogs,
			panels:      store.Panels,
			notify:      notifications,
		},
		Engineers:      &EngineerService{repo: store.Engineers},
		ChecklistItems: &ChecklistItemService{repo: store.ChecklistItems},
		PmReports: &PmReportService{
			repo:       store.PmReports,
			workOrders: workOrders,
			devices:    store.PanelDevices,
			notify:     notifications,
		},
		CmReports: &CmReportService{
			repo:       store.CmReports,
			workOrders: workOrders,
			pmReports:  store.PmReports,
			devices:    store.PanelDevices,
			activity:   store.WorkOrderActivityLogs,
			notify:     notifications,
		},
		Attachments: &AttachmentService{
			repo:      store.Attachments,
			s3:        s3,
			appPrefix: appPrefix,
		},
		Notifications: notifications,
	}
}

// Enumerations accepted by the API. They mirror the CHECK constraints of the
// schema, so a bad value is rejected before it reaches PostgreSQL.
const (
	CommunicationStatuses = "ONLINE OFFLINE DEGRADED UNKNOWN"
	HealthStatuses        = "NORMAL WARNING CRITICAL UNKNOWN"
	CalibrationResults    = "PASS FAIL ADJUSTED"
	WorkOrderTypes        = "PM CM"
	PmScheduleTypes       = "THREE_MONTH SIX_MONTH"
	WorkOrderStatuses     = "ASSIGNED IN_PROGRESS PENDING PENDING_APPROVAL COMPLETED CONDITIONAL CANCELLED"
	WorkOrderPriorities   = "HIGH MEDIUM LOW"
)
