# Session Log

## Session 1 - Initial Implementation

### Goal

Build a standalone Go tool that identifies and relocates orphan files in an Immich photo library where the only difference between the orphan and the tracked asset is file extension case (e.g., `photo.JPG` on disk vs `photo.jpg` tracked by Immich).

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
    types.go              # API request/response structs (Search, Asset, User)
    client.go             # HTTP client: pagination, current user lookup
    client_test.go        # 4 tests (single page, multi-page, API error, context cancel)
  scanner/
    scanner.go            # filepath.WalkDir with directory exclusions
    scanner_test.go       # 5 tests (normal scan, empty dir, cancel, prefix, exclusions)
  matcher/
    matcher.go            # Core: find files on disk not tracked by Immich
    matcher_test.go       # 4 tests (all tracked, mixed, none tracked, empty inputs)
  mover/
    mover.go              # Move files with dry-run default, cross-device fallback
    mover_test.go         # 4 tests (dry-run, actual move, dir structure, multiple files)
```

### Implementation Steps

1. **Initialized Go module**
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
- **63,589 assets** fetched across 64 pages (~10 seconds)
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

After excluding internal dirs, all 219,834 remaining files still showed as untracked. Root cause: Immich API returns Docker-internal absolute paths (e.g., `/data/library/username/2024/photo.jpg`) while the scanner produces paths relative to `--library-path` (e.g., `library/username/2024/photo.jpg`).

Added `--path-prefix` flag (default `/data/`) that strips the Docker mount prefix from API paths before comparison.

#### 6. User-scoped library scanning (`immich/client.go`, `immich/types.go`, `main.go`)

After path prefix fix, many files were still untracked because the scanner walked the entire library root including other users' directories and `upload/` trees.

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

---

## Session 3 - Rename, Cleanup, and Release

### Goal

Rename the project from `immich_dupe_detector` to `immich-stray-finder` to better reflect its expanded scope, clean up history, and publish an initial release.

### Changes Made

#### 1. Project rename

- Renamed Go module from `github.com/goeland86/immich_dupe_detector` to `github.com/goeland86/immich-stray-finder`
- Updated all import paths in `main.go`
- Updated `.gitignore` binary entries
- Updated all references in `README.md` and `SESSION_LOG.md`
- Renamed GitHub repository via `gh repo rename`
- Updated local git remote URL

#### 2. README rewrite

Rewrote `README.md` to reflect current functionality:
- Describes general untracked file detection (not just extension-case duplicates)
- Documents `--path-prefix` flag
- Documents automatic user detection and library scoping
- Documents excluded Immich-internal directories
- Added cross-compilation instructions

#### 3. Git history squash

Squashed all 4 commits into a single initial commit using an orphan branch approach (required because the root commit has no parent to soft-reset to).

#### 4. Initial release (v1.0.0)

Built binaries for three platforms and published as a GitHub release:
- `immich-stray-finder_linux_amd64` -- Linux x86-64
- `immich-stray-finder_linux_arm64` -- Linux ARM64
- `immich-stray-finder_windows_amd64.exe` -- Windows x86-64

### Verification

- `go test ./...` -- all tests pass
- `go vet ./...` -- no issues
- All three binaries verified with `file` command
- Release published at v1.0.0 with all artifacts

---

## Session 4 - Full-Scope Multi-User Stray Detection

### Goal

Expand from scanning only the authenticated user's `library/{storageLabel}/` directory to finding **any** file on disk that Immich doesn't know about, across all users and all directory types (thumbnails, encoded video, profile pictures).

### Changes Made

#### 1. Admin auto-detection (`immich/client.go`)

Added `FetchAllUsers(ctx)` method calling `GET /api/admin/users`. Returns `ErrNotAdmin` sentinel on 403, enabling graceful fallback to single-user mode.

#### 2. Multi-user asset fetching (`immich/types.go`, `immich/client.go`)

- Added `OwnerID` field to `Asset` and `SearchMetadataRequest` structs
- Added `AllAssetsResult` struct bundling three sets: `AssetPaths`, `AssetIDs`, `UserIDs`
- Replaced `FetchAllAssetPaths` with `FetchAllAssets(ctx, userIDs)`:
  - Admin mode: iterates per user with `ownerId` filter, merging results
  - Single-user mode: searches without filter
  - Collects `originalPath`, asset `ID`, and `OwnerID` into the three maps

#### 3. Expanded scanner (`scanner/scanner.go`)

Reduced `excludeDirs` from `{thumbs, encoded-video, backups, profile}` to just `{backups}`. The other directories are now scanned and matched by UUID.

#### 4. Directory-aware matcher (`matcher/matcher.go`)

Added `MatchContext` struct and rewrote `FindUntracked` to dispatch by top-level directory:
- `library/`, `upload/` → exact path match against `AssetPaths`
- `thumbs/`, `encoded-video/` → extract UUID from filename, check against `AssetIDs`
- `profile/` → extract user UUID from path, check against `UserIDs`
- `.immich` → always known
- Unknown directories → flagged as untracked

Added helpers: `matchByAssetID`, `matchByUserID`, `extractUUID`, `isValidUUID`.

#### 5. Rewired orchestration (`main.go`)

New flow:
1. Try `FetchAllUsers` → admin mode if ok, single-user if `ErrNotAdmin`
2. Admin: fetch assets per user, scan entire `--library-path`
3. Single-user: fetch current user, scan `library/{storageLabel}/` only
4. Build `MatchContext`, call `FindUntracked`, report/move

Extracted `reportAndMove` helper to avoid duplicating the output logic.

#### 6. Updated tests

- `immich/client_test.go`: Renamed `TestFetchAllAssetPaths_*` to `TestFetchAllAssets_*`, added `TestFetchAllUsers_Success`, `TestFetchAllUsers_NotAdmin`, `TestFetchAllAssets_MultiUser`
- `scanner/scanner_test.go`: Updated `TestScanFiles_ExcludesImmichDirs` → `TestScanFiles_ExcludesBackupsOnly`, verifies thumbs/encoded-video/profile are now scanned
- `matcher/matcher_test.go`: Replaced 4 tests with 14 tests covering all matching strategies (library, upload, thumbs, encoded-video, profile, .immich, unknown dirs, mixed, empty inputs, UUID extraction/validation)

#### 7. Updated documentation

- `README.md`: Documented admin mode, matching strategies table, expanded pipeline description
- `SESSION_LOG.md`: Added this session entry

### Verification

- `go vet ./...` -- no issues
- `go test ./...` -- all tests pass (immich, matcher, mover, scanner)
- `GOOS=linux GOARCH=amd64 go build` -- cross-compiles successfully
