package mover

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestMoveOrphans_DryRun(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a source file.
	os.MkdirAll(filepath.Join(srcDir, "upload", "2024"), 0o755)
	srcFile := filepath.Join(srcDir, "upload", "2024", "photo.JPG")
	os.WriteFile(srcFile, []byte("photo data"), 0o644)

	relPaths := []string{"upload/2024/photo.JPG"}

	err := MoveOrphans(relPaths, srcDir, dstDir, true, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Source file should still exist in dry-run mode.
	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		t.Error("source file should still exist in dry-run mode")
	}

	// Destination file should NOT exist.
	dstFile := filepath.Join(dstDir, "upload", "2024", "photo.JPG")
	if _, err := os.Stat(dstFile); !os.IsNotExist(err) {
		t.Error("destination file should not exist in dry-run mode")
	}
}

func TestMoveOrphans_ActualMove(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a source file.
	os.MkdirAll(filepath.Join(srcDir, "upload", "2024"), 0o755)
	srcFile := filepath.Join(srcDir, "upload", "2024", "photo.JPG")
	content := []byte("photo data")
	os.WriteFile(srcFile, content, 0o644)

	relPaths := []string{"upload/2024/photo.JPG"}

	err := MoveOrphans(relPaths, srcDir, dstDir, false, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Source file should be gone.
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source file should have been removed")
	}

	// Destination file should exist with correct content.
	dstFile := filepath.Join(dstDir, "upload", "2024", "photo.JPG")
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("destination content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestMoveOrphans_PreservesDirectoryStructure(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create nested files.
	os.MkdirAll(filepath.Join(srcDir, "upload", "lib", "admin", "2024", "01"), 0o755)
	srcFile := filepath.Join(srcDir, "upload", "lib", "admin", "2024", "01", "img.JPG")
	os.WriteFile(srcFile, []byte("data"), 0o644)

	relPaths := []string{"upload/lib/admin/2024/01/img.JPG"}

	err := MoveOrphans(relPaths, srcDir, dstDir, false, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dstFile := filepath.Join(dstDir, "upload", "lib", "admin", "2024", "01", "img.JPG")
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("destination file should exist with preserved directory structure")
	}
}

func TestMoveOrphans_MultipleFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.MkdirAll(filepath.Join(srcDir, "a"), 0o755)
	os.MkdirAll(filepath.Join(srcDir, "b"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "a", "f1.JPG"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "b", "f2.PNG"), []byte("2"), 0o644)

	relPaths := []string{"a/f1.JPG", "b/f2.PNG"}

	err := MoveOrphans(relPaths, srcDir, dstDir, false, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, rel := range relPaths {
		dst := filepath.Join(dstDir, filepath.FromSlash(rel))
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", dst)
		}
	}
}
