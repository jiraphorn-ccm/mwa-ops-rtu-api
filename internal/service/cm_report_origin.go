package service

import (
	"strings"

	"github.com/rtu-api/internal/db/sqlc"
)

// CmReportOrigin classifies a cm_reports row for repair-history UI.
const (
	CmOriginStandalone        = "STANDALONE"
	CmOriginPMOnsiteCM        = "PM_ONSITE_CM"
	CmOriginPMEscalated       = "PM_ESCALATED"
	CmOriginPMOnsiteFixLegacy = "PM_ONSITE_FIX_LEGACY"
)

// ComputeCmReportOrigin derives origin from FKs and report content.
func ComputeCmReportOrigin(cr sqlc.CmReport) string {
	if cr.WorkOrderID == nil {
		if cr.PmReportID != nil {
			return CmOriginPMOnsiteFixLegacy
		}
		return "UNKNOWN"
	}
	if cr.PmReportID != nil {
		if cr.PendingReason != nil && strings.TrimSpace(*cr.PendingReason) != "" {
			return CmOriginPMEscalated
		}
		return CmOriginPMOnsiteCM
	}
	return CmOriginStandalone
}

// IsCmRepairCompleted reports whether the repair counts as approved/finished.
func IsCmRepairCompleted(workOrderStatus *string, origin string) bool {
	if origin == CmOriginPMOnsiteFixLegacy {
		return true
	}
	if workOrderStatus == nil {
		return false
	}
	switch *workOrderStatus {
	case "COMPLETED", "CONDITIONAL":
		return true
	default:
		return false
	}
}
