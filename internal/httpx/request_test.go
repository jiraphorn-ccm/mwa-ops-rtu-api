package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

type patchShape struct {
	Title *string `json:"title"`
}

func TestBindLenientIgnoresUnknownFields(t *testing.T) {
	body := `{
		"title": "updated",
		"status": "ASSIGNED",
		"work_order_type": "PM",
		"panel_id": "9fb86a82-e73f-41e7-88cf-a83afe8e578d"
	}`
	req := httptest.NewRequest("PATCH", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var in patchShape
	fields, err := BindLenient(req, &in)
	if err != nil {
		t.Fatal(err)
	}
	if in.Title == nil || *in.Title != "updated" {
		t.Fatalf("title=%v", in.Title)
	}
	if !fields.Has("status") {
		t.Fatal("FieldSet should still record keys the client sent")
	}
}

func TestBindRejectsUnknownFields(t *testing.T) {
	body := `{"title": "x", "status": "ASSIGNED"}`
	req := httptest.NewRequest("PATCH", "/", strings.NewReader(body))

	var in patchShape
	_, err := Bind(req, &in)
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*AppError)
	if !ok || appErr.Code.Code != ErrUnknownFields.Code {
		t.Fatalf("got %#v", err)
	}
}
