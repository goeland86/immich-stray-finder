package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/goeland86/immich-stray-finder/immich"
	"github.com/goeland86/immich-stray-finder/matcher"
	"github.com/goeland86/immich-stray-finder/mover"
	"github.com/goeland86/immich-stray-finder/scanner"
)

func main() {
	immichURL := flag.String("immich-url", "", "Immich server URL (e.g., http://immich:2283)")
	apiKey := flag.String("api-key", "", "Immich API key")
	libraryPath := flag.String("library-path", "", "Immich storage root on disk (parent of upload/)")
	pathPrefix := flag.String("path-prefix", "/data/", "Prefix to strip from Immich originalPath values to make them relative to library-path")
	targetDir := flag.String("target-dir", "./immich-orphans", "Directory to move orphan files to")
	move := flag.Bool("move", false, "Actually move files (dry-run by default)")
	verbose := flag.Bool("verbose", false, "Enable debug logging")
	flag.Parse()

	if *immichURL == "" || *apiKey == "" || *libraryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --immich-url, --api-key, and --library-path are required")
		flag.Usage()
		os.Exit(1)
	}

	// Set up structured logging.
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Set up context with signal handling for clean shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, logger, *immichURL, *apiKey, *libraryPath, *pathPrefix, *targetDir, *move); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, immichURL, apiKey, libraryPath, pathPrefix, targetDir string, doMove bool) error {
	// Step 1: Identify the current user and fetch all asset paths from Immich.
	client := immich.NewClient(immichURL, apiKey, logger)

	user, err := client.FetchCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("fetch current user: %w", err)
	}
	if user.StorageLabel == "" {
		return fmt.Errorf("user %q has no storage label set in Immich", user.Name)
	}

	logger.Info("fetching asset paths from Immich", "url", immichURL)
	rawPaths, err := client.FetchAllAssetPaths(ctx)
	if err != nil {
		return fmt.Errorf("fetch assets: %w", err)
	}

	// Strip the Docker-internal path prefix so API paths become relative to
	// library-path, matching the scanner output.
	assetPaths := make(map[string]struct{}, len(rawPaths))
	for p := range rawPaths {
		assetPaths[strings.TrimPrefix(p, pathPrefix)] = struct{}{}
	}
	logger.Info("normalized asset paths", "prefix_stripped", pathPrefix, "count", len(assetPaths))

	// Step 2: Scan only the current user's library directory.
	userLibrary := filepath.Join(libraryPath, "library", user.StorageLabel)
	logger.Info("scanning filesystem", "path", userLibrary, "user", user.StorageLabel)
	rawFiles, err := scanner.ScanFiles(ctx, userLibrary, logger)
	if err != nil {
		return fmt.Errorf("scan filesystem: %w", err)
	}

	// Prepend "library/{storageLabel}/" so paths match the normalized API paths.
	diskPrefix := "library/" + user.StorageLabel + "/"
	diskFiles := make([]string, len(rawFiles))
	for i, f := range rawFiles {
		diskFiles[i] = diskPrefix + f
	}

	// Step 3: Find untracked files.
	logger.Info("matching files against Immich database")
	untracked := matcher.FindUntracked(diskFiles, assetPaths, logger)

	if len(untracked) == 0 {
		logger.Info("no untracked files found")
		return nil
	}

	// Step 4: Report or move.
	fmt.Fprintf(os.Stderr, "\nFound %d untracked file(s):\n", len(untracked))
	for _, u := range untracked {
		fmt.Fprintf(os.Stderr, "  %s\n", u.RelPath)
	}

	untrackedPaths := make([]string, len(untracked))
	for i, u := range untracked {
		untrackedPaths[i] = u.RelPath
	}

	if !doMove {
		fmt.Fprintln(os.Stderr, "\nDry-run mode: no files were moved. Use --move to relocate untracked files.")
	}

	return mover.MoveOrphans(untrackedPaths, libraryPath, targetDir, !doMove, logger)
}
