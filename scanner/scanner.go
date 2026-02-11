package scanner

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
)

// excludeDirs are Immich-internal directories that should be skipped during
// scanning. These contain generated thumbnails, transcoded video, backups, and
// profile data — none of which appear in the asset originalPath list.
var excludeDirs = map[string]struct{}{
	"thumbs":        {},
	"encoded-video": {},
	"backups":       {},
	"profile":       {},
}

// ScanFiles walks libraryPath and returns all file paths relative to it,
// using forward slashes to match Immich's originalPath format.
// Immich-internal directories (thumbs, encoded-video, backups, profile) are
// automatically excluded.
func ScanFiles(ctx context.Context, libraryPath string, logger *slog.Logger) ([]string, error) {
	var files []string

	libraryPath = filepath.Clean(libraryPath)

	err := filepath.WalkDir(libraryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("error accessing path", "path", path, "error", err)
			return nil // skip but continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			// Skip excluded top-level directories.
			if path != libraryPath {
				rel, relErr := filepath.Rel(libraryPath, path)
				if relErr == nil {
					topDir := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
					if _, excluded := excludeDirs[topDir]; excluded {
						logger.Debug("skipping excluded directory", "dir", topDir)
						return filepath.SkipDir
					}
				}
			}
			return nil
		}

		rel, err := filepath.Rel(libraryPath, path)
		if err != nil {
			logger.Warn("cannot compute relative path", "path", path, "error", err)
			return nil
		}

		// Normalize to forward slashes to match Immich's originalPath.
		rel = filepath.ToSlash(rel)

		files = append(files, rel)
		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Info("filesystem scan complete",
		"library_path", libraryPath,
		"files_found", len(files),
	)
	return files, nil
}

// ScanFilesWithPrefix walks libraryPath and returns paths with the given
// prefix prepended, using forward slashes. This is useful when Immich stores
// paths like "upload/library/admin/..." and libraryPath points to the parent
// of "upload/".
func ScanFilesWithPrefix(ctx context.Context, libraryPath, prefix string, logger *slog.Logger) ([]string, error) {
	files, err := ScanFiles(ctx, libraryPath, logger)
	if err != nil {
		return nil, err
	}

	if prefix == "" {
		return files, nil
	}

	prefix = strings.TrimRight(prefix, "/") + "/"
	for i, f := range files {
		files[i] = prefix + f
	}
	return files, nil
}
