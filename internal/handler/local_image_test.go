package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/rtu-api/internal/httpx"
)

func TestLocalImageUpload_storesOnDiskNotS3(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	h := NewLocalImageHandler(dir)

	r := chi.NewRouter()
	r.Post("/", h.Upload)
	r.Get("/{filename}", h.Get)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "proof.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(part, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var env httpx.SuccessEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env.Data)
	var data localImageResult
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}

	if data.S3 || data.Storage != "local_disk" {
		t.Fatalf("expected local disk, got %+v", data)
	}
	if _, err := os.Stat(data.StoredAt); err != nil {
		t.Fatalf("file missing on disk: %v", err)
	}
	if filepath.Dir(data.StoredAt) != dir {
		t.Fatalf("stored_at dir=%q want %q", filepath.Dir(data.StoredAt), dir)
	}

	get := httptest.NewRequest(http.MethodGet, "/"+data.Filename, nil)
	got := httptest.NewRecorder()
	r.ServeHTTP(got, get)
	if got.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
	}
	if got.Body.Len() == 0 {
		t.Fatal("get returned empty body")
	}
	if _, err := io.ReadAll(got.Body); err != nil {
		t.Fatal(err)
	}
}
