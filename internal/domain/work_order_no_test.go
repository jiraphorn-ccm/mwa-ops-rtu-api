package domain

import "testing"

func TestFormatWorkOrderNo(t *testing.T) {
	tests := []struct {
		typ, panel string
		seq        int64
		want       string
	}{
		{"PM", "RTU-011", 1, "PM-RTU-011-0001"},
		{"CM", "RTU-011", 42, "CM-RTU-011-0042"},
		{"PM", "U120", 9999, "PM-U120-9999"},
		{"PM", "RTU-011", 10_000, "PM-RTU-011-10000"},
		{"CM", "RTU-011", 10_001, "CM-RTU-011-10001"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatWorkOrderNo(tt.typ, tt.panel, tt.seq); got != tt.want {
				t.Fatalf("FormatWorkOrderNo() = %q, want %q", got, tt.want)
			}
		})
	}
}
