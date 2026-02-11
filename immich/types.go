package immich

// SearchMetadataRequest is the body for POST /api/search/metadata.
type SearchMetadataRequest struct {
	Page    int `json:"page"`
	Size    int `json:"size"`
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
	ID           string `json:"id"`
	OriginalPath string `json:"originalPath"`
	OriginalFileName string `json:"originalFileName"`
	Type         string `json:"type"`
}

// User represents the current user returned by GET /api/users/me.
type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	StorageLabel string `json:"storageLabel"`
}
