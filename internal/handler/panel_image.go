package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/service"
)

// PanelImageHandler serves /panels/{id}/images.
type PanelImageHandler struct {
	svc *service.PanelImageService
}

// List handles GET /panels/{id}/images.
func (h *PanelImageHandler) List(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	page, filter, err := service.ParsePanelImageList(r, panelID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.svc.List(r.Context(), page, filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessList, httpx.NewList(items, page, total))
}

// Get handles GET /panels/{id}/images/{imageId}.
func (h *PanelImageHandler) Get(w http.ResponseWriter, r *http.Request) {
	panelID, imageID, err := h.panelImageIDs(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	img, err := h.svc.Get(r.Context(), panelID, imageID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDetail, img)
}

// Create handles POST /panels/{id}/images.
func (h *PanelImageHandler) Create(w http.ResponseWriter, r *http.Request) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	in, err := parseImageUpload(r, panelID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if strings.TrimSpace(in.ImageType) == "" {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("image_type", httpx.IssueRequired, "This field is required."))
		return
	}
	defer in.Body.Close()

	img, err := h.svc.Upload(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, img)
}

// Update handles PUT and PATCH /panels/{id}/images/{imageId}.
//
//   - PUT  + multipart/form-data → replace file (optional metadata fields)
//   - PUT  + application/json   → full metadata replace (image_type, sort_order required)
//   - PATCH + application/json  → partial metadata update
func (h *PanelImageHandler) Update(w http.ResponseWriter, r *http.Request) {
	panelID, imageID, err := h.panelImageIDs(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if r.Method != http.MethodPut {
			httpx.Error(w, r, httpx.Err(httpx.ErrMethodNotAllowed))
			return
		}
		in, err := parseImageUpload(r, panelID)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		defer in.Body.Close()

		img, err := h.svc.Replace(r.Context(), panelID, imageID, in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.Success(w, r, httpx.SuccessUpdate, img)
		return
	}

	var in service.PanelImageUpdateInput
	fields, err := httpx.Bind(r, &in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	img, err := h.svc.Update(r.Context(), panelID, imageID, fields, in, r.Method == http.MethodPut)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessUpdate, img)
}

// Delete handles DELETE /panels/{id}/images/{imageId}.
func (h *PanelImageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	panelID, imageID, err := h.panelImageIDs(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.Delete(r.Context(), panelID, imageID); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.Success(w, r, httpx.SuccessDelete, httpx.Removed{ID: imageID, Deleted: true})
}

func (h *PanelImageHandler) panelImageIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	panelID, err := httpx.UUIDParam(r, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	imageID, err := httpx.UUIDParam(r, "imageId")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return panelID, imageID, nil
}

func parseImageUpload(r *http.Request, panelID uuid.UUID) (service.UploadInput, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return service.UploadInput{}, httpx.Err(httpx.ErrInvalidBody).WithCause(err)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return service.UploadInput{}, httpx.Err(httpx.ErrValidationFailed).
			WithField("file", httpx.IssueRequired, "File is required.")
	}

	in := service.UploadInput{
		PanelID:     panelID,
		ImageType:   r.FormValue("image_type"),
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
		Body:        file,
	}

	if caption, ok := r.MultipartForm.Value["caption"]; ok && len(caption) > 0 {
		c := caption[0]
		in.Caption = &c
	}
	if sort := r.FormValue("sort_order"); sort != "" {
		n, err := strconv.ParseInt(sort, 10, 16)
		if err != nil {
			return service.UploadInput{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("sort_order", httpx.IssueInvalid, "Must be a small integer.")
		}
		v := int16(n)
		in.SortOrder = &v
	}

	return in, nil
}
