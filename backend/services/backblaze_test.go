package services

import (
	"path/filepath"
	"testing"
)

func TestBuildRemoteObjectPathIncludesParentFolder(t *testing.T) {
	localPath := filepath.Join("screenshots", "171717", "shot_display0.webp")

	got := buildRemoteObjectPath("user-1", localPath)
	want := "screenshots/user-1/171717/shot_display0.webp"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildRemoteObjectPathFallsBackToFilename(t *testing.T) {
	got := buildRemoteObjectPath("user-1", "shot_display0.webp")
	want := "screenshots/user-1/shot_display0.webp"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
