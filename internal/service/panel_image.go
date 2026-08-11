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

const maxPanelImageBytes = 10 << 20 // 10 MB

var allowedImageMimes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// PanelImageTypes lists accepted image_type values.
const PanelImageTypes = "EXTERIOR INTERIOR DEVICE"

// PanelImageService manages panel photos in S3.
type PanelImageService struct {
	repo      *repository.PanelImageRepository
	panels    *repository.PanelRepository
	s3        *storage.S3Client
	appPrefix string
}

// PanelImageView is a panel image enriched with a presigned URL.
type PanelImageView struct {
	sqlc.PanelImage
	URL string `json:"url"`
}

// PanelImageUpdateInput is the PATCH body for caption / sort_order / image_type.
type PanelImageUpdateInput struct {
	ImageType *string `json:"image_type" validate:"omitempty,oneof=EXTERIOR INTERIOR DEVICE"`
	Caption   *string `json:"caption" validate:"omitempty,max=2000"`
	SortOrder *int16  `json:"sort_order"`
}

// UploadInput carries the multipart upload payload.
type UploadInput struct {
	PanelID     uuid.UUID
	ImageType   string
	Caption     *string
	SortOrder   *int16
	Filename    string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

// List returns one page of images for a panel.
func (s *PanelImageService) List(ctx context.Context, page httpx.Page, filter repository.PanelImageFilter) ([]PanelImageView, int64, error) {
	if _, err := s.panels.Get(ctx, filter.PanelID); err != nil {
		return nil, 0, err
	}

	items, total, err := s.repo.List(ctx, page, filter)
	if err != nil {
		return nil, 0, err
	}

	out := make([]PanelImageView, len(items))
	for i, item := range items {
		out[i] = s.toView(ctx, item.PanelImage)
	}
	return out, total, nil
}

// Get returns one image with a presigned URL.
func (s *PanelImageService) Get(ctx context.Context, panelID, id uuid.UUID) (PanelImageView, error) {
	img, err := s.repo.GetForPanel(ctx, panelID, id)
	if err != nil {
		return PanelImageView{}, err
	}
	return s.toView(ctx, img), nil
}

// Upload stores a new image in S3 and records metadata.
func (s *PanelImageService) Upload(ctx context.Context, in UploadInput) (PanelImageView, error) {
	if s.s3 == nil || !s.s3.Configured() {
		return PanelImageView{}, httpx.Err(httpx.ErrS3NotConfigured)
	}

	panel, err := s.panels.Get(ctx, in.PanelID)
	if err != nil {
		return PanelImageView{}, err
	}

	imageType, err := normalizeImageType(in.ImageType)
	if err != nil {
		return PanelImageView{}, err
	}

	mimeType, ext, err := validateImageFile(in.Filename, in.ContentType, in.Size)
	if err != nil {
		return PanelImageView{}, err
	}

	objectID := uuid.New()
	key := storage.PanelImageKey(s.appPrefix, panel.Code, objectID.String(), ext)

	if err := s.s3.Upload(ctx, key, mimeType, in.Body); err != nil {
		return PanelImageView{}, httpx.Err(httpx.ErrInternal).WithCause(err)
	}

	img, err := s.repo.Create(ctx, sqlc.CreatePanelImageParams{
		PanelID:      in.PanelID,
		ImageType:    imageType,
		S3Bucket:     s.s3.Bucket(),
		S3Key:        key,
		OriginalName: optionalTrimmed(in.Filename),
		MimeType:     mimeType,
		FileSize:     in.Size,
		Caption:      in.Caption,
		SortOrder:    in.SortOrder,
	})
	if err != nil {
		_ = s.s3.Delete(ctx, key)
		return PanelImageView{}, err
	}
	return s.toView(ctx, img), nil
}

// Replace swaps the file of an existing image: upload new, update DB, delete old S3 object.
func (s *PanelImageService) Replace(ctx context.Context, panelID, id uuid.UUID, in UploadInput) (PanelImageView, error) {
	if s.s3 == nil || !s.s3.Configured() {
		return PanelImageView{}, httpx.Err(httpx.ErrS3NotConfigured)
	}

	existing, err := s.repo.GetForPanel(ctx, panelID, id)
	if err != nil {
		return PanelImageView{}, err
	}

	panel, err := s.panels.Get(ctx, panelID)
	if err != nil {
		return PanelImageView{}, err
	}

	mimeType, ext, err := validateImageFile(in.Filename, in.ContentType, in.Size)
	if err != nil {
		return PanelImageView{}, err
	}

	objectID := uuid.New()
	newKey := storage.PanelImageKey(s.appPrefix, panel.Code, objectID.String(), ext)

	if err := s.s3.Upload(ctx, newKey, mimeType, in.Body); err != nil {
		return PanelImageView{}, httpx.Err(httpx.ErrInternal).WithCause(err)
	}

	img, err := s.repo.ReplaceFile(ctx, sqlc.ReplacePanelImageFileParams{
		ID:           id,
		PanelID:      panelID,
		S3Key:        newKey,
		MimeType:     mimeType,
		FileSize:     in.Size,
		OriginalName: optionalTrimmed(in.Filename),
	})
	if err != nil {
		_ = s.s3.Delete(ctx, newKey)
		return PanelImageView{}, err
	}

	if err := s.s3.Delete(ctx, existing.S3Key); err != nil {
		return PanelImageView{}, httpx.Err(httpx.ErrInternal).WithCause(err)
	}

	if in.ImageType != "" || in.Caption != nil || in.SortOrder != nil {
		params := sqlc.UpdatePanelImageParams{ID: id, PanelID: panelID}
		if in.ImageType != "" {
			t, err := normalizeImageType(in.ImageType)
			if err != nil {
				return PanelImageView{}, err
			}
			params.ImageTypeDoUpdate = true
			params.ImageType = t
		}
		if in.Caption != nil {
			params.CaptionDoUpdate = true
			params.Caption = in.Caption
		}
		if in.SortOrder != nil {
			params.SortOrderDoUpdate = true
			params.SortOrder = *in.SortOrder
		}
		updated, err := s.repo.Update(ctx, params)
		if err != nil {
			return PanelImageView{}, err
		}
		img = updated
	}

	return s.toView(ctx, img), nil
}

// Update applies metadata changes. fullReplace (HTTP PUT + JSON) requires image_type and sort_order.
func (s *PanelImageService) Update(ctx context.Context, panelID, id uuid.UUID, fields httpx.FieldSet, in PanelImageUpdateInput, fullReplace bool) (PanelImageView, error) {
	if fullReplace {
		if !fields.Has("image_type") || in.ImageType == nil {
			return PanelImageView{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("image_type", httpx.IssueRequired, "This field is required.")
		}
		if !fields.Has("sort_order") || in.SortOrder == nil {
			return PanelImageView{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("sort_order", httpx.IssueRequired, "This field is required.")
		}
	}

	params := sqlc.UpdatePanelImageParams{ID: id, PanelID: panelID}

	if fullReplace || fields.Has("image_type") {
		if in.ImageType == nil {
			return PanelImageView{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("image_type", httpx.IssueRequired, "This field is required.")
		}
		t, err := normalizeImageType(*in.ImageType)
		if err != nil {
			return PanelImageView{}, err
		}
		params.ImageTypeDoUpdate = true
		params.ImageType = t
	}
	if fullReplace || fields.Has("caption") {
		params.CaptionDoUpdate = true
		params.Caption = in.Caption
	}
	if fullReplace || fields.Has("sort_order") {
		if in.SortOrder == nil {
			return PanelImageView{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("sort_order", httpx.IssueRequired, "This field is required.")
		}
		params.SortOrderDoUpdate = true
		params.SortOrder = *in.SortOrder
	}

	img, err := s.repo.Update(ctx, params)
	if err != nil {
		return PanelImageView{}, err
	}
	return s.toView(ctx, img), nil
}

// Delete removes the DB row and the S3 object.
func (s *PanelImageService) Delete(ctx context.Context, panelID, id uuid.UUID) error {
	if s.s3 == nil || !s.s3.Configured() {
		return httpx.Err(httpx.ErrS3NotConfigured)
	}
	img, err := s.repo.Delete(ctx, panelID, id)
	if err != nil {
		return err
	}
	if err := s.s3.Delete(ctx, img.S3Key); err != nil {
		return httpx.Err(httpx.ErrInternal).WithCause(err)
	}
	return nil
}

func (s *PanelImageService) toView(ctx context.Context, img sqlc.PanelImage) PanelImageView {
	view := PanelImageView{PanelImage: img}
	if url, err := s.presign(ctx, img.S3Key); err == nil {
		view.URL = url
	}
	return view
}

func (s *PanelImageService) presign(ctx context.Context, key string) (string, error) {
	if s.s3 == nil || !s.s3.Configured() {
		return "", nil
	}
	return s.s3.PresignGet(ctx, key)
}

func normalizeImageType(raw string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	switch t {
	case "EXTERIOR", "INTERIOR", "DEVICE":
		return t, nil
	default:
		return "", httpx.Err(httpx.ErrImageTypeInvalid)
	}
}

func validateImageFile(filename, contentType string, size int64) (mimeType, ext string, err error) {
	if size <= 0 {
		return "", "", httpx.Err(httpx.ErrValidationFailed).
			WithField("file", httpx.IssueRequired, "File is required.")
	}
	if size > maxPanelImageBytes {
		return "", "", httpx.Err(httpx.ErrImageTooLarge)
	}

	mimeType = strings.ToLower(strings.TrimSpace(contentType))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	ext, ok := allowedImageMimes[mimeType]
	if !ok {
		return "", "", httpx.Err(httpx.ErrImageMimeInvalid)
	}
	return mimeType, ext, nil
}

func optionalTrimmed(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
