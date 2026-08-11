package service

import (
	"context"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
	"github.com/rtu-api/internal/storage"
)

// maxAttachmentBytes matches ck_attachments_file_size in migration 000006.
const maxAttachmentBytes = 10 << 20 // 10 MB

// AttachmentEntityTypes lists the values accepted for entity_type, matching
// ck_attachments_entity_type.
const AttachmentEntityTypes = "WORK_ORDER PM_REPORT CM_REPORT CALIBRATION PM_GROUND_TEST PM_POWER_TEST_POINT PANEL_DEVICE"

// attachmentExtByMime falls back to a fixed extension when the client sends
// a bare mime type (e.g. multipart uploads from mobile clients often omit
// filenames). Anything else keeps the extension from the uploaded filename.
var attachmentExtByMime = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"application/pdf": ".pdf",
}

// AttachmentService manages polymorphic file uploads in S3 for rtu.attachments.
type AttachmentService struct {
	repo      *repository.AttachmentRepository
	s3        *storage.S3Client
	appPrefix string
}

// AttachmentView is an attachment enriched with a presigned URL.
type AttachmentView struct {
	sqlc.Attachment
	URL string `json:"url"`
}

// AttachmentUploadInput carries the multipart upload payload for one file.
// CreatedBy is required (rtu.attachments.created_by is NOT NULL) and is a
// plain form field, matching the actor_id convention used by the work order
// endpoints rather than relying on auth middleware.
type AttachmentUploadInput struct {
	EntityType  string
	EntityID    uuid.UUID
	CreatedBy   uuid.UUID
	Caption     *string
	Filename    string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

// ListByEntity returns every attachment of one entity.
func (s *AttachmentService) ListByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]AttachmentView, error) {
	entityType = strings.ToUpper(strings.TrimSpace(entityType))
	items, err := s.repo.ListByEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	out := make([]AttachmentView, len(items))
	for i, item := range items {
		out[i] = s.toView(ctx, item)
	}
	return out, nil
}

// Get returns one attachment with a presigned URL.
func (s *AttachmentService) Get(ctx context.Context, id uuid.UUID) (AttachmentView, error) {
	att, err := s.repo.Get(ctx, id)
	if err != nil {
		return AttachmentView{}, err
	}
	return s.toView(ctx, att), nil
}

// Upload stores a new file in S3 and records its metadata, after checking
// that the referenced entity actually exists.
func (s *AttachmentService) Upload(ctx context.Context, in AttachmentUploadInput) (AttachmentView, error) {
	if s.s3 == nil || !s.s3.Configured() {
		return AttachmentView{}, httpx.Err(httpx.ErrS3NotConfigured)
	}
	if in.CreatedBy == uuid.Nil {
		return AttachmentView{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("created_by", httpx.IssueRequired, "This field is required.")
	}

	entityType := strings.ToUpper(strings.TrimSpace(in.EntityType))
	if !isValidAttachmentEntityType(entityType) {
		return AttachmentView{}, httpx.Err(httpx.ErrAttachmentEntityInvalid)
	}

	found, err := s.repo.EntityExists(ctx, entityType, in.EntityID)
	if err != nil {
		return AttachmentView{}, err
	}
	if !found {
		return AttachmentView{}, httpx.Err(httpx.ErrAttachmentEntityInvalid).
			WithField("entity_id", httpx.IssueInvalid, "No matching record for this entity_type.")
	}

	mimeType, ext, err := validateAttachmentFile(in.Filename, in.ContentType, in.Size)
	if err != nil {
		return AttachmentView{}, err
	}

	objectID := uuid.New()
	key := storage.AttachmentKey(s.appPrefix, entityType, in.EntityID.String(), objectID.String(), ext)

	if err := s.s3.Upload(ctx, key, mimeType, in.Body); err != nil {
		return AttachmentView{}, httpx.Err(httpx.ErrInternal).WithCause(err)
	}

	att, err := s.repo.Create(ctx, sqlc.CreateAttachmentParams{
		EntityType:   entityType,
		EntityID:     in.EntityID,
		S3Bucket:     s.s3.Bucket(),
		S3Key:        key,
		OriginalName: optionalTrimmed(in.Filename),
		MimeType:     mimeType,
		FileSize:     in.Size,
		Caption:      in.Caption,
		CreatedBy:    in.CreatedBy,
	})
	if err != nil {
		_ = s.s3.Delete(ctx, key)
		return AttachmentView{}, err
	}
	return s.toView(ctx, att), nil
}

// UpdateCaption changes the caption of an attachment.
func (s *AttachmentService) UpdateCaption(ctx context.Context, id uuid.UUID, caption *string) (AttachmentView, error) {
	att, err := s.repo.UpdateCaption(ctx, id, caption)
	if err != nil {
		return AttachmentView{}, err
	}
	return s.toView(ctx, att), nil
}

// Delete removes the DB row and the S3 object.
func (s *AttachmentService) Delete(ctx context.Context, id uuid.UUID) error {
	if s.s3 == nil || !s.s3.Configured() {
		return httpx.Err(httpx.ErrS3NotConfigured)
	}
	att, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.s3.Delete(ctx, att.S3Key); err != nil {
		return httpx.Err(httpx.ErrInternal).WithCause(err)
	}
	return nil
}

func (s *AttachmentService) toView(ctx context.Context, att sqlc.Attachment) AttachmentView {
	view := AttachmentView{Attachment: att}
	if s.s3 != nil && s.s3.Configured() {
		if url, err := s.s3.PresignGet(ctx, att.S3Key); err == nil {
			view.URL = url
		}
	}
	return view
}

func isValidAttachmentEntityType(t string) bool {
	for _, v := range strings.Fields(AttachmentEntityTypes) {
		if v == t {
			return true
		}
	}
	return false
}

func validateAttachmentFile(filename, contentType string, size int64) (mimeType, ext string, err error) {
	if size <= 0 {
		return "", "", httpx.Err(httpx.ErrValidationFailed).
			WithField("file", httpx.IssueRequired, "File is required.")
	}
	if size > maxAttachmentBytes {
		return "", "", httpx.Err(httpx.ErrAttachmentTooLarge)
	}

	mimeType = strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ext = attachmentExtByMime[mimeType]
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(filename))
	}
	return mimeType, ext, nil
}
