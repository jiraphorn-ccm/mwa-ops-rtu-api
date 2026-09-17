package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/db"
)

const dashboardTZ = "Asia/Bangkok"

// DashboardRepository runs the aggregate queries behind GET /dashboard.
type DashboardRepository struct {
	pool *pgxpool.Pool
}

const dashboardOpenSQL = `'ASSIGNED','IN_PROGRESS','PENDING','PENDING_APPROVAL'`

const dashboardClosedSQL = `'COMPLETED','CONDITIONAL'`

// DashboardStationSnapshot is the live panel status rollup.
type DashboardStationSnapshot struct {
	Total    int64 `db:"total"`
	Online   int64 `db:"online"`
	Critical int64 `db:"critical"`
	Watch    int64 `db:"watch"`
	Areas    int64 `db:"areas"`
}

// DashboardPMPlan is PM work planned in a window versus completed on time.
type DashboardPMPlan struct {
	Planned         int64 `db:"planned"`
	CompletedOnPlan int64 `db:"completed_on_plan"`
}

// DashboardCMOpen is open CM work at a point in time.
type DashboardCMOpen struct {
	Count   int64 `db:"count"`
	OverSLA int64 `db:"over_sla"`
}

// DashboardApprovals is the pending-approval queue.
type DashboardApprovals struct {
	Count          int64    `db:"count"`
	OverSLA        int64    `db:"over_sla"`
	OldestAgeHours *float64 `db:"oldest_age_hours"`
}

// DashboardSLARow is SLA compliance for work orders in a window.
type DashboardSLARow struct {
	Within        int64    `db:"within_count"`
	Near          int64    `db:"near_count"`
	Over          int64    `db:"over_count"`
	AvgCloseHours *float64 `db:"avg_close_hours"`
}

// DashboardMaintenanceBucket is PM/CM throughput for one series bucket.
type DashboardMaintenanceBucket struct {
	Ord         int64 `db:"ord"`
	PMCompleted int64 `db:"pm_completed"`
	PMDelayed   int64 `db:"pm_delayed"`
	CMCompleted int64 `db:"cm_completed"`
	CMOpen      int64 `db:"cm_open"`
}

// DashboardAvailabilityBucket is the CM-incident availability proxy for one bucket.
type DashboardAvailabilityBucket struct {
	Ord          int64   `db:"ord"`
	Availability float64 `db:"availability"`
}

// DashboardStationRisk is one ranked station that needs attention.
type DashboardStationRisk struct {
	ID                uuid.UUID `db:"id"`
	Code              string    `db:"code"`
	Location          *string   `db:"location"`
	OperationalStatus string    `db:"status"`
	LatestIssue       *string   `db:"latest_issue"`
	OpenWorkOrders    int64     `db:"open_work_orders"`
	DowntimeSeconds   int64     `db:"downtime_seconds"`
}

// DashboardMapStation is an active panel with coordinates for the map.
type DashboardMapStation struct {
	ID                uuid.UUID        `db:"id"`
	Code              string           `db:"code"`
	Location          *string          `db:"location"`
	Latitude          *decimal.Decimal `db:"latitude"`
	Longitude         *decimal.Decimal `db:"longitude"`
	OperationalStatus string           `db:"status"`
}

const dashboardStationSnapshotSQL = `
SELECT
    count(*) FILTER (WHERE p.active)::bigint AS total,
    count(*) FILTER (WHERE p.active AND (` + panelStatusExprSQL + `) = 'NORMAL')::bigint AS online,
    count(*) FILTER (WHERE p.active AND (` + panelStatusExprSQL + `) = 'ABNORMAL')::bigint AS critical,
    count(*) FILTER (WHERE p.active AND (` + panelStatusExprSQL + `) = 'MONITORING')::bigint AS watch,
    count(DISTINCT btrim(p.location)) FILTER (
        WHERE p.active AND p.location IS NOT NULL AND btrim(p.location) <> ''
    )::bigint AS areas
FROM rtu.panels p`

const dashboardPMPlanSQL = `
SELECT
    count(*)::bigint AS planned,
    count(*) FILTER (
        WHERE wo.status IN (` + dashboardClosedSQL + `)
          AND (wo.due_date IS NULL OR (wo.closed_at AT TIME ZONE '` + dashboardTZ + `')::date <= wo.due_date)
    )::bigint AS completed_on_plan
FROM rtu.work_orders wo
WHERE wo.active
  AND wo.work_order_type = 'PM'
  AND wo.status <> 'CANCELLED'
  AND (
        (wo.planned_date IS NOT NULL
            AND wo.planned_date >= ($1 AT TIME ZONE '` + dashboardTZ + `')::date
            AND wo.planned_date <= ($2 AT TIME ZONE '` + dashboardTZ + `')::date)
     OR (wo.planned_date IS NULL AND wo.due_date IS NOT NULL
            AND wo.due_date >= ($1 AT TIME ZONE '` + dashboardTZ + `')::date
            AND wo.due_date <= ($2 AT TIME ZONE '` + dashboardTZ + `')::date)
     OR (wo.planned_date IS NULL AND wo.due_date IS NULL
            AND wo.created_at >= $1 AND wo.created_at < $2)
  )`

const dashboardCMOpenSQL = `
SELECT
    count(*)::bigint AS count,
    count(*) FILTER (
        WHERE wo.due_date IS NOT NULL
          AND wo.due_date < ($1 AT TIME ZONE '` + dashboardTZ + `')::date
    )::bigint AS over_sla
FROM rtu.work_orders wo
WHERE wo.active
  AND wo.work_order_type = 'CM'
  AND wo.status <> 'CANCELLED'
  AND wo.created_at < $1
  AND (wo.closed_at IS NULL OR wo.closed_at >= $1)`

const dashboardApprovalsSQL = `
SELECT
    count(*)::bigint AS count,
    count(*) FILTER (
        WHERE r.submitted_at IS NOT NULL
          AND r.submitted_at < now() - ($1::int * interval '1 hour')
    )::bigint AS over_sla,
    EXTRACT(EPOCH FROM (now() - min(r.submitted_at))) / 3600.0 AS oldest_age_hours
FROM rtu.work_orders wo
JOIN rtu.work_order_rounds r ON r.id = wo.current_round_id
WHERE wo.active
  AND wo.status = 'PENDING_APPROVAL'`

const dashboardSLASQL = `
SELECT
    count(*) FILTER (
        WHERE CASE
            WHEN wo.closed_at IS NOT NULL AND wo.status IN (` + dashboardClosedSQL + `)
                THEN (wo.closed_at AT TIME ZONE '` + dashboardTZ + `')::date <= wo.due_date
            WHEN wo.closed_at IS NULL
                THEN wo.due_date >= ($3 AT TIME ZONE '` + dashboardTZ + `')::date + $4::int
            ELSE FALSE
        END
    )::bigint AS within_count,
    count(*) FILTER (
        WHERE wo.closed_at IS NULL
          AND wo.due_date >= ($3 AT TIME ZONE '` + dashboardTZ + `')::date
          AND wo.due_date < ($3 AT TIME ZONE '` + dashboardTZ + `')::date + $4::int
    )::bigint AS near_count,
    count(*) FILTER (
        WHERE CASE
            WHEN wo.closed_at IS NOT NULL AND wo.status IN (` + dashboardClosedSQL + `)
                THEN (wo.closed_at AT TIME ZONE '` + dashboardTZ + `')::date > wo.due_date
            WHEN wo.closed_at IS NULL
                THEN wo.due_date < ($3 AT TIME ZONE '` + dashboardTZ + `')::date
            ELSE FALSE
        END
    )::bigint AS over_count,
    avg(EXTRACT(EPOCH FROM (wo.closed_at - wo.created_at)) / 3600.0)
        FILTER (WHERE wo.closed_at IS NOT NULL AND wo.closed_at >= $1 AND wo.closed_at < $2)
        AS avg_close_hours
FROM rtu.work_orders wo
WHERE wo.active
  AND wo.status <> 'CANCELLED'
  AND wo.due_date IS NOT NULL
  AND (
        (wo.closed_at IS NOT NULL AND wo.closed_at >= $1 AND wo.closed_at < $2)
     OR (wo.due_date >= ($1 AT TIME ZONE '` + dashboardTZ + `')::date
            AND wo.due_date <= ($2 AT TIME ZONE '` + dashboardTZ + `')::date)
     OR (wo.closed_at IS NULL AND wo.due_date <= ($2 AT TIME ZONE '` + dashboardTZ + `')::date)
  )`

const dashboardCriticalCMUnstartedSQL = `
SELECT count(*)::bigint
FROM rtu.work_orders wo
WHERE wo.active
  AND wo.work_order_type = 'CM'
  AND wo.priority = 'HIGH'
  AND wo.status = 'ASSIGNED'`

const dashboardMaintenanceSeriesSQL = `
SELECT b.ord,
    count(*) FILTER (
        WHERE wo.work_order_type = 'PM'
          AND wo.status IN (` + dashboardClosedSQL + `)
          AND wo.closed_at >= b.bucket_start AND wo.closed_at < b.bucket_end
          AND (wo.due_date IS NULL OR (wo.closed_at AT TIME ZONE '` + dashboardTZ + `')::date <= wo.due_date)
    )::bigint AS pm_completed,
    count(*) FILTER (
        WHERE wo.work_order_type = 'PM'
          AND wo.status <> 'CANCELLED'
          AND (
                (wo.status IN (` + dashboardClosedSQL + `)
                    AND wo.due_date IS NOT NULL
                    AND (wo.closed_at AT TIME ZONE '` + dashboardTZ + `')::date > wo.due_date
                    AND wo.closed_at >= b.bucket_start AND wo.closed_at < b.bucket_end)
             OR (wo.closed_at IS NULL
                    AND wo.due_date IS NOT NULL
                    AND wo.due_date >= (b.bucket_start AT TIME ZONE '` + dashboardTZ + `')::date
                    AND wo.due_date <= ((b.bucket_end - interval '1 second') AT TIME ZONE '` + dashboardTZ + `')::date
                    AND wo.due_date < (now() AT TIME ZONE '` + dashboardTZ + `')::date)
          )
    )::bigint AS pm_delayed,
    count(*) FILTER (
        WHERE wo.work_order_type = 'CM'
          AND wo.status IN (` + dashboardClosedSQL + `)
          AND wo.closed_at >= b.bucket_start AND wo.closed_at < b.bucket_end
    )::bigint AS cm_completed,
    count(*) FILTER (
        WHERE wo.work_order_type = 'CM'
          AND wo.status <> 'CANCELLED'
          AND wo.created_at < b.bucket_end
          AND (wo.closed_at IS NULL OR wo.closed_at >= b.bucket_end)
    )::bigint AS cm_open
FROM unnest($1::timestamptz[], $2::timestamptz[]) WITH ORDINALITY AS b(bucket_start, bucket_end, ord)
LEFT JOIN rtu.work_orders wo ON wo.active
GROUP BY b.ord
ORDER BY b.ord`

const dashboardAvailabilitySeriesSQL = `
SELECT b.ord,
    CASE
        WHEN $3::bigint <= 0 THEN 100
        ELSE ROUND((
            1 - (
                SELECT count(DISTINCT wo.panel_id)::numeric
                FROM rtu.work_orders wo
                WHERE wo.active
                  AND wo.work_order_type = 'CM'
                  AND wo.status <> 'CANCELLED'
                  AND wo.created_at < b.bucket_end
                  AND (wo.closed_at IS NULL OR wo.closed_at >= b.bucket_start)
            ) / $3::numeric
        ) * 1000) / 10
    END::float8 AS availability
FROM unnest($1::timestamptz[], $2::timestamptz[]) WITH ORDINALITY AS b(bucket_start, bucket_end, ord)
ORDER BY b.ord`

const dashboardStationsAtRiskSQL = `
SELECT
    p.id, p.code, p.location,
    (` + panelStatusExprSQL + `) AS status,
    (
        SELECT COALESCE(
            (
                SELECT pt.name
                FROM rtu.work_orders wo
                JOIN rtu.work_order_problem_topics wopt
                  ON wopt.work_order_id = wo.id
                JOIN rtu.problem_topics pt ON pt.id = wopt.problem_topic_id
                WHERE wo.panel_id = p.id
                  AND wo.active
                  AND wo.work_order_type = 'CM'
                  AND wo.status IN (` + dashboardOpenSQL + `)
                ORDER BY wo.created_at DESC, wopt.sort_order ASC
                LIMIT 1
            ),
            (
                SELECT NULLIF(btrim(wo.title), '')
                FROM rtu.work_orders wo
                WHERE wo.panel_id = p.id
                  AND wo.active
                  AND wo.work_order_type = 'CM'
                  AND wo.status IN (` + dashboardOpenSQL + `)
                ORDER BY wo.created_at DESC
                LIMIT 1
            )
        )
    ) AS latest_issue,
    (
        SELECT count(*)::bigint
        FROM rtu.work_orders wo
        WHERE wo.panel_id = p.id
          AND wo.active
          AND wo.status IN (` + dashboardOpenSQL + `)
    ) AS open_work_orders,
    GREATEST(0, EXTRACT(EPOCH FROM (
        now() - COALESCE(
            (
                SELECT min(pd.last_seen_at)
                FROM rtu.panel_devices pd
                WHERE pd.panel_id = p.id
                  AND pd.active
                  AND (pd.health_status = 'CRITICAL' OR pd.communication_status = 'OFFLINE')
                  AND pd.last_seen_at IS NOT NULL
            ),
            (
                SELECT min(wo.created_at)
                FROM rtu.work_orders wo
                WHERE wo.panel_id = p.id
                  AND wo.active
                  AND wo.work_order_type = 'CM'
                  AND wo.status IN (` + dashboardOpenSQL + `)
            )
        )
    )))::bigint AS downtime_seconds
FROM rtu.panels p
WHERE p.active
  AND (` + panelStatusExprSQL + `) IN ('ABNORMAL', 'MONITORING')
ORDER BY
    CASE (` + panelStatusExprSQL + `) WHEN 'ABNORMAL' THEN 0 ELSE 1 END,
    open_work_orders DESC,
    downtime_seconds DESC,
    p.code
LIMIT $1`

const dashboardMapStationsSQL = `
SELECT
    p.id, p.code, p.location, p.latitude, p.longitude,
    (` + panelStatusExprSQL + `) AS status
FROM rtu.panels p
WHERE p.active
  AND p.latitude IS NOT NULL
  AND p.longitude IS NOT NULL
ORDER BY p.code, p.id`

func (r *DashboardRepository) StationSnapshot(ctx context.Context) (DashboardStationSnapshot, error) {
	return collectOne[DashboardStationSnapshot](ctx, r.pool, dashboardStationSnapshotSQL)
}

func (r *DashboardRepository) PMPlan(ctx context.Context, from, to time.Time) (DashboardPMPlan, error) {
	return collectOne[DashboardPMPlan](ctx, r.pool, dashboardPMPlanSQL, from, to)
}

func (r *DashboardRepository) CMOpen(ctx context.Context, at time.Time) (DashboardCMOpen, error) {
	return collectOne[DashboardCMOpen](ctx, r.pool, dashboardCMOpenSQL, at)
}

func (r *DashboardRepository) PendingApprovals(ctx context.Context, slaHours int) (DashboardApprovals, error) {
	return collectOne[DashboardApprovals](ctx, r.pool, dashboardApprovalsSQL, slaHours)
}

func (r *DashboardRepository) SLA(ctx context.Context, from, to, now time.Time, nearDays int) (DashboardSLARow, error) {
	return collectOne[DashboardSLARow](ctx, r.pool, dashboardSLASQL, from, to, now, nearDays)
}

func (r *DashboardRepository) CriticalCMUnstarted(ctx context.Context) (int64, error) {
	var count int64
	if err := r.pool.QueryRow(ctx, dashboardCriticalCMUnstartedSQL).Scan(&count); err != nil {
		return 0, db.Translate(err)
	}
	return count, nil
}

func (r *DashboardRepository) MaintenanceSeries(ctx context.Context, starts, ends []time.Time) ([]DashboardMaintenanceBucket, error) {
	if len(starts) == 0 {
		return nil, nil
	}
	return collectMany[DashboardMaintenanceBucket](ctx, r.pool, dashboardMaintenanceSeriesSQL, starts, ends)
}

func (r *DashboardRepository) AvailabilitySeries(ctx context.Context, starts, ends []time.Time, totalStations int64) ([]DashboardAvailabilityBucket, error) {
	if len(starts) == 0 {
		return nil, nil
	}
	return collectMany[DashboardAvailabilityBucket](ctx, r.pool, dashboardAvailabilitySeriesSQL, starts, ends, totalStations)
}

func (r *DashboardRepository) StationsAtRisk(ctx context.Context, limit int) ([]DashboardStationRisk, error) {
	return collectMany[DashboardStationRisk](ctx, r.pool, dashboardStationsAtRiskSQL, limit)
}

func (r *DashboardRepository) MapStations(ctx context.Context) ([]DashboardMapStation, error) {
	return collectMany[DashboardMapStation](ctx, r.pool, dashboardMapStationsSQL)
}

func collectOne[T any](ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (T, error) {
	var zero T
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return zero, db.Translate(err)
	}
	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return zero, db.Translate(err)
	}
	return item, nil
}

func collectMany[T any](ctx context.Context, pool *pgxpool.Pool, query string, args ...any) ([]T, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, db.Translate(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, db.Translate(err)
	}
	if items == nil {
		items = []T{}
	}
	return items, nil
}
