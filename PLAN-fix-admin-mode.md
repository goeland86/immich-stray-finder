# Fix Plan: Admin Mode Asset Fetching & Matcher Bugs

## Context

Live testing against Immich v2.5.6 revealed that the `POST /api/search/metadata` endpoint is **always scoped to the calling user** — there is no `ownerId` field in the v2 API schema. The per-user asset fetching in admin mode silently returns only the admin's own assets, causing all other users' files to be flagged as untracked. Additionally, `.immich` marker files are misidentified as untracked.

Test results: 397 false positives out of 903 files scanned (expected: 13 true strays).

## Bugs

1. **`.immich` marker files flagged as untracked** — The matcher checks if the *top-level directory* equals `.immich`, but marker files appear as `library/.immich`, `thumbs/.immich`, etc. where the top-level dir is `library`/`thumbs`. Need to check if the *filename* is `.immich`.

2. **Admin mode returns only admin's own assets** — The Immich v2 search/metadata API has no `ownerId` field and always returns the calling user's assets. The `SearchMetadataRequest.OwnerID` field is silently ignored. Result: 253 of 295 assets found, all belonging to admin.

3. **User IDs not fully populated** — `total_user_ids=1` despite 3 users, because only asset owners show up in the result set (and only admin's assets are returned).

## Approach

### Fix 1: `.immich` marker files (matcher/matcher.go)

Add a filename check at the top of `isKnown()` before the directory switch:
```go
if path.Base(relPath) == ".immich" {
    return true
}
```

### Fix 2: Add PostgreSQL direct query for admin mode

The Immich v2 API cannot fetch other users' assets. The reliable solution is to query PostgreSQL directly. Add `github.com/jackc/pgx/v5` as a dependency and a `--db-url` flag.

**New file: `immich/db.go`**
- `FetchAllAssetsFromDB(ctx, dbURL) (*AllAssetsResult, error)`
- Single query: `SELECT id, "ownerId", "originalPath" FROM asset WHERE "deletedAt" IS NULL AND status = 'active'`
- Populates all three maps: AssetPaths, AssetIDs, UserIDs

**Modified: `immich/types.go`**
- Remove `OwnerID` from `SearchMetadataRequest` (field doesn't exist in Immich v2 API)

**Modified: `immich/client.go`**
- Simplify `FetchAllAssets`: remove per-user iteration since ownerId filter doesn't work
- Single-user mode remains: paginate through search/metadata (works fine for own assets)

**Modified: `main.go`**
- Add `--db-url` flag
- Admin mode flow: require `--db-url`, query DB for all assets, merge with user list from admin API
- Single-user mode flow: unchanged (API works fine for own assets)
- If admin mode detected but no `--db-url`, warn and fall back to single-user scan

### Fix 3: Remove dead `OwnerID` code

Clean up `SearchMetadataRequest.OwnerID` and the per-user iteration logic in `fetchAssetsPage` since Immich v2 doesn't support it.

## Files to Modify

1. `go.mod` — add `pgx/v5` dependency
2. `immich/db.go` — **new** — PostgreSQL asset fetcher
3. `immich/db_test.go` — **new** — tests for DB fetcher
4. `immich/types.go` — remove `OwnerID` from `SearchMetadataRequest`
5. `immich/client.go` — simplify `FetchAllAssets` (remove per-user loop), remove `ownerID` param from `fetchAssetsPage`
6. `immich/client_test.go` — update tests for simplified API client
7. `matcher/matcher.go` — fix `.immich` detection (check filename, not directory)
8. `matcher/matcher_test.go` — add test for `.immich` in subdirectories
9. `main.go` — add `--db-url` flag, wire up DB path for admin mode

## Verification

1. `go vet ./...` — no issues
2. `go test ./...` — all tests pass
3. Cross-compile: `GOOS=linux GOARCH=arm64 go build`
4. Deploy to bpi-r4 and run:
   ```
   ./immich-stray-finder \
     --immich-url http://localhost:2283 \
     --api-key <admin-key> \
     --library-path /srv/immich-app/library \
     --path-prefix /data/ \
     --db-url postgres://postgres:GZRdZCFlw7M5L5IgYieZD04X9urQJ5Id@172.20.2.11:5432/immich \
     --verbose
   ```
5. Expected: exactly 13 untracked files (the strays we planted)
