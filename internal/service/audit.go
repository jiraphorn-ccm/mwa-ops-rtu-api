package service

import (
	"context"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// AuditService lists recorded HTTP / auth actions.
type AuditService struct {
	repo *repository.AuditLogRepository
}

// List returns one page of audit events.
func (s *AuditService) List(ctx context.Context, page httpx.Page, filter repository.AuditLogFilter) ([]repository.AuditLogListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}
