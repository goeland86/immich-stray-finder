package immich

import (
	"context"
	"testing"
)

func TestFetchAllAssetsFromDB_BadURL(t *testing.T) {
	// Verify that an invalid connection URL produces a clear error rather
	// than a panic. We don't need a real Postgres instance for this.
	_, err := FetchAllAssetsFromDB(context.Background(), "postgres://invalid:5432/nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestFetchAllAssetsFromDB_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchAllAssetsFromDB(ctx, "postgres://localhost:5432/immich")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
