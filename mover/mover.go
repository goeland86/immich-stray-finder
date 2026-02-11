package mover

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// MoveOrphans relocates orphan files from libraryPath to targetDir,
// preserving directory structure. If dryRun is true, only logs what
// would be moved without actually moving anything.
//
// relPaths are forward-slash relative paths (matching Immich's originalPath).
func MoveOrphans(relPaths []string, libraryPath, targetDir string, dryRun bool, logger *slog.Logger) error {
	for _, relPath := range relPaths {
		// Convert forward-slash relative path to OS path.
		srcRel := filepath.FromSlash(relPath)
		src := filepath.Join(libraryPath, srcRel)
		dst := filepath.Join(targetDir, srcRel)

		if dryRun {
			logger.Info("[dry-run] would move", "src", src, "dst", dst)
			continue
		}

		if err := moveFile(src, dst, logger); err != nil {
			logger.Error("failed to move file", "src", src, "dst", dst, "error", err)
			return fmt.Errorf("move %s -> %s: %w", src, dst, err)
		}

		logger.Info("moved file", "src", src, "dst", dst)
	}
	return nil
}

// moveFile moves src to dst. It tries os.Rename first for efficiency,
// falling back to copy+delete for cross-device moves.
func moveFile(src, dst string, logger *slog.Logger) error {
	// Ensure destination directory exists.
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dstDir, err)
	}

	// Try rename first (same filesystem).
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	logger.Debug("rename failed, falling back to copy+delete",
		"src", src, "dst", dst, "error", err,
	)

	// Fallback: copy then delete.
	if err := copyFile(src, dst); err != nil {
		return err
	}

	return os.Remove(src)
}

// copyFile copies src to dst, preserving file permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return dstFile.Close()
}
