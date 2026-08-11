package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// NotificationTypes lists the values accepted for type, matching
// ck_notifications_type (System Design Screen 06).
const NotificationTypes = "NEW_ASSIGNMENT PENDING_WORK PENDING_APPROVAL COMPLETED CM_PENDING"

// NotificationService applies the business rules of rtu.notifications.
type NotificationService struct {
	repo       *repository.NotificationRepository
	workOrders *repository.WorkOrderRepository
}

// NotificationCreateInput is the POST /notifications body.
type NotificationCreateInput struct {
	WorkOrderID uuid.UUID `json:"work_order_id" validate:"required"`
	RecipientID uuid.UUID `json:"recipient_id" validate:"required"`
	Type        string    `json:"type" validate:"required,oneof=NEW_ASSIGNMENT PENDING_WORK PENDING_APPROVAL COMPLETED CM_PENDING"`
	Title       *string   `json:"title" validate:"omitempty,max=255"`
	Message     *string   `json:"message"`
}

// List returns one page of a recipient's notifications.
func (s *NotificationService) List(ctx context.Context, page httpx.Page, filter repository.NotificationFilter) ([]repository.NotificationListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single notification.
func (s *NotificationService) Get(ctx context.Context, id uuid.UUID) (sqlc.Notification, error) {
	return s.repo.Get(ctx, id)
}

// Create raises a notification for a work order event. Emitted by the
// application layer (e.g. after assignment or submission); see
// WorkOrderService / ApprovalService for the workflow moments that could
// call this next.
func (s *NotificationService) Create(ctx context.Context, in NotificationCreateInput) (sqlc.Notification, error) {
	if _, err := s.workOrders.Get(ctx, in.WorkOrderID); err != nil {
		return sqlc.Notification{}, err
	}
	return s.repo.Create(ctx, sqlc.CreateNotificationParams{
		WorkOrderID: in.WorkOrderID,
		RecipientID: in.RecipientID,
		Type:        in.Type,
		Title:       in.Title,
		Message:     in.Message,
	})
}

// MarkRead marks one notification as read for its recipient.
func (s *NotificationService) MarkRead(ctx context.Context, id, recipientID uuid.UUID) (sqlc.Notification, error) {
	return s.repo.MarkRead(ctx, id, recipientID)
}

// MarkAllRead marks every unread notification of a recipient as read.
func (s *NotificationService) MarkAllRead(ctx context.Context, recipientID uuid.UUID) (int64, error) {
	return s.repo.MarkAllRead(ctx, recipientID)
}

// CountUnread returns how many unread notifications a recipient has.
func (s *NotificationService) CountUnread(ctx context.Context, recipientID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, recipientID)
}

// Delete removes a notification, scoped to its recipient.
func (s *NotificationService) Delete(ctx context.Context, id, recipientID uuid.UUID) error {
	return s.repo.Delete(ctx, id, recipientID)
}
