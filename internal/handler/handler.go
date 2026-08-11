// Package handler adapts HTTP requests to the service layer and renders every
// answer through the shared response envelope.
package handler

import (
	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/service"
)

// Handlers bundles every HTTP handler behind a single dependency.
type Handlers struct {
	Health                 *HealthHandler
	Panels                 *PanelHandler
	DeviceModels           *DeviceModelHandler
	PanelDevices           *PanelDeviceHandler
	CalibrationInstruments *CalibrationInstrumentHandler
	Calibrations           *CalibrationHandler
	PanelImages            *PanelImageHandler
	WorkOrders             *WorkOrderHandler
	Approvals              *ApprovalHandler
	Engineers              *EngineerHandler
	ChecklistItems         *ChecklistItemHandler
	PmReports              *PmReportHandler
	CmReports              *CmReportHandler
	Attachments            *AttachmentHandler
	Notifications          *NotificationHandler
}

// New wires the handlers onto the services.
func New(cfg *config.Config, svc *service.Services, health *HealthHandler) *Handlers {
	return &Handlers{
		Health:                 health,
		Panels:                 &PanelHandler{svc: svc.Panels},
		DeviceModels:           &DeviceModelHandler{svc: svc.DeviceModels},
		PanelDevices:           &PanelDeviceHandler{svc: svc.PanelDevices},
		CalibrationInstruments: &CalibrationInstrumentHandler{svc: svc.CalibrationInstruments},
		Calibrations:           &CalibrationHandler{svc: svc.Calibrations},
		PanelImages:            &PanelImageHandler{svc: svc.PanelImages},
		WorkOrders:             &WorkOrderHandler{svc: svc.WorkOrders},
		Approvals:              &ApprovalHandler{svc: svc.Approvals},
		Engineers:              &EngineerHandler{svc: svc.Engineers},
		ChecklistItems:         &ChecklistItemHandler{svc: svc.ChecklistItems},
		PmReports:              &PmReportHandler{svc: svc.PmReports},
		CmReports:              &CmReportHandler{svc: svc.CmReports},
		Attachments:            &AttachmentHandler{svc: svc.Attachments},
		Notifications:          &NotificationHandler{svc: svc.Notifications},
	}
}
