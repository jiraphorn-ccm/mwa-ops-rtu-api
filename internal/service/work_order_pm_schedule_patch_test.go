package service

import (
	"testing"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

func TestCheckPmScheduleTypePatch(t *testing.T) {
	three := "THREE_MONTH"

	t.Run("PM ASSIGNED allows change", func(t *testing.T) {
		err := checkPmScheduleTypePatch(sqlc.WorkOrder{
			WorkOrderType: "PM",
			Status:        "ASSIGNED",
		}, &three)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PM IN_PROGRESS allows change", func(t *testing.T) {
		err := checkPmScheduleTypePatch(sqlc.WorkOrder{
			WorkOrderType: "PM",
			Status:        "IN_PROGRESS",
		}, &three)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PM PENDING allows change", func(t *testing.T) {
		err := checkPmScheduleTypePatch(sqlc.WorkOrder{
			WorkOrderType: "PM",
			Status:        "PENDING",
		}, &three)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PM PENDING_APPROVAL rejects", func(t *testing.T) {
		err := checkPmScheduleTypePatch(sqlc.WorkOrder{
			WorkOrderType: "PM",
			Status:        "PENDING_APPROVAL",
		}, &three)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrWorkOrderStatusInvalid {
			t.Fatalf("got %#v", err)
		}
	})

	t.Run("CM rejects pm_schedule_type", func(t *testing.T) {
		err := checkPmScheduleTypePatch(sqlc.WorkOrder{
			WorkOrderType: "CM",
			Status:        "ASSIGNED",
		}, &three)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrPmScheduleTypeNotAllowed {
			t.Fatalf("got %#v", err)
		}
	})

	t.Run("PM requires non-null value", func(t *testing.T) {
		err := checkPmScheduleTypePatch(sqlc.WorkOrder{
			WorkOrderType: "PM",
			Status:        "ASSIGNED",
		}, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrPmScheduleTypeRequired {
			t.Fatalf("got %#v", err)
		}
	})
}
