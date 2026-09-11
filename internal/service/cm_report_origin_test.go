package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
)

func TestComputeCmReportOrigin(t *testing.T) {
	wo := uuid.New()
	pm := uuid.New()
	pending := "waiting for parts"
	action := "replaced fuse"

	tests := []struct {
		name   string
		report sqlc.CmReport
		want   string
	}{
		{
			name:   "standalone",
			report: sqlc.CmReport{WorkOrderID: &wo},
			want:   CmOriginStandalone,
		},
		{
			name:   "pm onsite cm",
			report: sqlc.CmReport{WorkOrderID: &wo, PmReportID: &pm, CorrectiveAction: &action},
			want:   CmOriginPMOnsiteCM,
		},
		{
			name:   "pm escalated",
			report: sqlc.CmReport{WorkOrderID: &wo, PmReportID: &pm, PendingReason: &pending},
			want:   CmOriginPMEscalated,
		},
		{
			name:   "legacy onsite",
			report: sqlc.CmReport{PmReportID: &pm},
			want:   CmOriginPMOnsiteFixLegacy,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeCmReportOrigin(tc.report); got != tc.want {
				t.Fatalf("origin = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsCmRepairCompleted(t *testing.T) {
	completed := "COMPLETED"
	pending := "PENDING_APPROVAL"
	if !IsCmRepairCompleted(&completed, CmOriginPMOnsiteCM) {
		t.Fatal("expected completed CM onsite")
	}
	if IsCmRepairCompleted(&pending, CmOriginPMOnsiteCM) {
		t.Fatal("pending approval should not be completed")
	}
	if !IsCmRepairCompleted(nil, CmOriginPMOnsiteFixLegacy) {
		t.Fatal("legacy onsite treated completed")
	}
}
