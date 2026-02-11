package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

const defaultPageSize = 1000

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

// FetchAllAssetPaths returns a set of all originalPath values known to Immich.
func (c *Client) FetchAllAssetPaths(ctx context.Context) (map[string]struct{}, error) {
	paths := make(map[string]struct{})
	page := 1

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		reqBody := SearchMetadataRequest{
			Page: page,
			Size: defaultPageSize,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/api/search/metadata", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request page %d: %w", page, err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status %d on page %d: %s",
				resp.StatusCode, page, string(respBody))
		}

		var searchResp SearchMetadataResponse
		if err := json.Unmarshal(respBody, &searchResp); err != nil {
			return nil, fmt.Errorf("unmarshal response page %d: %w", page, err)
		}

		for _, asset := range searchResp.Assets.Items {
			if asset.OriginalPath != "" {
				paths[asset.OriginalPath] = struct{}{}
			}
		}

		c.logger.Debug("fetched asset page",
			"page", page,
			"count", searchResp.Assets.Count,
			"total_so_far", len(paths),
		)

		if searchResp.Assets.NextPage == nil || searchResp.Assets.Count == 0 {
			break
		}
		nextPage, err := strconv.Atoi(*searchResp.Assets.NextPage)
		if err != nil {
			return nil, fmt.Errorf("parse nextPage %q: %w", *searchResp.Assets.NextPage, err)
		}
		page = nextPage
	}

	c.logger.Info("finished fetching assets from Immich", "total_assets", len(paths))
	return paths, nil
}
