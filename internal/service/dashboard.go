package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/domain"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// DashboardService assembles the executive dashboard from live panel status
// and work-order aggregates.
type DashboardService struct {
	repo *repository.DashboardRepository
}

// ParseDashboardPeriod reads GET /dashboard?period=.
func ParseDashboardPeriod(r *http.Request) (string, error) {
	q := httpx.NewQuery(r)
	period := q.Enum("period", PeriodToday, Period7D, PeriodMonth, PeriodYear)
	if err := q.Err(); err != nil {
		return "", err
	}
	if period == nil {
		return PeriodMonth, nil
	}
	return *period, nil
}

// ParseDashboardRiskLimit reads GET /dashboard/stations-at-risk?limit=.
func ParseDashboardRiskLimit(r *http.Request) (int, error) {
	return parseDashboardRiskLimit(r, DashboardStationsAtRiskDefault)
}

func parseDashboardRiskLimit(r *http.Request, def int) (int, error) {
	q := httpx.NewQuery(r)
	limit := q.Int("limit", def, 1, DashboardStationsAtRiskMax)
	if err := q.Err(); err != nil {
		return 0, err
	}
	return limit, nil
}

// DashboardOverview is the GET /dashboard payload — every executive widget
// except the map, which is GET /dashboard/map.
type DashboardOverview struct {
	GeneratedAt    time.Time                  `json:"generated_at"`
	Period         DashboardPeriodView        `json:"period"`
	Targets        DashboardTargets           `json:"targets"`
	Metrics        DashboardMetrics           `json:"metrics"`
	Availability   DashboardAvailability      `json:"availability"`
	Maintenance    DashboardMaintenance       `json:"maintenance"`
	SLA            DashboardSLA               `json:"sla"`
	Decisions      []DashboardDecision        `json:"decisions"`
	StationsAtRisk []DashboardStationRiskView `json:"stations_at_risk"`
}

// DashboardPeriodView describes the selected calendar window.
type DashboardPeriodView struct {
	Key          string    `json:"key"`
	Timezone     string    `json:"timezone"`
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	PreviousFrom time.Time `json:"previous_from"`
	PreviousTo   time.Time `json:"previous_to"`
}

// DashboardTargets is the threshold set the UI should render next to KPIs.
type DashboardTargets struct {
	AvailabilityPercent float64 `json:"availability_percent"`
	PMPlanPercent       float64 `json:"pm_plan_percent"`
	SLAPercent          float64 `json:"sla_percent"`
	ApprovalSLAHours    int     `json:"approval_sla_hours"`
}

// DashboardMetrics is the six KPI cards on the dashboard.
type DashboardMetrics struct {
	Stations         DashboardStationMetrics  `json:"stations"`
	PM               DashboardPMMetrics       `json:"pm"`
	CMOpen           DashboardCountDelta      `json:"cm_open"`
	PendingApprovals DashboardApprovalMetrics `json:"pending_approvals"`
}

// DashboardStationMetrics is live RTU station health (not period-filtered).
type DashboardStationMetrics struct {
	Total         int64   `json:"total"`
	AreaCount     int64   `json:"area_count"`
	OnlineCount   int64   `json:"online_count"`
	OnlinePercent float64 `json:"online_percent"`
	AbnormalCount int64   `json:"abnormal_count"`
	CriticalCount int64   `json:"critical_count"`
	WatchCount    int64   `json:"watch_count"`
}

// DashboardPMMetrics is PM completed-on-plan for the selected period.
type DashboardPMMetrics struct {
	Planned      int64   `json:"planned"`
	Completed    int64   `json:"completed_on_plan"`
	Percent      float64 `json:"percent"`
	DeltaPercent float64 `json:"delta_percent"`
}

// DashboardCountDelta is an open-queue count with period-over-period change.
type DashboardCountDelta struct {
	Count      int64 `json:"count"`
	OverSLA    int64 `json:"over_sla"`
	DeltaCount int64 `json:"delta_count"`
}

// DashboardApprovalMetrics is the pending-approval queue.
type DashboardApprovalMetrics struct {
	Count          int64    `json:"count"`
	OverSLA        int64    `json:"over_sla"`
	OldestAgeHours *float64 `json:"oldest_age_hours"`
}

// DashboardAvailability is the availability KPI plus the trend series.
type DashboardAvailability struct {
	CurrentPercent float64                      `json:"current_percent"`
	TargetPercent  float64                      `json:"target_percent"`
	Series         []DashboardAvailabilityPoint `json:"series"`
}

// DashboardAvailabilityPoint is one availability series sample.
type DashboardAvailabilityPoint struct {
	Bucket       string  `json:"bucket"`
	Label        string  `json:"label"`
	Availability float64 `json:"availability"`
}

// DashboardMaintenance is the PM/CM bar chart.
type DashboardMaintenance struct {
	Series []DashboardMaintenancePoint `json:"series"`
}

// DashboardMaintenancePoint is one PM/CM series sample.
type DashboardMaintenancePoint struct {
	Bucket      string `json:"bucket"`
	Label       string `json:"label"`
	PMCompleted int64  `json:"pm_completed"`
	PMDelayed   int64  `json:"pm_delayed"`
	CMCompleted int64  `json:"cm_completed"`
	CMOpen      int64  `json:"cm_open"`
}

// DashboardSLA is SLA compliance for the selected period.
type DashboardSLA struct {
	Percent       float64 `json:"percent"`
	TargetPercent float64 `json:"target_percent"`
	DeltaPercent  float64 `json:"delta_percent"`
	Within        int64   `json:"within"`
	Near          int64   `json:"near"`
	Over          int64   `json:"over"`
	AvgCloseHours float64 `json:"avg_close_hours"`
}

// DashboardDecision is one item in "เรื่องที่ต้องตัดสินใจ".
type DashboardDecision struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Tone   string `json:"tone"`
	Count  int64  `json:"count"`
}

// DashboardStationRiskView is one row of "สถานีที่ต้องให้ความสนใจ".
type DashboardStationRiskView struct {
	ID                uuid.UUID `json:"id"`
	Code              string    `json:"code"`
	Location          string    `json:"location"`
	OperationalStatus string    `json:"operational_status"`
	LatestIssue       string    `json:"latest_issue"`
	OpenWorkOrders    int64     `json:"open_work_orders"`
	DowntimeSeconds   int64     `json:"downtime_seconds"`
}

// DashboardMapStationView is one marker for GET /dashboard/map.
type DashboardMapStationView struct {
	ID                uuid.UUID        `json:"id"`
	Code              string           `json:"code"`
	Location          string           `json:"location"`
	Latitude          *decimal.Decimal `json:"latitude"`
	Longitude         *decimal.Decimal `json:"longitude"`
	OperationalStatus string           `json:"operational_status"`
}

// Overview builds the executive dashboard for the requested period.
func (s *DashboardService) Overview(ctx context.Context, periodKey string, now time.Time) (DashboardOverview, error) {
	window := resolveDashboardPeriod(periodKey, now)
	buckets := dashboardBuckets(window)

	stations, err := s.repo.StationSnapshot(ctx)
	if err != nil {
		return DashboardOverview{}, err
	}
	pm, err := s.repo.PMPlan(ctx, window.From, window.To)
	if err != nil {
		return DashboardOverview{}, err
	}
	prevPM, err := s.repo.PMPlan(ctx, window.PreviousFrom, window.PreviousTo)
	if err != nil {
		return DashboardOverview{}, err
	}
	cm, err := s.repo.CMOpen(ctx, window.To)
	if err != nil {
		return DashboardOverview{}, err
	}
	prevCM, err := s.repo.CMOpen(ctx, window.PreviousTo)
	if err != nil {
		return DashboardOverview{}, err
	}
	approvals, err := s.repo.PendingApprovals(ctx, DashboardApprovalSLAHours)
	if err != nil {
		return DashboardOverview{}, err
	}
	sla, err := s.repo.SLA(ctx, window.From, window.To, window.To, DashboardNearSLADays)
	if err != nil {
		return DashboardOverview{}, err
	}
	prevSLA, err := s.repo.SLA(ctx, window.PreviousFrom, window.PreviousTo, window.PreviousTo, DashboardNearSLADays)
	if err != nil {
		return DashboardOverview{}, err
	}
	unstarted, err := s.repo.CriticalCMUnstarted(ctx)
	if err != nil {
		return DashboardOverview{}, err
	}

	starts, ends := bucketBounds(buckets)
	availRows, err := s.repo.AvailabilitySeries(ctx, starts, ends, stations.Total)
	if err != nil {
		return DashboardOverview{}, err
	}
	maintRows, err := s.repo.MaintenanceSeries(ctx, starts, ends)
	if err != nil {
		return DashboardOverview{}, err
	}
	risks, err := s.repo.StationsAtRisk(ctx, DashboardStationsAtRiskTop)
	if err != nil {
		return DashboardOverview{}, err
	}

	pmPercent := dashboardPercent(pm.CompletedOnPlan, pm.Planned)
	prevPMPercent := dashboardPercent(prevPM.CompletedOnPlan, prevPM.Planned)
	slaValue := slaPercent(sla)
	prevSLAValue := slaPercent(prevSLA)
	onlinePercent := dashboardPercent(stations.Online, stations.Total)

	avgClose := 0.0
	if sla.AvgCloseHours != nil {
		avgClose = round1(*sla.AvgCloseHours)
	}

	return DashboardOverview{
		GeneratedAt: window.To.UTC(),
		Period: DashboardPeriodView{
			Key:          window.Key,
			Timezone:     window.Timezone,
			From:         window.From,
			To:           window.To,
			PreviousFrom: window.PreviousFrom,
			PreviousTo:   window.PreviousTo,
		},
		Targets: DashboardTargets{
			AvailabilityPercent: DashboardAvailabilityTarget,
			PMPlanPercent:       DashboardPMPlanTarget,
			SLAPercent:          DashboardSLATarget,
			ApprovalSLAHours:    DashboardApprovalSLAHours,
		},
		Metrics: DashboardMetrics{
			Stations: DashboardStationMetrics{
				Total:         stations.Total,
				AreaCount:     stations.Areas,
				OnlineCount:   stations.Online,
				OnlinePercent: onlinePercent,
				AbnormalCount: stations.Critical + stations.Watch,
				CriticalCount: stations.Critical,
				WatchCount:    stations.Watch,
			},
			PM: DashboardPMMetrics{
				Planned:      pm.Planned,
				Completed:    pm.CompletedOnPlan,
				Percent:      pmPercent,
				DeltaPercent: round1(pmPercent - prevPMPercent),
			},
			CMOpen: DashboardCountDelta{
				Count:      cm.Count,
				OverSLA:    cm.OverSLA,
				DeltaCount: cm.Count - prevCM.Count,
			},
			PendingApprovals: DashboardApprovalMetrics{
				Count:          approvals.Count,
				OverSLA:        approvals.OverSLA,
				OldestAgeHours: round1Ptr(approvals.OldestAgeHours),
			},
		},
		Availability: DashboardAvailability{
			CurrentPercent: onlinePercent,
			TargetPercent:  DashboardAvailabilityTarget,
			Series:         overlayLiveAvailability(buckets, availRows, onlinePercent),
		},
		Maintenance: DashboardMaintenance{
			Series: mapMaintenanceSeries(buckets, maintRows),
		},
		SLA: DashboardSLA{
			Percent:       slaValue,
			TargetPercent: DashboardSLATarget,
			DeltaPercent:  round1(slaValue - prevSLAValue),
			Within:        sla.Within,
			Near:          sla.Near,
			Over:          sla.Over,
			AvgCloseHours: avgClose,
		},
		Decisions:      buildDashboardDecisions(unstarted, approvals, pmPercent, pm.Planned),
		StationsAtRisk: mapStationRisks(risks),
	}, nil
}

// StationsAtRisk returns the ranked attention list (full collection).
func (s *DashboardService) StationsAtRisk(ctx context.Context, limit int) ([]DashboardStationRiskView, error) {
	if limit <= 0 {
		limit = DashboardStationsAtRiskTop
	}
	if limit > DashboardStationsAtRiskMax {
		limit = DashboardStationsAtRiskMax
	}
	rows, err := s.repo.StationsAtRisk(ctx, limit)
	if err != nil {
		return nil, err
	}
	return mapStationRisks(rows), nil
}

// MapStations returns active panels that have coordinates.
func (s *DashboardService) MapStations(ctx context.Context) ([]DashboardMapStationView, error) {
	rows, err := s.repo.MapStations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DashboardMapStationView, 0, len(rows))
	for _, row := range rows {
		out = append(out, DashboardMapStationView{
			ID:                row.ID,
			Code:              row.Code,
			Location:          derefString(row.Location),
			Latitude:          row.Latitude,
			Longitude:         row.Longitude,
			OperationalStatus: fallbackStatus(row.OperationalStatus),
		})
	}
	return out, nil
}

func slaPercent(row repository.DashboardSLARow) float64 {
	return dashboardPercent(row.Within, row.Within+row.Near+row.Over)
}

func bucketBounds(buckets []DashboardBucket) ([]time.Time, []time.Time) {
	starts := make([]time.Time, len(buckets))
	ends := make([]time.Time, len(buckets))
	for i, b := range buckets {
		starts[i] = b.Start
		ends[i] = b.End
	}
	return starts, ends
}

func overlayLiveAvailability(buckets []DashboardBucket, rows []repository.DashboardAvailabilityBucket, live float64) []DashboardAvailabilityPoint {
	byOrd := make(map[int64]float64, len(rows))
	for _, row := range rows {
		byOrd[row.Ord] = round1(row.Availability)
	}
	out := make([]DashboardAvailabilityPoint, 0, len(buckets))
	last := len(buckets) - 1
	for i, b := range buckets {
		value := byOrd[int64(i+1)]
		if i == last {
			value = live
		}
		out = append(out, DashboardAvailabilityPoint{
			Bucket:       b.Key,
			Label:        b.Label,
			Availability: value,
		})
	}
	return out
}

func mapMaintenanceSeries(buckets []DashboardBucket, rows []repository.DashboardMaintenanceBucket) []DashboardMaintenancePoint {
	byOrd := make(map[int64]repository.DashboardMaintenanceBucket, len(rows))
	for _, row := range rows {
		byOrd[row.Ord] = row
	}
	out := make([]DashboardMaintenancePoint, 0, len(buckets))
	for i, b := range buckets {
		row := byOrd[int64(i+1)]
		out = append(out, DashboardMaintenancePoint{
			Bucket:      b.Key,
			Label:       b.Label,
			PMCompleted: row.PMCompleted,
			PMDelayed:   row.PMDelayed,
			CMCompleted: row.CMCompleted,
			CMOpen:      row.CMOpen,
		})
	}
	return out
}

func mapStationRisks(rows []repository.DashboardStationRisk) []DashboardStationRiskView {
	out := make([]DashboardStationRiskView, 0, len(rows))
	for _, row := range rows {
		out = append(out, DashboardStationRiskView{
			ID:                row.ID,
			Code:              row.Code,
			Location:          derefString(row.Location),
			OperationalStatus: fallbackStatus(row.OperationalStatus),
			LatestIssue:       derefString(row.LatestIssue),
			OpenWorkOrders:    row.OpenWorkOrders,
			DowntimeSeconds:   row.DowntimeSeconds,
		})
	}
	return out
}

func buildDashboardDecisions(unstarted int64, approvals repository.DashboardApprovals, pmPercent float64, pmPlanned int64) []DashboardDecision {
	out := make([]DashboardDecision, 0, 3)
	if unstarted > 0 {
		out = append(out, DashboardDecision{
			Key:    "critical_cm_unstarted",
			Title:  "งาน CM ระดับวิกฤตยังไม่มีผู้รับผิดชอบ",
			Detail: fmt.Sprintf("%d งาน · ควรมอบหมายภายในวันนี้", unstarted),
			Tone:   "rose",
			Count:  unstarted,
		})
	}
	if approvals.OverSLA > 0 {
		age := "—"
		if approvals.OldestAgeHours != nil {
			age = fmt.Sprintf("รายการเก่าสุด %.0f ชั่วโมง", *approvals.OldestAgeHours)
		}
		out = append(out, DashboardDecision{
			Key:    "approvals_over_sla",
			Title:  "รายการรออนุมัติเกิน SLA",
			Detail: fmt.Sprintf("%d รายการ · %s", approvals.OverSLA, age),
			Tone:   "amber",
			Count:  approvals.OverSLA,
		})
	}
	if pmPlanned > 0 && pmPercent < DashboardPMPlanTarget {
		out = append(out, DashboardDecision{
			Key:    "pm_below_target",
			Title:  "ผล PM ต่ำกว่าเป้าหมาย",
			Detail: fmt.Sprintf("สำเร็จ %.1f%% จากเป้าหมาย %.0f%%", pmPercent, DashboardPMPlanTarget),
			Tone:   "blue",
			Count:  1,
		})
	}
	return out
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func fallbackStatus(status string) string {
	switch status {
	case domain.PanelStatusAbnormal, domain.PanelStatusMonitoring, domain.PanelStatusNormal:
		return status
	default:
		return domain.PanelStatusNormal
	}
}

func round1Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	x := round1(*v)
	return &x
}
