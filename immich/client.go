package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

const defaultPageSize = 1000

// ErrNotAdmin is returned when the API key does not have admin privileges.
var ErrNotAdmin = errors.New("API key does not have admin privileges")

// Client communicates with the Immich API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new Immich API client.
func NewClient(baseURL, apiKey string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{},
		logger:     logger,
	}
}

// FetchCurrentUser returns the user associated with the configured API key.
func (c *Client) FetchCurrentUser(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/users/me", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	c.logger.Info("authenticated user", "name", user.Name, "storage_label", user.StorageLabel)
	return &user, nil
}

// FetchAllUsers returns all users from the admin API.
// Returns ErrNotAdmin if the API key lacks admin privileges (403).
func (c *Client) FetchAllUsers(ctx context.Context) ([]User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/admin/users", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrNotAdmin
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("unmarshal users: %w", err)
	}

	c.logger.Info("fetched admin user list", "user_count", len(users))
	return users, nil
}

// FetchAllAssets collects all asset data needed for directory-aware matching.
// The Immich v2 search/metadata API is always scoped to the calling user's
// assets — there is no ownerId filter. This method paginates through all
// results available to the current API key.
func (c *Client) FetchAllAssets(ctx context.Context) (*AllAssetsResult, error) {
	result := &AllAssetsResult{
		AssetPaths: make(map[string]struct{}),
		AssetIDs:   make(map[string]struct{}),
		UserIDs:    make(map[string]struct{}),
	}

	if err := c.fetchAssetsPage(ctx, result); err != nil {
		return nil, err
	}

	c.logger.Info("finished fetching assets from Immich",
		"total_paths", len(result.AssetPaths),
		"total_asset_ids", len(result.AssetIDs),
		"total_user_ids", len(result.UserIDs),
	)
	return result, nil
}

// fetchAssetsPage paginates through the search endpoint and merges results
// into the provided AllAssetsResult.
func (c *Client) fetchAssetsPage(ctx context.Context, result *AllAssetsResult) error {
	page := 1
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		reqBody := SearchMetadataRequest{
			Page: page,
			Size: defaultPageSize,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/api/search/metadata", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request page %d: %w", page, err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read response page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API returned status %d on page %d: %s",
				resp.StatusCode, page, string(respBody))
		}

		var searchResp SearchMetadataResponse
		if err := json.Unmarshal(respBody, &searchResp); err != nil {
			return fmt.Errorf("unmarshal response page %d: %w", page, err)
		}

		for _, asset := range searchResp.Assets.Items {
			if asset.OriginalPath != "" {
				result.AssetPaths[asset.OriginalPath] = struct{}{}
			}
			if asset.ID != "" {
				result.AssetIDs[asset.ID] = struct{}{}
			}
			if asset.OwnerID != "" {
				result.UserIDs[asset.OwnerID] = struct{}{}
			}
		}

		c.logger.Debug("fetched asset page",
			"page", page,
			"count", searchResp.Assets.Count,
			"total_paths_so_far", len(result.AssetPaths),
		)

		if searchResp.Assets.NextPage == nil || searchResp.Assets.Count == 0 {
			break
		}
		nextPage, err := strconv.Atoi(*searchResp.Assets.NextPage)
		if err != nil {
			return fmt.Errorf("parse nextPage %q: %w", *searchResp.Assets.NextPage, err)
		}
		page = nextPage
	}

	return nil
}
