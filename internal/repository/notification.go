package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// NotificationRepository reads and writes rtu.notifications (System Design
// Screen 06).
type NotificationRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var notificationSortable = httpx.Sortable{
	"created_at": "n.created_at",
	"is_read":    "n.is_read",
	"type":       "n.type",
}

// NotificationSortable lists the sort keys accepted by the list endpoint.
func NotificationSortable() httpx.Sortable { return notificationSortable }

// NotificationFilter narrows a notification list query to one recipient.
type NotificationFilter struct {
	RecipientID uuid.UUID
	IsRead      *bool
	Type        *string
}

// NotificationListItem is a notification row with the total row count.
type NotificationListItem struct {
	sqlc.Notification
	TotalCount int64 `db:"total_count" json:"-"`
}

const notificationListSelect = `
SELECT
    n.id, n.work_order_id, n.recipient_id, n.type, n.title, n.message,
    n.is_read, n.read_at, n.created_at, n.updated_at, n.created_by, n.updated_by,
    count(*) OVER ()::bigint AS total_count
FROM rtu.notifications n
WHERE %s
ORDER BY %s %s, n.id %s
LIMIT %s OFFSET %s`

// List returns one page of a recipient's notifications.
func (r *NotificationRepository) List(ctx context.Context, page httpx.Page, filter NotificationFilter) ([]NotificationListItem, int64, error) {
	a := &args{}
	conds := conditions{"n.recipient_id = " + a.add(filter.RecipientID)}
	if filter.IsRead != nil {
		conds = append(conds, "n.is_read = "+a.add(*filter.IsRead))
	}
	if filter.Type != nil {
		conds = append(conds, "n.type = "+a.add(*filter.Type))
	}

	query := fmt.Sprintf(notificationListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[NotificationListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// Get returns a single notification by id.
func (r *NotificationRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.Notification, error) {
	n, err := r.q.GetNotification(ctx, id)
	if err != nil {
		return sqlc.Notification{}, db.Translate(err, db.WithNotFound(httpx.ErrNotificationNotFnd))
	}
	return n, nil
}

// Create inserts a notification for a work order event.
func (r *NotificationRepository) Create(ctx context.Context, arg sqlc.CreateNotificationParams) (sqlc.Notification, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	n, err := r.q.CreateNotification(ctx, arg)
	if err != nil {
		return sqlc.Notification{}, db.Translate(err, db.Options{Constraints: db.Constraints{
			"fk_notifications_work_order": httpx.ErrWorkOrderNotFnd,
		}})
	}
	return n, nil
}

// MarkRead marks one notification as read, scoped to its recipient so a
// caller cannot mark someone else's notification.
func (r *NotificationRepository) MarkRead(ctx context.Context, id, recipientID uuid.UUID) (sqlc.Notification, error) {
	n, err := r.q.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{
		ID: id, RecipientID: recipientID, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.Notification{}, db.Translate(err, db.WithNotFound(httpx.ErrNotificationNotFnd))
	}
	return n, nil
}

// MarkAllRead marks every unread notification of a recipient as read and
// returns how many were updated.
func (r *NotificationRepository) MarkAllRead(ctx context.Context, recipientID uuid.UUID) (int64, error) {
	affected, err := r.q.MarkAllNotificationsRead(ctx, sqlc.MarkAllNotificationsReadParams{
		RecipientID: recipientID, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return 0, db.Translate(err)
	}
	return affected, nil
}

// CountUnread returns how many unread notifications a recipient has.
func (r *NotificationRepository) CountUnread(ctx context.Context, recipientID uuid.UUID) (int64, error) {
	total, err := r.q.CountUnreadNotifications(ctx, recipientID)
	if err != nil {
		return 0, db.Translate(err)
	}
	return total, nil
}

// Delete removes a notification, scoped to its recipient.
func (r *NotificationRepository) Delete(ctx context.Context, id, recipientID uuid.UUID) error {
	affected, err := r.q.DeleteNotification(ctx, sqlc.DeleteNotificationParams{ID: id, RecipientID: recipientID})
	if err != nil {
		return db.Translate(err)
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrNotificationNotFnd)
	}
	return nil
}
