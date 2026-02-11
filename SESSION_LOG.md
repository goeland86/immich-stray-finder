# Session Log

## Session 1 - Initial Implementation

### Goal

Build a standalone Go tool (`immich-stray-finder`) that identifies and relocates orphan files in an Immich photo library where the only difference between the orphan and the tracked asset is file extension case (e.g., `photo.JPG` on disk vs `photo.jpg` tracked by Immich).

### Architectural Decision

Chose a **standalone HTTP client** using only Go standard library (`net/http`, `encoding/json`) instead of depending on `immich-go`. Rationale: `immich-go` pulls in ~35 dependencies (TUI, SSH/SFTP, image processing, Cobra/Viper) for a tool that needs exactly one API call. Zero external dependencies means fast builds and no version coupling.

### What Was Built

#### Project Structure

```
immich-stray-finder/
  go.mod                  # module github.com/goeland86/immich-stray-finder
  main.go                 # CLI flags, orchestration, signal handling, slog logging
  .gitignore
  immich/
    types.go              # SearchMetadataRequest/Response, Asset struct
    client.go             # HTTP client: POST /api/search/metadata with pagination
    client_test.go        # 4 tests (single page, multi-page, API error, context cancel)
  scanner/
    scanner.go            # filepath.WalkDir, forward-slash normalized relative paths
    scanner_test.go       # 4 tests (normal scan, empty dir, cancel, prefix)
  matcher/
    matcher.go            # Core: find orphans whose extension case-variant IS tracked
    matcher_test.go       # 6 tests (basic, no orphans, unrelated, no ext, mixed, variants)
  mover/
    mover.go              # Move files with dry-run default, cross-device fallback
    mover_test.go         # 4 tests (dry-run, actual move, dir structure, multiple files)
```

### Implementation Steps

1. **Initialized Go module** (`go mod init github.com/goeland86/immich-stray-finder`)
2. **Created `immich/types.go`** -- API request/response structs matching Immich's `/api/search/metadata` endpoint
3. **Created `immich/client.go`** -- HTTP client that paginates through all assets and collects `originalPath` values into a `map[string]struct{}` for O(1) lookup
4. **Created `scanner/scanner.go`** -- Walks the library directory with `filepath.WalkDir`, returns relative paths normalized to forward slashes to match Immich's path format
5. **Created `matcher/matcher.go`** -- For each disk file not in the Immich asset map, checks if a lower/upper extension case variant IS tracked. If so, marks it as an orphan.
6. **Created `mover/mover.go`** -- Relocates orphan files preserving directory structure. Dry-run by default. Falls back to copy+delete when `os.Rename` fails (cross-device)
7. **Created `main.go`** -- Wires everything together with CLI flags, `signal.NotifyContext` for Ctrl+C, and `log/slog` structured logging
8. **Created all test files** -- 18 unit tests total using `httptest` mock servers and temp directories
9. **Created `.gitignore`** -- Excludes binaries, IDE files, OS artifacts, and output directory

### Test Fix

The initial `scanner_test.go` had a test creating both `photo1.jpg` and `photo1.JPG` in the same directory. This failed on Windows because its filesystem is case-insensitive -- both names refer to the same file. Fixed by using distinct base names in the scanner test (the case-variant logic is properly tested in `matcher/`).

### Live Testing Against Real Immich Instance

Cross-compiled the binary for Linux (`GOOS=linux GOARCH=amd64`) and deployed via `scp` to the server running Immich.

#### API Type Fix

The real Immich API returns `nextPage` as a JSON string (e.g., `"2"`) or `null`, not an integer as originally assumed. Fixed:
- `immich/types.go` -- Changed `NextPage` from `int` to `*string`
- `immich/client.go` -- Added `strconv.Atoi` parsing for the string page number, `nil` check for last page
- `immich/client_test.go` -- Updated all test fixtures to use `*string` via `strPtr()` helper

#### Dry-Run Results

Ran against a production Immich instance:
- **63,589 assets** fetched from Immich across 64 pages (~10 seconds)
- **722,471 files** scanned on disk (~5 seconds)
- **0 orphans found** -- the library had no extension-case duplicate files

### Verification

- `go build .` -- compiles successfully (8.8 MB binary, zero dependencies)
- `go test ./...` -- all 18 tests pass
- `go vet ./...` -- no issues
- Live dry-run against production Immich -- works correctly

---

## Session 2 - Expand to General Untracked File Detection

### Goal

Expand the tool from only finding extension-case duplicates to finding **all** files on disk not tracked by Immich, regardless of reason. Also fix path mismatches and exclude Immich-internal directories.

### Changes Made

#### 1. Matcher rewrite (`matcher/matcher.go`)

Replaced extension-case-variant matching with simple set-difference logic:
- Removed `extensionVariants()` function entirely
- Renamed `Orphan` struct to `UntrackedFile` with just `RelPath` (removed `TrackedPath`)
- Renamed `FindOrphans` to `FindUntracked`: any disk file not in `assetPaths` is untracked

#### 2. Matcher tests rewrite (`matcher/matcher_test.go`)

Replaced 6 extension-case-specific tests with 4 tests for the simpler logic:
- `TestFindUntracked_AllTracked` -- 0 untracked
- `TestFindUntracked_MixedTrackedAndUntracked` -- correct untracked list
- `TestFindUntracked_NoneTracked` -- all files untracked
- `TestFindUntracked_EmptyInputs` -- edge cases

#### 3. Main wiring update (`main.go`)

- Updated call from `FindOrphans` to `FindUntracked`
- Updated output: prints each untracked file path (no more "tracked:" line)
- Updated log messages ("orphan" -> "untracked file")

#### 4. Exclude Immich-internal directories (`scanner/scanner.go`)

First live test found 722,471 files scanned with everything flagged as untracked -- the scanner was walking `thumbs/`, `encoded-video/`, `backups/`, and `profile/` directories that contain Immich-generated files not present in the asset API.

Added automatic exclusion of these top-level directories during `filepath.WalkDir` using `filepath.SkipDir`. Added `TestScanFiles_ExcludesImmichDirs` test.

#### 5. Path prefix stripping (`main.go`)

After excluding internal dirs, all 219,834 remaining files still showed as untracked. Root cause: Immich API returns Docker-internal absolute paths (e.g., `/data/library/UserName/2024/photo.jpg`) while the scanner produces paths relative to `--library-path` (e.g., `library/UserName/2024/photo.jpg`).

Added `--path-prefix` flag (default `/data/`) that strips the Docker mount prefix from API paths before comparison. This brought matches from 0 to 63,556 out of 63,589 assets.

#### 6. User-scoped library scanning (`immich/client.go`, `immich/types.go`, `main.go`)

After path prefix fix, 156,278 files were still untracked because the scanner walked the entire library root including other users' directories and `upload/` trees.

- Added `User` struct to `immich/types.go` with `StorageLabel` field
- Added `FetchCurrentUser()` method to `immich/client.go` calling `GET /api/users/me`
- Updated `main.go` to fetch the current user first, then scope the scan to `library/{storageLabel}/` only

### Live Testing Results

After all fixes, dry-run against production Immich:
- **63,589 assets** fetched from API
- **125,422 files** scanned (user's library only, internal dirs excluded)
- **63,540 matched** as tracked
- **61,882 untracked files** found (sidecar `.xmp` files, files with incorrect dates, etc.)

### Verification

- `go test ./...` -- all tests pass
- `go vet ./...` -- no issues
- Live dry-run -- correctly identifies untracked files scoped to the authenticated user
