package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildOpenCmWorkOrderConditions(t *testing.T) {
	panelID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deviceID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	topicID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	excludeID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	t.Run("base status filter", func(t *testing.T) {
		a := &args{}
		where := buildOpenCmWorkOrderConditions(a, panelID, OpenCmWorkOrderFilter{}).where()
		for _, want := range []string{
			"wo.work_order_type = 'CM'",
			"wo.status IN ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL')",
		} {
			if !strings.Contains(where, want) {
				t.Fatalf("where=%q missing %q", where, want)
			}
		}
	})

	t.Run("device and topic filters use junction table", func(t *testing.T) {
		a := &args{}
		where := buildOpenCmWorkOrderConditions(a, panelID, OpenCmWorkOrderFilter{
			PanelDeviceID:  &deviceID,
			ProblemTopicID: &topicID,
		}).where()
		if !strings.Contains(where, openCmEffectiveDevice) {
			t.Fatalf("where=%q missing effective device expression", where)
		}
		if !strings.Contains(where, "work_order_problem_topics wopt") {
			t.Fatalf("where=%q missing junction topic predicate", where)
		}
	})

	t.Run("topic only uses junction table", func(t *testing.T) {
		a := &args{}
		where := buildOpenCmWorkOrderConditions(a, panelID, OpenCmWorkOrderFilter{
			ProblemTopicID: &topicID,
		}).where()
		if !strings.Contains(where, "work_order_problem_topics wopt") {
			t.Fatalf("where=%q missing junction topic predicate", where)
		}
	})

	t.Run("exclude work order", func(t *testing.T) {
		a := &args{}
		where := buildOpenCmWorkOrderConditions(a, panelID, OpenCmWorkOrderFilter{
			ExcludeWorkOrderID: &excludeID,
		}).where()
		if !strings.Contains(where, "wo.id <> $2") {
			t.Fatalf("where=%q missing exclude predicate", where)
		}
	})
}
