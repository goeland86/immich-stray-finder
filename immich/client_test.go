package immich

import (
	"context"
	"encoding/json"
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

func TestFetchAllAssetPaths_SinglePage(t *testing.T) {
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
					{ID: "1", OriginalPath: "upload/library/admin/2024/photo1.jpg"},
					{ID: "2", OriginalPath: "upload/library/admin/2024/photo2.JPG"},
				},
				NextPage: nil,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", testLogger())
	paths, err := client.FetchAllAssetPaths(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if _, ok := paths["upload/library/admin/2024/photo1.jpg"]; !ok {
		t.Error("missing photo1.jpg")
	}
	if _, ok := paths["upload/library/admin/2024/photo2.JPG"]; !ok {
		t.Error("missing photo2.JPG")
	}
}

func TestFetchAllAssetPaths_MultiPage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req SearchMetadataRequest
		json.NewDecoder(r.Body).Decode(&req)

		var resp SearchMetadataResponse
		if req.Page <= 1 {
			resp = SearchMetadataResponse{
				Assets: SearchAssets{
					Total:    3,
					Count:    2,
					Items: []Asset{
						{ID: "1", OriginalPath: "upload/photo1.jpg"},
						{ID: "2", OriginalPath: "upload/photo2.jpg"},
					},
					NextPage: strPtr("2"),
				},
			}
		} else {
			resp = SearchMetadataResponse{
				Assets: SearchAssets{
					Total:    3,
					Count:    1,
					Items: []Asset{
						{ID: "3", OriginalPath: "upload/photo3.jpg"},
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
	paths, err := client.FetchAllAssetPaths(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestFetchAllAssetPaths_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key", testLogger())
	_, err := client.FetchAllAssetPaths(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestFetchAllAssetPaths_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SearchMetadataResponse{
			Assets: SearchAssets{Count: 1, Items: []Asset{{ID: "1", OriginalPath: "p.jpg"}}, NextPage: strPtr("2")},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := NewClient(server.URL, "key", testLogger())
	_, err := client.FetchAllAssetPaths(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
