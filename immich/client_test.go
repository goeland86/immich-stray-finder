package immich

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func strPtr(s string) *string { return &s }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestFetchAllAssets_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/search/metadata" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("unexpected api key: %s", r.Header.Get("x-api-key"))
		}

		resp := SearchMetadataResponse{
			Assets: SearchAssets{
				Total: 2,
				Count: 2,
				Items: []Asset{
					{ID: "aaaaaaaa-1111-2222-3333-444444444444", OwnerID: "user-1", OriginalPath: "upload/library/admin/2024/photo1.jpg"},
					{ID: "bbbbbbbb-1111-2222-3333-444444444444", OwnerID: "user-1", OriginalPath: "upload/library/admin/2024/photo2.JPG"},
				},
				NextPage: nil,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", testLogger())
	result, err := client.FetchAllAssets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AssetPaths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(result.AssetPaths))
	}
	if _, ok := result.AssetPaths["upload/library/admin/2024/photo1.jpg"]; !ok {
		t.Error("missing photo1.jpg path")
	}
	if _, ok := result.AssetPaths["upload/library/admin/2024/photo2.JPG"]; !ok {
		t.Error("missing photo2.JPG path")
	}
	if len(result.AssetIDs) != 2 {
		t.Errorf("expected 2 asset IDs, got %d", len(result.AssetIDs))
	}
	if _, ok := result.AssetIDs["aaaaaaaa-1111-2222-3333-444444444444"]; !ok {
		t.Error("missing asset ID aaaaaaaa-...")
	}
	if len(result.UserIDs) != 1 {
		t.Errorf("expected 1 user ID, got %d", len(result.UserIDs))
	}
	if _, ok := result.UserIDs["user-1"]; !ok {
		t.Error("missing user ID user-1")
	}
}

func TestFetchAllAssets_MultiPage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req SearchMetadataRequest
		json.NewDecoder(r.Body).Decode(&req)

		var resp SearchMetadataResponse
		if req.Page <= 1 {
			resp = SearchMetadataResponse{
				Assets: SearchAssets{
					Total: 3,
					Count: 2,
					Items: []Asset{
						{ID: "id-1", OwnerID: "user-1", OriginalPath: "upload/photo1.jpg"},
						{ID: "id-2", OwnerID: "user-1", OriginalPath: "upload/photo2.jpg"},
					},
					NextPage: strPtr("2"),
				},
			}
		} else {
			resp = SearchMetadataResponse{
				Assets: SearchAssets{
					Total: 3,
					Count: 1,
					Items: []Asset{
						{ID: "id-3", OwnerID: "user-1", OriginalPath: "upload/photo3.jpg"},
					},
					NextPage: nil,
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", testLogger())
	result, err := client.FetchAllAssets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AssetPaths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(result.AssetPaths))
	}
	if len(result.AssetIDs) != 3 {
		t.Errorf("expected 3 asset IDs, got %d", len(result.AssetIDs))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestFetchAllAssets_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key", testLogger())
	_, err := client.FetchAllAssets(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestFetchAllAssets_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SearchMetadataResponse{
			Assets: SearchAssets{Count: 1, Items: []Asset{{ID: "1", OwnerID: "u", OriginalPath: "p.jpg"}}, NextPage: strPtr("2")},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := NewClient(server.URL, "key", testLogger())
	_, err := client.FetchAllAssets(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchAllUsers_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/admin/users" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "admin-key" {
			t.Errorf("unexpected api key: %s", r.Header.Get("x-api-key"))
		}

		users := []User{
			{ID: "user-1", Name: "Alice", StorageLabel: "alice"},
			{ID: "user-2", Name: "Bob", StorageLabel: "bob"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin-key", testLogger())
	users, err := client.FetchAllUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", users[0].Name)
	}
	if users[1].Name != "Bob" {
		t.Errorf("expected Bob, got %s", users[1].Name)
	}
}

func TestFetchAllUsers_NotAdmin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Forbidden"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "non-admin-key", testLogger())
	_, err := client.FetchAllUsers(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !errors.Is(err, ErrNotAdmin) {
		t.Errorf("expected ErrNotAdmin, got: %v", err)
	}
}

func TestFetchAllAssets_CollectsMultipleOwners(t *testing.T) {
	// The API returns assets from the calling user only, but the response
	// may contain different ownerIDs. Verify they are all collected.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SearchMetadataResponse{
			Assets: SearchAssets{
				Total: 3,
				Count: 3,
				Items: []Asset{
					{ID: "asset-1a", OwnerID: "user-1", OriginalPath: "/data/library/alice/photo1.jpg"},
					{ID: "asset-1b", OwnerID: "user-1", OriginalPath: "/data/library/alice/photo2.jpg"},
					{ID: "asset-2a", OwnerID: "user-2", OriginalPath: "/data/library/bob/photo1.jpg"},
				},
				NextPage: nil,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin-key", testLogger())
	result, err := client.FetchAllAssets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.AssetPaths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(result.AssetPaths))
	}
	if len(result.AssetIDs) != 3 {
		t.Errorf("expected 3 asset IDs, got %d", len(result.AssetIDs))
	}
	if len(result.UserIDs) != 2 {
		t.Errorf("expected 2 user IDs, got %d", len(result.UserIDs))
	}
	if _, ok := result.AssetPaths["/data/library/alice/photo1.jpg"]; !ok {
		t.Error("missing alice/photo1.jpg")
	}
	if _, ok := result.AssetPaths["/data/library/bob/photo1.jpg"]; !ok {
		t.Error("missing bob/photo1.jpg")
	}
}
