package immich

// SearchMetadataRequest is the body for POST /api/search/metadata.
// Note: Immich v2 API has no ownerId field — search is always scoped to the
// calling user's assets.
type SearchMetadataRequest struct {
	Page     int  `json:"page"`
	Size     int  `json:"size"`
	WithExif bool `json:"withExif,omitempty"`
}

// SearchMetadataResponse wraps the paginated response from the search endpoint.
type SearchMetadataResponse struct {
	Assets SearchAssets `json:"assets"`
}

// SearchAssets contains the items and pagination info.
type SearchAssets struct {
	Total    int     `json:"total"`
	Count    int     `json:"count"`
	Items    []Asset `json:"items"`
	NextPage *string `json:"nextPage"`
}

// Asset represents a single asset returned by the Immich API.
type Asset struct {
	ID               string `json:"id"`
	OwnerID          string `json:"ownerId"`
	OriginalPath     string `json:"originalPath"`
	OriginalFileName string `json:"originalFileName"`
	Type             string `json:"type"`
}

// User represents a user returned by the Immich API.
type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	StorageLabel string `json:"storageLabel"`
}

// AllAssetsResult bundles the three sets needed for directory-aware matching.
type AllAssetsResult struct {
	// AssetPaths contains all originalPath values from Immich assets.
	AssetPaths map[string]struct{}
	// AssetIDs contains all asset UUIDs.
	AssetIDs map[string]struct{}
	// UserIDs contains all known user UUIDs.
	UserIDs map[string]struct{}
}
