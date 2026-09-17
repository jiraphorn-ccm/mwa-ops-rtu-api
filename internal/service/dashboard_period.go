package service

import (
	"fmt"
	"math"
	"time"

	_ "time/tzdata"
)

// Dashboard period keys accepted by GET /dashboard?period=.
const (
	PeriodToday = "TODAY"
	Period7D    = "7D"
	PeriodMonth = "MONTH"
	PeriodYear  = "YEAR"
)

const dashboardTimezone = "Asia/Bangkok"

// Dashboard targets surfaced to the client so the UI does not hard-code them.
const (
	DashboardAvailabilityTarget    = 97.0
	DashboardPMPlanTarget          = 90.0
	DashboardSLATarget             = 90.0
	DashboardApprovalSLAHours      = 8
	DashboardNearSLADays           = 2
	DashboardStationsAtRiskTop     = 10
	DashboardStationsAtRiskDefault = 50
	DashboardStationsAtRiskMax     = 200
)

var thaiMonths = []string{
	"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.",
	"ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค.",
}

// DashboardWindow is a calendar window in Asia/Bangkok plus the matching
// previous window used for delta comparisons.
type DashboardWindow struct {
	Key          string
	Timezone     string
	From         time.Time
	To           time.Time
	PreviousFrom time.Time
	PreviousTo   time.Time
}

// DashboardBucket is one point on an availability or maintenance series.
type DashboardBucket struct {
	Start time.Time
	End   time.Time
	Key   string
	Label string
}

func dashboardLocation() *time.Location {
	loc, err := time.LoadLocation(dashboardTimezone)
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}

func resolveDashboardPeriod(key string, now time.Time) DashboardWindow {
	if key == "" {
		key = PeriodMonth
	}
	loc := dashboardLocation()
	now = now.In(loc)

	var from time.Time
	switch key {
	case PeriodToday:
		from = startOfDay(now)
	case Period7D:
		from = startOfDay(now).AddDate(0, 0, -6)
	case PeriodYear:
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
	default:
		key = PeriodMonth
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	}

	to := now
	prevFrom, prevTo := previousDashboardWindow(key, from, to)
	return DashboardWindow{
		Key:          key,
		Timezone:     dashboardTimezone,
		From:         from,
		To:           to,
		PreviousFrom: prevFrom,
		PreviousTo:   prevTo,
	}
}

func previousDashboardWindow(key string, from, to time.Time) (time.Time, time.Time) {
	switch key {
	case PeriodToday:
		return from.AddDate(0, 0, -1), to.AddDate(0, 0, -1)
	case Period7D:
		return from.AddDate(0, 0, -7), to.AddDate(0, 0, -7)
	case PeriodYear:
		return from.AddDate(-1, 0, 0), to.AddDate(-1, 0, 0)
	default:
		return from.AddDate(0, -1, 0), to.AddDate(0, -1, 0)
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func dashboardBuckets(window DashboardWindow) []DashboardBucket {
	loc := window.From.Location()
	switch window.Key {
	case PeriodToday:
		return hourlyBuckets(window.From, window.To)
	case PeriodYear:
		return monthlyBuckets(window.From, window.To, loc)
	default:
		return dailyBuckets(window.From, window.To)
	}
}

func hourlyBuckets(from, to time.Time) []DashboardBucket {
	var out []DashboardBucket
	cur := from.Truncate(time.Hour)
	for cur.Before(to) {
		end := cur.Add(time.Hour)
		if end.After(to) {
			end = to
		}
		out = append(out, DashboardBucket{
			Start: cur,
			End:   end,
			Key:   cur.Format("2006-01-02T15"),
			Label: cur.Format("15:04"),
		})
		cur = cur.Add(time.Hour)
	}
	if len(out) == 0 {
		out = append(out, DashboardBucket{
			Start: from,
			End:   to,
			Key:   from.Format("2006-01-02T15"),
			Label: from.Format("15:04"),
		})
	}
	return out
}

func dailyBuckets(from, to time.Time) []DashboardBucket {
	var out []DashboardBucket
	cur := startOfDay(from)
	last := startOfDay(to)
	for !cur.After(last) {
		end := cur.AddDate(0, 0, 1)
		if end.After(to) {
			end = to
		}
		out = append(out, DashboardBucket{
			Start: cur,
			End:   end,
			Key:   cur.Format("2006-01-02"),
			Label: thaiDayLabel(cur),
		})
		cur = cur.AddDate(0, 0, 1)
	}
	return out
}

func monthlyBuckets(from, to time.Time, loc *time.Location) []DashboardBucket {
	var out []DashboardBucket
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc)
	last := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, loc)
	for !cur.After(last) {
		end := cur.AddDate(0, 1, 0)
		if end.After(to) {
			end = to
		}
		if !end.After(cur) {
			end = to
		}
		out = append(out, DashboardBucket{
			Start: cur,
			End:   end,
			Key:   cur.Format("2006-01"),
			Label: thaiMonths[int(cur.Month())-1],
		})
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

func thaiDayLabel(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), thaiMonths[int(t.Month())-1])
}

func dashboardPercent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*1000) / 10
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
