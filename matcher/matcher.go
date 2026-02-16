package matcher

import (
	"log/slog"
	"path"
	"regexp"
	"strings"
)

// uuidRegex matches a standard UUID (8-4-4-4-12 hex digits).
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// UntrackedFile represents a file on disk that is not tracked by Immich.
type UntrackedFile struct {
	// RelPath is the relative path of the untracked file (forward-slash separated).
	RelPath string
}

// MatchContext holds all the data needed for directory-aware matching.
type MatchContext struct {
	// AssetPaths contains all originalPath values (prefix-stripped) from Immich.
	AssetPaths map[string]struct{}
	// AssetIDs contains all known asset UUIDs.
	AssetIDs map[string]struct{}
	// UserIDs contains all known user UUIDs.
	UserIDs map[string]struct{}
}

// FindUntracked compares filesystem paths against Immich data and returns
// files that are not tracked by Immich.
//
// diskFiles: relative paths from the filesystem scan (forward-slash normalized).
// mctx: match context containing asset paths, asset IDs, and user IDs.
func FindUntracked(diskFiles []string, mctx *MatchContext, logger *slog.Logger) []UntrackedFile {
	var untracked []UntrackedFile

	for _, relPath := range diskFiles {
		if !isKnown(relPath, mctx) {
			untracked = append(untracked, UntrackedFile{RelPath: relPath})
			logger.Debug("found untracked file", "path", relPath)
		}
	}

	logger.Info("matching complete", "untracked_found", len(untracked))
	return untracked
}

// isKnown dispatches by top-level directory to determine whether a file is
// tracked by Immich.
func isKnown(relPath string, mctx *MatchContext) bool {
	topDir := strings.SplitN(relPath, "/", 2)[0]

	switch topDir {
	case "library", "upload":
		// Exact path match against originalPath set.
		_, ok := mctx.AssetPaths[relPath]
		return ok

	case "thumbs", "encoded-video":
		// Extract asset UUID from filename.
		return matchByAssetID(relPath, mctx.AssetIDs)

	case "profile":
		// Extract user UUID from path.
		return matchByUserID(relPath, mctx.UserIDs)

	case ".immich":
		// Immich marker files are always considered known.
		return true

	default:
		// Unknown top-level directories are flagged as untracked.
		return false
	}
}

// matchByAssetID extracts a UUID from the filename and checks it against
// the set of known asset IDs. Thumbnail files are named like
// "{assetId}-thumbnail.webp" and encoded videos like "{assetId}.mp4".
func matchByAssetID(relPath string, assetIDs map[string]struct{}) bool {
	filename := path.Base(relPath)
	uuid := extractUUID(filename)
	if uuid == "" {
		return false
	}
	_, ok := assetIDs[uuid]
	return ok
}

// matchByUserID extracts a user UUID from the 2nd path segment and checks
// it against the set of known user IDs. Profile paths look like
// "profile/{userId}/profile-image.jpg".
func matchByUserID(relPath string, userIDs map[string]struct{}) bool {
	parts := strings.SplitN(relPath, "/", 3)
	if len(parts) < 2 {
		return false
	}
	userID := parts[1]
	if !isValidUUID(userID) {
		return false
	}
	_, ok := userIDs[userID]
	return ok
}

// extractUUID extracts a UUID from the beginning of a string. The UUID must
// be the first 36 characters and be valid. This handles filenames like
// "aaaaaaaa-1111-2222-3333-444444444444-thumbnail.webp" and
// "aaaaaaaa-1111-2222-3333-444444444444.mp4".
func extractUUID(s string) string {
	if len(s) < 36 {
		return ""
	}
	candidate := s[:36]
	if isValidUUID(candidate) {
		return candidate
	}
	return ""
}

// isValidUUID checks whether a string is a valid UUID (8-4-4-4-12 hex).
func isValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}
