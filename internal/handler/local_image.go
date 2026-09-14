package handler

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
)

const maxLocalImageBytes = 10 << 20 // 10 MB

var localImageExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// LocalImageHandler stores uploads on the local disk under server/images.
// It does not use S3 — this exists to prove local storage independently.
type LocalImageHandler struct {
	dir string
}

// NewLocalImageHandler writes files into dir (created on first upload).
func NewLocalImageHandler(dir string) *LocalImageHandler {
	if strings.TrimSpace(dir) == "" {
		dir = "images"
	}
	return &LocalImageHandler{dir: dir}
}

type localImageResult struct {
	Filename    string `json:"filename"`
	StoredAt    string `json:"stored_at"`
	URL         string `json:"url"`
	Storage     string `json:"storage"`
	S3          bool   `json:"s3"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// Upload handles POST /local-images. The multipart field name is "file".
func (h *LocalImageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxLocalImageBytes); err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrInvalidBody).WithCause(err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("file", httpx.IssueRequired, "File is required."))
		return
	}
	defer file.Close()

	mimeType, ext, err := localImageType(header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrInternal).WithCause(err))
		return
	}

	filename := uuid.NewString() + ext
	absDir, err := filepath.Abs(h.dir)
	if err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrInternal).WithCause(err))
		return
	}
	dest := filepath.Join(absDir, filename)

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrInternal).WithCause(err))
		return
	}

	written, copyErr := io.Copy(out, io.LimitReader(file, maxLocalImageBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dest)
		cause := copyErr
		if cause == nil {
			cause = closeErr
		}
		httpx.Error(w, r, httpx.Err(httpx.ErrInternal).WithCause(cause))
		return
	}
	if written > maxLocalImageBytes {
		_ = os.Remove(dest)
		httpx.Error(w, r, httpx.Err(httpx.ErrImageTooLarge))
		return
	}
	if written == 0 {
		_ = os.Remove(dest)
		httpx.Error(w, r, httpx.Err(httpx.ErrValidationFailed).
			WithField("file", httpx.IssueRequired, "File is required."))
		return
	}

	httpx.Success(w, r, httpx.SuccessCreate, localImageResult{
		Filename:    filename,
		StoredAt:    dest,
		URL:         "/local-images/" + filename,
		Storage:     "local_disk",
		S3:          false,
		ContentType: mimeType,
		Size:        written,
	})
}

// Get handles GET /local-images/{filename} and streams the file from disk.
func (h *LocalImageHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if !safeLocalFilename(name) {
		httpx.Error(w, r, httpx.Err(httpx.ErrNotFound))
		return
	}

	absDir, err := filepath.Abs(h.dir)
	if err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrInternal).WithCause(err))
		return
	}
	path := filepath.Join(absDir, name)
	if _, err := os.Stat(path); err != nil {
		httpx.Error(w, r, httpx.Err(httpx.ErrNotFound))
		return
	}

	http.ServeFile(w, r, path)
}

func safeLocalFilename(name string) bool {
	if name == "" || name != filepath.Base(name) {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, allowed := range localImageExts {
		if ext == allowed {
			return true
		}
	}
	return false
}

func localImageType(filename, contentType string, size int64) (string, string, error) {
	if size > maxLocalImageBytes {
		return "", "", httpx.Err(httpx.ErrImageTooLarge)
	}

	mimeType := strings.ToLower(strings.TrimSpace(contentType))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	ext, ok := localImageExts[mimeType]
	if !ok {
		return "", "", httpx.Err(httpx.ErrImageMimeInvalid)
	}
	return mimeType, ext, nil
}
