package main

import (
	"context"
	"errors"
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
	dbURL := flag.String("db-url", "", "PostgreSQL connection URL for admin mode (e.g., postgres://user:pass@host:5432/immich)")
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

	if err := run(ctx, logger, *immichURL, *apiKey, *libraryPath, *pathPrefix, *targetDir, *dbURL, *move); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, immichURL, apiKey, libraryPath, pathPrefix, targetDir, dbURL string, doMove bool) error {
	client := immich.NewClient(immichURL, apiKey, logger)

	// Step 1: Detect admin mode by trying the admin users endpoint.
	adminMode := false
	var allUserIDs map[string]struct{}

	users, err := client.FetchAllUsers(ctx)
	if err == nil {
		// Admin mode: we have the full user list.
		adminMode = true
		allUserIDs = make(map[string]struct{}, len(users))
		for _, u := range users {
			allUserIDs[u.ID] = struct{}{}
			logger.Info("discovered user", "name", u.Name, "id", u.ID, "storage_label", u.StorageLabel)
		}
		logger.Info("admin mode activated", "user_count", len(users))
	} else if errors.Is(err, immich.ErrNotAdmin) {
		// Single-user fallback.
		logger.Info("not an admin API key, falling back to single-user mode")
	} else {
		return fmt.Errorf("check admin status: %w", err)
	}

	// Step 2: Fetch assets.
	var result *immich.AllAssetsResult

	if adminMode && dbURL != "" {
		// Admin mode with direct DB access: query PostgreSQL for all users' assets.
		logger.Info("fetching all assets from database", "db", redactDBURL(dbURL))
		result, err = immich.FetchAllAssetsFromDB(ctx, dbURL)
		if err != nil {
			return fmt.Errorf("fetch assets from database: %w", err)
		}
		// Merge user IDs from the admin user list (in case some users have no assets).
		for uid := range allUserIDs {
			result.UserIDs[uid] = struct{}{}
		}
	} else {
		if adminMode {
			// Admin key detected but no --db-url: warn and fall back to single-user scan.
			logger.Warn("admin API key detected but --db-url not provided; the Immich v2 search API " +
				"cannot fetch other users' assets. Falling back to single-user scan (admin's assets only). " +
				"Provide --db-url for full multi-user stray detection.")
		}

		// Single-user mode: identify the current user.
		user, err := client.FetchCurrentUser(ctx)
		if err != nil {
			return fmt.Errorf("fetch current user: %w", err)
		}
		if user.StorageLabel == "" {
			return fmt.Errorf("user %q has no storage label set in Immich", user.Name)
		}

		logger.Info("fetching asset paths from Immich", "url", immichURL)
		result, err = client.FetchAllAssets(ctx)
		if err != nil {
			return fmt.Errorf("fetch assets: %w", err)
		}
		// Add the current user's ID.
		result.UserIDs[user.ID] = struct{}{}

		// In single-user mode, we only scan the user's library directory.
		userLibrary := filepath.Join(libraryPath, "library", user.StorageLabel)
		logger.Info("scanning filesystem (single-user mode)", "path", userLibrary, "user", user.StorageLabel)
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

		// Strip the path prefix from asset paths.
		strippedPaths := make(map[string]struct{}, len(result.AssetPaths))
		for p := range result.AssetPaths {
			strippedPaths[strings.TrimPrefix(p, pathPrefix)] = struct{}{}
		}
		result.AssetPaths = strippedPaths
		logger.Info("normalized asset paths", "prefix_stripped", pathPrefix, "count", len(result.AssetPaths))

		// Build match context and find untracked files.
		mctx := &matcher.MatchContext{
			AssetPaths: result.AssetPaths,
			AssetIDs:   result.AssetIDs,
			UserIDs:    result.UserIDs,
		}

		logger.Info("matching files against Immich database")
		untracked := matcher.FindUntracked(diskFiles, mctx, logger)
		return reportAndMove(untracked, libraryPath, targetDir, doMove, logger)
	}

	// Admin mode with DB: scan the entire library-path root.
	// Strip the path prefix from asset paths.
	strippedPaths := make(map[string]struct{}, len(result.AssetPaths))
	for p := range result.AssetPaths {
		strippedPaths[strings.TrimPrefix(p, pathPrefix)] = struct{}{}
	}
	result.AssetPaths = strippedPaths
	logger.Info("normalized asset paths", "prefix_stripped", pathPrefix, "count", len(result.AssetPaths))

	logger.Info("scanning filesystem (admin mode)", "path", libraryPath)
	diskFiles, err := scanner.ScanFiles(ctx, libraryPath, logger)
	if err != nil {
		return fmt.Errorf("scan filesystem: %w", err)
	}

	// Build match context.
	mctx := &matcher.MatchContext{
		AssetPaths: result.AssetPaths,
		AssetIDs:   result.AssetIDs,
		UserIDs:    result.UserIDs,
	}

	logger.Info("matching files against Immich database")
	untracked := matcher.FindUntracked(diskFiles, mctx, logger)
	return reportAndMove(untracked, libraryPath, targetDir, doMove, logger)
}

// redactDBURL masks the password in a PostgreSQL connection URL for logging.
func redactDBURL(dbURL string) string {
	// postgres://user:password@host:port/db → postgres://user:***@host:port/db
	atIdx := strings.Index(dbURL, "@")
	if atIdx == -1 {
		return dbURL
	}
	prefix := dbURL[:atIdx]
	colonIdx := strings.LastIndex(prefix, ":")
	if colonIdx == -1 {
		return dbURL
	}
	return prefix[:colonIdx+1] + "***" + dbURL[atIdx:]
}

func reportAndMove(untracked []matcher.UntrackedFile, libraryPath, targetDir string, doMove bool, logger *slog.Logger) error {
	if len(untracked) == 0 {
		logger.Info("no untracked files found")
		return nil
	}

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
