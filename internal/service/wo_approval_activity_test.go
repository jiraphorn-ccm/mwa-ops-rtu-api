package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/repository"
)

func TestBuildApprovalOutcomeRejectedRework(t *testing.T) {
	woID := uuid.New()
	roundID := uuid.New()
	reviewer := uuid.New()
	assignee := uuid.New()
	note := "fix checklist"

	outcome, notify := buildApprovalOutcome(
		repository.WorkOrderView{WorkOrder: sqlc.WorkOrder{ID: woID, Status: "PENDING_APPROVAL"}},
		ApprovalDecisionInput{ReviewerID: reviewer, Decision: "REJECTED", Note: &note},
		false, nil, roundID, &assignee,
	)
	if notify == nil || *notify != assignee {
		t.Fatalf("notify rework assignee: got %v", notify)
	}
	if outcome.NewStatus != "PENDING" || outcome.Rework == nil {
		t.Fatalf("expected PENDING rework outcome, got %+v", outcome)
	}
	if len(outcome.ActivityLogs) != 1 {
		t.Fatalf("expected 1 activity log, got %d", len(outcome.ActivityLogs))
	}
	log := outcome.ActivityLogs[0]
	if log.Action != "REJECTED" || log.WorkOrderRoundID == nil || *log.WorkOrderRoundID != roundID {
		t.Fatalf("unexpected reject log: %+v", log)
	}
	if log.FromStatus == nil || *log.FromStatus != "PENDING_APPROVAL" {
		t.Fatalf("from_status: %+v", log.FromStatus)
	}
	if log.ToStatus == nil || *log.ToStatus != "PENDING" {
		t.Fatalf("to_status: %+v", log.ToStatus)
	}
}

func TestBuildApprovalOutcomeRejectedEscalate(t *testing.T) {
	woID := uuid.New()
	roundID := uuid.New()
	reviewer := uuid.New()
	cmNo := "CM-RTU-00001-0002"

	outcome, notify := buildApprovalOutcome(
		repository.WorkOrderView{WorkOrder: sqlc.WorkOrder{ID: woID, Status: "PENDING_APPROVAL"}},
		ApprovalDecisionInput{ReviewerID: reviewer, Decision: "REJECTED"},
		true, &cmNo, roundID, nil,
	)
	if notify != nil {
		t.Fatalf("escalate should not notify rework")
	}
	if outcome.NewStatus != "CONDITIONAL" || !outcome.CloseWO {
		t.Fatalf("expected CONDITIONAL close, got %+v", outcome)
	}
	if len(outcome.ActivityLogs) != 2 {
		t.Fatalf("expected REJECTED + CM_SPAWNED, got %d logs", len(outcome.ActivityLogs))
	}
	if outcome.ActivityLogs[0].Action != "REJECTED" {
		t.Fatalf("first log: %+v", outcome.ActivityLogs[0])
	}
	spawn := outcome.ActivityLogs[1]
	if spawn.Action != "CM_SPAWNED" || spawn.Note == nil {
		t.Fatalf("spawn log: %+v", spawn)
	}
	want := "Escalated to CM work order CM-RTU-00001-0002"
	if *spawn.Note != want {
		t.Fatalf("note = %q, want %q", *spawn.Note, want)
	}
}

func TestBuildApprovalOutcomeApproved(t *testing.T) {
	woID := uuid.New()
	roundID := uuid.New()
	reviewer := uuid.New()

	outcome, _ := buildApprovalOutcome(
		repository.WorkOrderView{WorkOrder: sqlc.WorkOrder{ID: woID, Status: "PENDING_APPROVAL"}},
		ApprovalDecisionInput{ReviewerID: reviewer, Decision: "APPROVED"},
		false, nil, roundID, nil,
	)
	if len(outcome.ActivityLogs) != 1 || outcome.ActivityLogs[0].Action != "APPROVED" {
		t.Fatalf("logs: %+v", outcome.ActivityLogs)
	}
	if outcome.ActivityLogs[0].ToStatus == nil || *outcome.ActivityLogs[0].ToStatus != "COMPLETED" {
		t.Fatalf("to_status: %+v", outcome.ActivityLogs[0].ToStatus)
	}
}
