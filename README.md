# immich-stray-finder

A standalone tool that finds and relocates **untracked files** in an [Immich](https://immich.app/) photo library -- files that exist on disk but are not linked in the Immich database.

This helps reclaim disk space by identifying files that Immich doesn't know about, such as leftover imports, sidecar files, or extension-case duplicates.

## Features

- Zero external dependencies -- uses only Go standard library
- Automatic user detection via API key -- scans only your library
- Automatically excludes Immich-internal directories (`thumbs/`, `encoded-video/`, `backups/`, `profile/`)
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

1. **Identifies the current user** by calling `GET /api/users/me` with the provided API key, extracting the `storageLabel` to determine the user's library directory.
2. **Fetches all asset paths from Immich** by paginating through `POST /api/search/metadata`, collecting every `originalPath` into a set for O(1) lookup. The Docker-internal path prefix (default `/data/`) is stripped so paths are relative to `--library-path`.
3. **Scans the user's library directory** (`library/{storageLabel}/`) under `--library-path` using `filepath.WalkDir`, producing relative paths normalized to forward slashes. Immich-internal directories (`thumbs/`, `encoded-video/`, `backups/`, `profile/`) are automatically skipped.
4. **Finds untracked files** -- any file on disk whose path is not in the Immich asset set is untracked.
5. **Reports or moves** -- in dry-run mode (default), prints the list of untracked files. With `--move`, relocates them to `--target-dir` preserving the directory structure.

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

There are unit tests across all packages covering the HTTP client (with mock servers), filesystem scanner (including directory exclusion), matching algorithm, and file mover (both dry-run and actual moves).

## AI-Generated Code

This project was generated with the assistance of [Claude Code](https://claude.ai/claude-code) (Anthropic's Claude Opus 4.6). The code, tests, documentation, and session logs were produced through an interactive conversation with the AI, guided and reviewed by a human developer.

## License

AGPL-3.0 -- see [LICENSE](LICENSE) for details.
