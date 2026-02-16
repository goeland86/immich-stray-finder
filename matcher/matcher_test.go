package matcher

import (
	"log/slog"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newMatchContext() *MatchContext {
	return &MatchContext{
		AssetPaths: make(map[string]struct{}),
		AssetIDs:   make(map[string]struct{}),
		UserIDs:    make(map[string]struct{}),
	}
}

func TestFindUntracked_LibraryExactMatch(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetPaths["library/admin/2024/photo1.jpg"] = struct{}{}
	mctx.AssetPaths["library/admin/2024/photo2.JPG"] = struct{}{}

	diskFiles := []string{
		"library/admin/2024/photo1.jpg",
		"library/admin/2024/photo2.JPG",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked, got %d: %v", len(untracked), untracked)
	}
}

func TestFindUntracked_LibraryUntracked(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetPaths["library/admin/2024/photo1.jpg"] = struct{}{}

	diskFiles := []string{
		"library/admin/2024/photo1.jpg",
		"library/admin/2024/stray.png",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 1 {
		t.Fatalf("expected 1 untracked, got %d", len(untracked))
	}
	if untracked[0].RelPath != "library/admin/2024/stray.png" {
		t.Errorf("expected stray.png, got %s", untracked[0].RelPath)
	}
}

func TestFindUntracked_UploadExactMatch(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetPaths["upload/library/admin/2024/photo1.jpg"] = struct{}{}

	diskFiles := []string{
		"upload/library/admin/2024/photo1.jpg",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked, got %d", len(untracked))
	}
}

func TestFindUntracked_ThumbsTrackedByAssetID(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetIDs["aaaaaaaa-1111-2222-3333-444444444444"] = struct{}{}
	mctx.AssetIDs["bbbbbbbb-1111-2222-3333-444444444444"] = struct{}{}

	diskFiles := []string{
		"thumbs/user-uuid/aaaaaaaa-1111-2222-3333-444444444444-thumbnail.webp",
		"thumbs/user-uuid/bbbbbbbb-1111-2222-3333-444444444444-preview.jpeg",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked, got %d: %v", len(untracked), untracked)
	}
}

func TestFindUntracked_ThumbsStray(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetIDs["aaaaaaaa-1111-2222-3333-444444444444"] = struct{}{}

	diskFiles := []string{
		"thumbs/user-uuid/aaaaaaaa-1111-2222-3333-444444444444-thumbnail.webp",
		"thumbs/user-uuid/cccccccc-1111-2222-3333-444444444444-thumbnail.webp",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 1 {
		t.Fatalf("expected 1 untracked, got %d", len(untracked))
	}
	if untracked[0].RelPath != "thumbs/user-uuid/cccccccc-1111-2222-3333-444444444444-thumbnail.webp" {
		t.Errorf("unexpected untracked: %s", untracked[0].RelPath)
	}
}

func TestFindUntracked_EncodedVideoTracked(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetIDs["aaaaaaaa-1111-2222-3333-444444444444"] = struct{}{}

	diskFiles := []string{
		"encoded-video/user-uuid/aaaaaaaa-1111-2222-3333-444444444444.mp4",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked, got %d", len(untracked))
	}
}

func TestFindUntracked_ProfileTrackedByUserID(t *testing.T) {
	mctx := newMatchContext()
	mctx.UserIDs["aaaaaaaa-1111-2222-3333-444444444444"] = struct{}{}

	diskFiles := []string{
		"profile/aaaaaaaa-1111-2222-3333-444444444444/profile-image.jpg",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked, got %d", len(untracked))
	}
}

func TestFindUntracked_ProfileStray(t *testing.T) {
	mctx := newMatchContext()
	mctx.UserIDs["aaaaaaaa-1111-2222-3333-444444444444"] = struct{}{}

	diskFiles := []string{
		"profile/aaaaaaaa-1111-2222-3333-444444444444/profile-image.jpg",
		"profile/bbbbbbbb-1111-2222-3333-444444444444/profile-image.jpg",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 1 {
		t.Fatalf("expected 1 untracked, got %d", len(untracked))
	}
	if untracked[0].RelPath != "profile/bbbbbbbb-1111-2222-3333-444444444444/profile-image.jpg" {
		t.Errorf("unexpected untracked: %s", untracked[0].RelPath)
	}
}

func TestFindUntracked_ImmichMarkerAlwaysKnown(t *testing.T) {
	mctx := newMatchContext()

	diskFiles := []string{
		".immich",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected .immich to be known, got %d untracked", len(untracked))
	}
}

func TestFindUntracked_UnknownTopLevelDir(t *testing.T) {
	mctx := newMatchContext()

	diskFiles := []string{
		"unknown/some/file.txt",
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())
	if len(untracked) != 1 {
		t.Fatalf("expected 1 untracked for unknown dir, got %d", len(untracked))
	}
}

func TestFindUntracked_MixedDirectories(t *testing.T) {
	mctx := newMatchContext()
	mctx.AssetPaths["library/admin/photo.jpg"] = struct{}{}
	mctx.AssetPaths["upload/admin/video.mp4"] = struct{}{}
	mctx.AssetIDs["aaaaaaaa-1111-2222-3333-444444444444"] = struct{}{}
	mctx.UserIDs["bbbbbbbb-1111-2222-3333-444444444444"] = struct{}{}

	diskFiles := []string{
		"library/admin/photo.jpg",                                                    // tracked by path
		"library/admin/stray.xmp",                                                    // untracked
		"upload/admin/video.mp4",                                                      // tracked by path
		"thumbs/user-1/aaaaaaaa-1111-2222-3333-444444444444-thumbnail.webp",          // tracked by asset ID
		"thumbs/user-1/cccccccc-1111-2222-3333-444444444444-thumbnail.webp",          // untracked (unknown asset ID)
		"encoded-video/user-1/aaaaaaaa-1111-2222-3333-444444444444.mp4",              // tracked by asset ID
		"profile/bbbbbbbb-1111-2222-3333-444444444444/profile-image.jpg",             // tracked by user ID
		"profile/dddddddd-1111-2222-3333-444444444444/profile-image.jpg",             // untracked (unknown user ID)
		".immich",                                                                     // always known
		"unknown/file.dat",                                                            // unknown dir → untracked
	}

	untracked := FindUntracked(diskFiles, mctx, testLogger())

	untrackedPaths := make(map[string]bool)
	for _, u := range untracked {
		untrackedPaths[u.RelPath] = true
	}

	expectedUntracked := []string{
		"library/admin/stray.xmp",
		"thumbs/user-1/cccccccc-1111-2222-3333-444444444444-thumbnail.webp",
		"profile/dddddddd-1111-2222-3333-444444444444/profile-image.jpg",
		"unknown/file.dat",
	}

	if len(untracked) != len(expectedUntracked) {
		t.Fatalf("expected %d untracked, got %d: %v", len(expectedUntracked), len(untracked), untracked)
	}

	for _, e := range expectedUntracked {
		if !untrackedPaths[e] {
			t.Errorf("expected %q to be untracked", e)
		}
	}
}

func TestFindUntracked_EmptyInputs(t *testing.T) {
	mctx := newMatchContext()

	// No disk files.
	untracked := FindUntracked(nil, mctx, testLogger())
	if len(untracked) != 0 {
		t.Errorf("expected 0 untracked for empty disk files, got %d", len(untracked))
	}

	// Disk files but empty match context.
	untracked = FindUntracked([]string{"library/a.jpg"}, mctx, testLogger())
	if len(untracked) != 1 {
		t.Errorf("expected 1 untracked for empty match context, got %d", len(untracked))
	}
}

func TestExtractUUID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"aaaaaaaa-1111-2222-3333-444444444444-thumbnail.webp", "aaaaaaaa-1111-2222-3333-444444444444"},
		{"aaaaaaaa-1111-2222-3333-444444444444.mp4", "aaaaaaaa-1111-2222-3333-444444444444"},
		{"short", ""},
		{"not-a-uuid-at-all-but-long-enough-to-test", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractUUID(tt.input)
		if got != tt.want {
			t.Errorf("extractUUID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"aaaaaaaa-1111-2222-3333-444444444444", true},
		{"AAAAAAAA-1111-2222-3333-444444444444", true},
		{"not-a-uuid", false},
		{"", false},
		{"aaaaaaaa11112222333344444444444", false},  // no dashes
		{"aaaaaaaa-1111-2222-3333-44444444444g", false}, // invalid hex
	}

	for _, tt := range tests {
		got := isValidUUID(tt.input)
		if got != tt.want {
			t.Errorf("isValidUUID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
