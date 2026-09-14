package storage

import "testing"

func TestPanelImageKey(t *testing.T) {
	got := PanelImageKey("rtu", "PNL-001", "abc-123", ".jpg")
	want := "rtu/images/panels/PNL-001/abc-123.jpg"
	if got != want {
		t.Fatalf("PanelImageKey() = %q, want %q", got, want)
	}
}

func TestUserImageKey(t *testing.T) {
	got := UserImageKey("/rtu/", "PROFILE", "abc-123", ".png")
	want := "rtu/images/users/profile/abc-123.png"
	if got != want {
		t.Fatalf("UserImageKey() = %q, want %q", got, want)
	}
}
