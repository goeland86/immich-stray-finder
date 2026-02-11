package matcher

import (
	"log/slog"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestFindUntracked_AllTracked(t *testing.T) {
	assetPaths := map[string]struct{}{
		"upload/library/admin/2024/photo1.jpg": {},
		"upload/library/admin/2024/photo2.JPG": {},
		"upload/library/admin/2023/video.mp4":  {},
	}

	diskFiles := []string{
		"upload/library/admin/2024/photo1.jpg",
		"upload/library/admin/2024/photo2.JPG",
		"upload/library/admin/2023/video.mp4",
	}

	untracked := FindUntracked(diskFiles, assetPaths, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked, got %d", len(untracked))
	}
}

func TestFindUntracked_MixedTrackedAndUntracked(t *testing.T) {
	assetPaths := map[string]struct{}{
		"upload/library/admin/2024/photo1.jpg": {},
		"upload/library/admin/2023/video.mp4":  {},
	}

	diskFiles := []string{
		"upload/library/admin/2024/photo1.jpg", // tracked
		"upload/library/admin/2024/photo2.JPG", // untracked
		"upload/library/admin/2023/video.mp4",  // tracked
		"upload/library/admin/2023/extra.png",  // untracked
	}

	untracked := FindUntracked(diskFiles, assetPaths, testLogger())
	if len(untracked) != 2 {
		t.Fatalf("expected 2 untracked, got %d", len(untracked))
	}

	paths := make(map[string]bool)
	for _, u := range untracked {
		paths[u.RelPath] = true
	}

	if !paths["upload/library/admin/2024/photo2.JPG"] {
		t.Error("expected photo2.JPG to be untracked")
	}
	if !paths["upload/library/admin/2023/extra.png"] {
		t.Error("expected extra.png to be untracked")
	}
}

func TestFindUntracked_NoneTracked(t *testing.T) {
	assetPaths := map[string]struct{}{}

	diskFiles := []string{
		"upload/library/admin/2024/photo1.jpg",
		"upload/library/admin/2024/photo2.png",
	}

	untracked := FindUntracked(diskFiles, assetPaths, testLogger())
	if len(untracked) != 2 {
		t.Errorf("expected 2 untracked, got %d", len(untracked))
	}
}

func TestFindUntracked_EmptyInputs(t *testing.T) {
	// No disk files.
	untracked := FindUntracked(nil, map[string]struct{}{"a": {}}, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked for empty disk files, got %d", len(untracked))
	}

	// No asset paths.
	untracked = FindUntracked([]string{"a"}, map[string]struct{}{}, testLogger())
	if len(untracked) != 1 {
		t.Errorf("expected 1 untracked for empty asset paths, got %d", len(untracked))
	}

	// Both empty.
	untracked = FindUntracked(nil, map[string]struct{}{}, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked for both empty, got %d", len(untracked))
	}
}
