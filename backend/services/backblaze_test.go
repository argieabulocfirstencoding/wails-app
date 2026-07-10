package services

import (
	"path/filepath"
	"testing"
)

func TestBuildRemoteObjectPathIncludesParentFolder(t *testing.T) {
	localPath := filepath.Join("screenshots", "171717", "shot_display0.webp")

	got := buildRemoteObjectPath("user-1", "", 0, localPath)
	want := "screenshots/user-1/171717/shot_display0.webp"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildRemoteObjectPathFallsBackToFilename(t *testing.T) {
	got := buildRemoteObjectPath("user-1", "", 0, "shot_display0.webp")
	want := "screenshots/user-1/shot_display0.webp"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildRemoteObjectPathUsesExplicitFolderAndScreenshotID(t *testing.T) {
	localPath := filepath.Join("screenshots", "171717", "task-1", "shot_display0.webp")

	got := buildRemoteObjectPath("user-1", "productivity-123", 9, localPath)
	want := "screenshots/user-1/productivity-123/9-shot_display0.webp"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildRemoteObjectPathSanitizesExplicitFolder(t *testing.T) {
	got := buildRemoteObjectPath("user-1", "prod/../42", 7, "shot_display0.webp")
	want := "screenshots/user-1/prod42/7-shot_display0.webp"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
