# immich-stray-finder

A standalone tool that finds and relocates **untracked files** in an [Immich](https://immich.app/) photo library -- files that exist on disk but are not linked in the Immich database.

This helps reclaim disk space by identifying files that Immich doesn't know about, such as leftover imports, sidecar files, orphaned thumbnails, or stale encoded videos.

## Features

- Zero external dependencies -- uses only Go standard library
- **Admin mode auto-detection** -- with an admin API key, scans all users and all directories; with a regular key, falls back to single-user mode
- **Full-scope scanning** -- checks `library/`, `upload/`, `thumbs/`, `encoded-video/`, and `profile/` directories
- **Directory-aware matching** -- uses the right strategy per directory type (path match, UUID match, or user ID match)
- Strips Docker-internal path prefixes (`/data/` by default) so disk paths match API paths
- Dry-run by default -- shows what would be moved without touching anything
- Preserves directory structure when relocating files
- Cross-device move support (falls back to copy+delete)
- Clean shutdown on Ctrl+C via signal handling
- Structured logging with `log/slog`

## Building

Requires Go 1.22 or later.

```bash
git clone https://github.com/goeland86/immich-stray-finder.git
cd immich-stray-finder
go build .
```

This produces an `immich-stray-finder` binary (or `immich-stray-finder.exe` on Windows).

### Cross-compiling for Linux

```bash
GOOS=linux GOARCH=amd64 go build -o immich-stray-finder_linux .
```

## Usage

```
immich-stray-finder [flags]
```

### Required Flags

| Flag | Description |
|------|-------------|
| `--immich-url` | Immich server URL (e.g., `http://immich:2283`) |
| `--api-key` | Immich API key (generate in Immich under User Settings > API Keys) |
| `--library-path` | Path to the Immich storage root on disk (the directory containing `library/`, `upload/`, `thumbs/`, etc.) |

### Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--path-prefix` | `/data/` | Prefix to strip from Immich `originalPath` values to make them relative to `--library-path`. Change this if your Immich Docker volume mount differs. |
| `--target-dir` | `./immich-orphans` | Directory where untracked files will be moved |
| `--move` | `false` | Actually move files (dry-run by default) |
| `--verbose` | `false` | Enable debug logging |

### Examples

**Dry-run** (see what would be moved, without moving anything):

```bash
./immich-stray-finder \
  --immich-url http://192.168.1.100:2283 \
  --api-key your-api-key-here \
  --library-path /mnt/photos/immich
```

**Actually move untracked files:**

```bash
./immich-stray-finder \
  --immich-url http://192.168.1.100:2283 \
  --api-key your-api-key-here \
  --library-path /mnt/photos/immich \
  --target-dir /mnt/photos/untracked \
  --move
```

**With debug logging:**

```bash
./immich-stray-finder \
  --immich-url http://192.168.1.100:2283 \
  --api-key your-api-key-here \
  --library-path /mnt/photos/immich \
  --verbose
```

**Custom Docker path prefix** (if your Immich volume is not mounted at `/data`):

```bash
./immich-stray-finder \
  --immich-url http://192.168.1.100:2283 \
  --api-key your-api-key-here \
  --library-path /mnt/photos/immich \
  --path-prefix /custom/mount/
```

## How It Works

### Admin Mode Auto-Detection

On startup, the tool calls `GET /api/admin/users`. If the API key has admin privileges, it activates **admin mode** which scans all users and all directory types. If the call returns 403, it falls back to **single-user mode** (original behavior).

| Mode | Scope | Directories scanned |
|------|-------|-------------------|
| Admin | All users | `library/`, `upload/`, `thumbs/`, `encoded-video/`, `profile/` |
| Single-user | Current user only | `library/{storageLabel}/` only |

### Matching Strategies

Different directories use different strategies to determine whether a file is tracked:

| Directory | Strategy | How it works |
|-----------|----------|-------------|
| `library/`, `upload/` | Exact path match | File's relative path must exist in the set of `originalPath` values from the API |
| `thumbs/`, `encoded-video/` | Asset UUID match | The filename starts with an asset UUID (e.g., `{uuid}-thumbnail.webp`); that UUID is checked against all known asset IDs |
| `profile/` | User UUID match | The 2nd path segment is a user UUID (e.g., `profile/{userId}/...`); that UUID is checked against all known user IDs |
| `backups/` | Skipped | Contains system-managed database dumps, always excluded from scanning |
| `.immich` | Always known | Immich marker files are never flagged |

### Pipeline

1. **Auto-detect mode** by calling the admin users endpoint.
2. **Fetch assets** -- in admin mode, iterates per user with the `ownerId` filter; in single-user mode, searches without a filter.
3. **Scan the filesystem** -- admin mode scans the entire `--library-path`; single-user mode scans only `library/{storageLabel}/`.
4. **Match files** using directory-aware strategies.
5. **Report or move** -- in dry-run mode (default), prints untracked files. With `--move`, relocates them preserving directory structure.

### Path Matching

The tool automatically handles the path translation between Immich's Docker-internal paths and the host filesystem:

- Immich API returns paths like `/data/library/username/2024/01/photo.jpg`
- The `--path-prefix` (default `/data/`) is stripped, giving `library/username/2024/01/photo.jpg`
- The scanner produces relative paths like `library/username/2024/01/photo.jpg` from `--library-path`
- These match, so the file is tracked

## Running Tests

```bash
go test ./...
```

There are unit tests across all packages covering the HTTP client (with mock servers), filesystem scanner (including directory exclusion), matching algorithm (all directory strategies), and file mover (both dry-run and actual moves).

## AI-Generated Code

This project was generated with the assistance of [Claude Code](https://claude.ai/claude-code) (Anthropic's Claude Opus 4.6). The code, tests, documentation, and session logs were produced through an interactive conversation with the AI, guided and reviewed by a human developer.

## License

AGPL-3.0 -- see [LICENSE](LICENSE) for details.
