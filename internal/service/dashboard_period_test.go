package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/rtu-api/internal/repository"
)

func TestResolveDashboardPeriodMonth(t *testing.T) {
	loc := dashboardLocation()
	now := time.Date(2026, 9, 17, 13, 35, 0, 0, loc)
	w := resolveDashboardPeriod(PeriodMonth, now)
	if w.Key != PeriodMonth {
		t.Fatalf("key=%s", w.Key)
	}
	if !w.From.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("from=%s", w.From)
	}
	if !w.To.Equal(now) {
		t.Fatalf("to=%s", w.To)
	}
	if !w.PreviousFrom.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("previous_from=%s", w.PreviousFrom)
	}
	if !w.PreviousTo.Equal(time.Date(2026, 8, 17, 13, 35, 0, 0, loc)) {
		t.Fatalf("previous_to=%s", w.PreviousTo)
	}
}

func TestParseDashboardPeriod(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/dashboard", nil)
	got, err := ParseDashboardPeriod(req)
	if err != nil || got != PeriodMonth {
		t.Fatalf("default=%s err=%v", got, err)
	}

	req, _ = http.NewRequest(http.MethodGet, "/dashboard?period=7d", nil)
	got, err = ParseDashboardPeriod(req)
	if err != nil || got != Period7D {
		t.Fatalf("7d=%s err=%v", got, err)
	}

	req, _ = http.NewRequest(http.MethodGet, "/dashboard?period=week", nil)
	if _, err = ParseDashboardPeriod(req); err == nil {
		t.Fatal("expected invalid period error")
	}
}

func TestResolveDashboardPeriodDefault(t *testing.T) {
	w := resolveDashboardPeriod("", time.Date(2026, 1, 5, 9, 0, 0, 0, dashboardLocation()))
	if w.Key != PeriodMonth {
		t.Fatalf("default key=%s", w.Key)
	}
}

func TestResolveDashboardPeriodTodayAnd7D(t *testing.T) {
	loc := dashboardLocation()
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, loc)
	today := resolveDashboardPeriod(PeriodToday, now)
	if !today.From.Equal(time.Date(2026, 9, 17, 0, 0, 0, 0, loc)) {
		t.Fatalf("today from=%s", today.From)
	}
	week := resolveDashboardPeriod(Period7D, now)
	if !week.From.Equal(time.Date(2026, 9, 11, 0, 0, 0, 0, loc)) {
		t.Fatalf("7d from=%s", week.From)
	}
}

func TestDashboardBucketsYear(t *testing.T) {
	loc := dashboardLocation()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, loc)
	w := resolveDashboardPeriod(PeriodYear, now)
	buckets := dashboardBuckets(w)
	if len(buckets) != 7 {
		t.Fatalf("year buckets=%d want 7", len(buckets))
	}
	if buckets[0].Label != "ม.ค." || buckets[6].Label != "ก.ค." {
		t.Fatalf("labels=%v %v", buckets[0].Label, buckets[6].Label)
	}
	if !buckets[6].End.Equal(now) {
		t.Fatalf("last end=%s", buckets[6].End)
	}
}

func TestDashboardPercent(t *testing.T) {
	if got := dashboardPercent(128, 132); got != 97.0 {
		t.Fatalf("percent=%v", got)
	}
	if got := dashboardPercent(0, 0); got != 0 {
		t.Fatalf("zero total=%v", got)
	}
}

func TestBuildDashboardDecisions(t *testing.T) {
	age := 19.2
	got := buildDashboardDecisions(2, repository.DashboardApprovals{OverSLA: 4, OldestAgeHours: &age}, 78, 46)
	if len(got) != 3 {
		t.Fatalf("decisions=%d", len(got))
	}
	if got[0].Key != "critical_cm_unstarted" || got[1].Tone != "amber" || got[2].Key != "pm_below_target" {
		t.Fatalf("keys=%v %v %v", got[0].Key, got[1].Key, got[2].Key)
	}
	none := buildDashboardDecisions(0, repository.DashboardApprovals{}, 95, 10)
	if none == nil || len(none) != 0 {
		t.Fatalf("expected empty decisions, got %#v", none)
	}
}

func TestOverlayLiveAvailabilityUsesCurrentPercentOnLastBucket(t *testing.T) {
	buckets := []DashboardBucket{
		{Key: "2026-06", Label: "มิ.ย."},
		{Key: "2026-07", Label: "ก.ค."},
	}
	rows := []repository.DashboardAvailabilityBucket{
		{Ord: 1, Availability: 96.1},
		{Ord: 2, Availability: 90},
	}
	got := overlayLiveAvailability(buckets, rows, 97.0)
	if got[0].Availability != 96.1 {
		t.Fatalf("first=%v", got[0].Availability)
	}
	if got[1].Availability != 97.0 {
		t.Fatalf("live overlay=%v", got[1].Availability)
	}
}
