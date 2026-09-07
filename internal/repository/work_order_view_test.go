package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
)

func TestWorkOrderViewJSONIncludesPanelCode(t *testing.T) {
	t.Parallel()

	v := WorkOrderView{
		WorkOrder: sqlc.WorkOrder{ID: uuid.New(), PanelID: uuid.New()},
		PanelCode: "RTU-00003",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, `"panel_code":"RTU-00003"`) {
		t.Fatalf("json missing panel_code: %s", body)
	}
}

func TestWorkOrderListQueryIncludesPanelCode(t *testing.T) {
	t.Parallel()

	if !strings.Contains(workOrderColumns, "p.code AS panel_code") {
		t.Fatalf("workOrderColumns missing panel_code: %s", workOrderColumns)
	}
	if !strings.Contains(workOrderFrom, "JOIN rtu.panels p ON p.id = wo.panel_id") {
		t.Fatalf("workOrderFrom missing panels join: %s", workOrderFrom)
	}
}
