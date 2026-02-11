package matcher

import (
	"log/slog"
)

// UntrackedFile represents a file on disk that is not tracked by Immich.
type UntrackedFile struct {
	// RelPath is the relative path of the untracked file (forward-slash separated).
	RelPath string
}

// FindUntracked compares filesystem paths against Immich asset paths and returns
// files that are not tracked by Immich.
//
// diskFiles: relative paths from the filesystem scan (forward-slash normalized).
// assetPaths: set of originalPath values from Immich.
func FindUntracked(diskFiles []string, assetPaths map[string]struct{}, logger *slog.Logger) []UntrackedFile {
	var untracked []UntrackedFile

	for _, relPath := range diskFiles {
		if _, tracked := assetPaths[relPath]; !tracked {
			untracked = append(untracked, UntrackedFile{RelPath: relPath})
			logger.Debug("found untracked file", "path", relPath)
		}
	}

	logger.Info("matching complete", "untracked_found", len(untracked))
	return untracked
}
