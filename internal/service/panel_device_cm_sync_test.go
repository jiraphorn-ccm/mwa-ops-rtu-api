package service

import "testing"

func TestIsOpenCmWorkOrder(t *testing.T) {
	tests := []struct {
		woType string
		status string
		want   bool
	}{
		{"CM", "ASSIGNED", true},
		{"CM", "PENDING_APPROVAL", true},
		{"CM", "COMPLETED", false},
		{"CM", "CONDITIONAL", false},
		{"PM", "ASSIGNED", false},
	}
	for _, tc := range tests {
		if got := isOpenCmWorkOrder(tc.woType, tc.status); got != tc.want {
			t.Errorf("isOpenCmWorkOrder(%q, %q) = %v, want %v", tc.woType, tc.status, got, tc.want)
		}
	}
}

func TestCmClosedHealthStatus(t *testing.T) {
	tests := []struct {
		final  string
		health string
		wantOK bool
	}{
		{"COMPLETED", "NORMAL", true},
		{"CONDITIONAL", "WARNING", true},
		{"PENDING", "", false},
	}
	for _, tc := range tests {
		got, ok := cmClosedHealthStatus(tc.final)
		if ok != tc.wantOK || got != tc.health {
			t.Errorf("cmClosedHealthStatus(%q) = (%q, %v), want (%q, %v)", tc.final, got, ok, tc.health, tc.wantOK)
		}
	}
}

func TestShouldClearCmHealthOnRecalc(t *testing.T) {
	tests := []struct {
		health string
		want   bool
	}{
		{"WARNING", true},
		{"NORMAL", false},
		{"CRITICAL", false},
		{"UNKNOWN", false},
	}
	for _, tc := range tests {
		if got := shouldClearCmHealthOnRecalc(tc.health); got != tc.want {
			t.Errorf("shouldClearCmHealthOnRecalc(%q) = %v, want %v", tc.health, got, tc.want)
		}
	}
}
