package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/middleware"
)

// AuditLogRepository reads and writes rtu.audit_logs.
type AuditLogRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var auditLogSortable = httpx.Sortable{
	"created_at": "a.created_at",
	"action":     "a.action",
	"method":     "a.method",
	"status":     "a.status_code",
}

// AuditLogSortable lists the sort keys accepted by GET /audit-logs.
func AuditLogSortable() httpx.Sortable { return auditLogSortable }

// AuditLogFilter narrows the audit list.
type AuditLogFilter struct {
	UserID   *uuid.UUID
	Action   *string
	Resource *string
	From     *time.Time
	To       *time.Time
}

const auditListSelect = `
SELECT
    a.id, a.user_id, a.action, a.method, a.path, a.resource, a.resource_id,
    a.status_code, a.ip_address, a.user_agent, a.request_id, a.created_at,
    u.employee_code AS actor_employee_code,
    u.first_name    AS actor_first_name,
    u.last_name     AS actor_last_name,
    count(*) OVER ()::bigint AS total_count
FROM rtu.audit_logs a
LEFT JOIN rtu.users u ON u.id = a.user_id
WHERE %s
ORDER BY %s %s, a.id %s
LIMIT %s OFFSET %s`

// AuditLogListItem is one audit row with the actor's name when known.
type AuditLogListItem struct {
	sqlc.AuditLog
	ActorEmployeeCode *string `db:"actor_employee_code" json:"actor_employee_code"`
	ActorFirstName    *string `db:"actor_first_name" json:"actor_first_name"`
	ActorLastName     *string `db:"actor_last_name" json:"actor_last_name"`
	TotalCount        int64   `db:"total_count" json:"-"`
}

// WriteAudit implements middleware.AuditSink.
func (r *AuditLogRepository) WriteAudit(ctx context.Context, e middleware.AuditEntry) error {
	status := int32(e.StatusCode)
	_, err := r.q.CreateAuditLog(ctx, sqlc.CreateAuditLogParams{
		UserID:     e.UserID,
		Action:     e.Action,
		Method:     e.Method,
		Path:       e.Path,
		Resource:   e.Resource,
		ResourceID: e.ResourceID,
		StatusCode: &status,
		IpAddress:  optionalStr(e.IP),
		UserAgent:  optionalStr(e.UserAgent),
		RequestID:  optionalStr(e.RequestID),
	})
	if err != nil {
		return db.Translate(err)
	}
	return nil
}

// Record is a synchronous audit write used by the auth service for login events.
func (r *AuditLogRepository) Record(ctx context.Context, e middleware.AuditEntry) error {
	return r.WriteAudit(ctx, e)
}

// List returns one page of audit events.
func (r *AuditLogRepository) List(ctx context.Context, page httpx.Page, filter AuditLogFilter) ([]AuditLogListItem, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.UserID != nil {
		conds = append(conds, "a.user_id = "+a.add(*filter.UserID))
	}
	if filter.Action != nil {
		conds = append(conds, "a.action = "+a.add(*filter.Action))
	}
	if filter.Resource != nil {
		conds = append(conds, "a.resource = "+a.add(*filter.Resource))
	}
	if filter.From != nil {
		conds = append(conds, "a.created_at >= "+a.add(*filter.From))
	}
	if filter.To != nil {
		conds = append(conds, "a.created_at < "+a.add(*filter.To))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(`(a.path ILIKE %s ESCAPE '\' OR a.action ILIKE %s ESCAPE '\')`, p, p))
	}

	query := fmt.Sprintf(auditListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[AuditLogListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

func optionalStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
