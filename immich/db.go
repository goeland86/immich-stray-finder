package immich

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FetchAllAssetsFromDB queries PostgreSQL directly for all active assets.
// This bypasses the Immich API limitation where search/metadata is scoped to
// the calling user only, allowing true multi-user stray detection in admin mode.
func FetchAllAssetsFromDB(ctx context.Context, dbURL string) (*AllAssetsResult, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT id, "ownerId", "originalPath" FROM asset WHERE "deletedAt" IS NULL AND status = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("query assets: %w", err)
	}
	defer rows.Close()

	result := &AllAssetsResult{
		AssetPaths: make(map[string]struct{}),
		AssetIDs:   make(map[string]struct{}),
		UserIDs:    make(map[string]struct{}),
	}

	for rows.Next() {
		var id, ownerID, originalPath string
		if err := rows.Scan(&id, &ownerID, &originalPath); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		if originalPath != "" {
			result.AssetPaths[originalPath] = struct{}{}
		}
		if id != "" {
			result.AssetIDs[id] = struct{}{}
		}
		if ownerID != "" {
			result.UserIDs[ownerID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return result, nil
}
