package scanner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestScanFiles(t *testing.T) {
	// Create a temp directory structure.
	tmpDir := t.TempDir()

	// Create files. Note: we use distinct base names because Windows
	// filesystems are case-insensitive (photo1.jpg and photo1.JPG would
	// be the same file). The case-variant matching is tested in matcher/.
	dirs := []string{
		filepath.Join(tmpDir, "upload", "library", "admin", "2024"),
		filepath.Join(tmpDir, "upload", "library", "admin", "2023"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	files := []string{
		filepath.Join(tmpDir, "upload", "library", "admin", "2024", "photo1.jpg"),
		filepath.Join(tmpDir, "upload", "library", "admin", "2024", "photo2.png"),
		filepath.Join(tmpDir, "upload", "library", "admin", "2023", "video.mp4"),
	}
	for _, f := range files {
		os.WriteFile(f, []byte("test"), 0o644)
	}

	result, err := ScanFiles(context.Background(), tmpDir, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(result)

	expected := []string{
		"upload/library/admin/2023/video.mp4",
		"upload/library/admin/2024/photo1.jpg",
		"upload/library/admin/2024/photo2.png",
	}
	sort.Strings(expected)

	if len(result) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(result), result)
	}

	for i, e := range expected {
		if result[i] != e {
			t.Errorf("file %d: expected %q, got %q", i, e, result[i])
		}
	}
}

func TestScanFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ScanFiles(context.Background(), tmpDir, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 files, got %d", len(result))
	}
}

func TestScanFiles_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ScanFiles(ctx, tmpDir, testLogger())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestScanFiles_ExcludesImmichDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in excluded directories.
	excludedDirs := []string{"thumbs", "encoded-video", "backups", "profile"}
	for _, d := range excludedDirs {
		os.MkdirAll(filepath.Join(tmpDir, d, "sub"), 0o755)
		os.WriteFile(filepath.Join(tmpDir, d, "sub", "file.dat"), []byte("test"), 0o644)
	}

	// Create a file in a non-excluded directory.
	os.MkdirAll(filepath.Join(tmpDir, "upload", "library"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "upload", "library", "photo.jpg"), []byte("test"), 0o644)

	result, err := ScanFiles(context.Background(), tmpDir, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(result), result)
	}
	if result[0] != "upload/library/photo.jpg" {
		t.Errorf("expected %q, got %q", "upload/library/photo.jpg", result[0])
	}
}

func TestScanFilesWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file.txt"), []byte("test"), 0o644)

	result, err := ScanFilesWithPrefix(context.Background(), tmpDir, "prefix", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result))
	}
	if result[0] != "prefix/subdir/file.txt" {
		t.Errorf("expected %q, got %q", "prefix/subdir/file.txt", result[0])
	}
}
