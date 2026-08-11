package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// AttachmentHandler serves the polymorphic attachment endpoints:
// /{entity}/{id}/attachments and the standalone /attachments/{id}.
type AttachmentHandler struct {
	svc *service.AttachmentService
}

// ListByEntity handles GET .../attachments for any parent entity route.
func (h *AttachmentHandler) ListByEntity(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID, err := httpx.UUIDParam(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}

		items, err := h.svc.ListByEntity(r.Context(), entityType, entityID)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}

		httpx.Success(w, r, httpx.SuccessList, httpx.NewCollection(items))
	}
}

// CreateForEntity handles POST .../attachments for any parent entity route.
func (h *AttachmentHandler) CreateForEntity(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID, err := httpx.UUIDParam(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}

		in, err := parseAttachmentUpload(r, entityType, entityID)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		defer in.Body.Close()

		att, err := h.svc.Upload(r.Context(), in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}

		httpx.Success(w, r, httpx.SuccessCreate, att)
	}
}

// Get handles GET /attachments/{id}.
func (h *AttachmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	att, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, att)
}

// attachmentCaptionInput is the PATCH /attachments/{id} body.
type attachmentCaptionInput struct {
	Caption *string `json:"caption" validate:"omitempty,max=2000"`
}

// Update handles PATCH /attachments/{id} (caption only — the file itself is
// immutable; delete and re-upload to replace it).
func (h *AttachmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var in attachmentCaptionInput
	if _, err := httpx.Bind(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	att, err := h.svc.UpdateCaption(r.Context(), id, in.Caption)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, att)
}

// Delete handles DELETE /attachments/{id}.
func (h *AttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: id, Deleted: true})
}

func parseAttachmentUpload(r *http.Request, entityType string, entityID uuid.UUID) (service.AttachmentUploadInput, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return service.AttachmentUploadInput{}, httpx.Err(httpx.ErrInvalidBody).WithCause(err)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return service.AttachmentUploadInput{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("file", httpx.IssueRequired, "File is required.")
	}

	createdBy, err := uuid.Parse(strings.TrimSpace(r.FormValue("created_by")))
	if err != nil {
		return service.AttachmentUploadInput{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("created_by", httpx.IssueRequired, "This field is required and must be a UUID.")
	}

	in := service.AttachmentUploadInput{
		EntityType:  entityType,
		EntityID:    entityID,
		CreatedBy:   createdBy,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
		Body:        file,
	}

	if caption, ok := r.MultipartForm.Value["caption"]; ok && len(caption) > 0 {
		c := caption[0]
		in.Caption = &c
	}

	return in, nil
}
