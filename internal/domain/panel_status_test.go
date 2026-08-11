package domain

import "testing"

func TestDeviceOperationalStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		comm, health, want string
	}{
		{"ONLINE", "NORMAL", PanelStatusNormal},
		{"OFFLINE", "NORMAL", PanelStatusAbnormal},
		{"ONLINE", "CRITICAL", PanelStatusAbnormal},
		{"DEGRADED", "NORMAL", PanelStatusMonitoring},
		{"ONLINE", "WARNING", PanelStatusMonitoring},
		{"UNKNOWN", "NORMAL", PanelStatusMonitoring},
		{"ONLINE", "UNKNOWN", PanelStatusMonitoring},
	}
	for _, tc := range cases {
		if got := DeviceOperationalStatus(tc.comm, tc.health); got != tc.want {
			t.Errorf("DeviceOperationalStatus(%q, %q) = %q, want %q", tc.comm, tc.health, got, tc.want)
		}
	}
}

func TestAggregatePanelStatus_priority(t *testing.T) {
	t.Parallel()
	if got := AggregatePanelStatus([]string{PanelStatusAbnormal, PanelStatusMonitoring}); got != PanelStatusMonitoring {
		t.Fatalf("monitoring+abnormal = %q, want MONITORING", got)
	}
	if got := AggregatePanelStatus([]string{PanelStatusAbnormal, PanelStatusNormal}); got != PanelStatusAbnormal {
		t.Fatalf("abnormal+normal = %q, want ABNORMAL", got)
	}
	if got := AggregatePanelStatus(nil); got != PanelStatusNormal {
		t.Fatalf("empty = %q, want NORMAL", got)
	}
}
