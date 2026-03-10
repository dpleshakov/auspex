package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CorpAsset represents one entry from GET /corporations/{id}/assets/.
type CorpAsset struct {
	ItemID       int64  `json:"item_id"`
	LocationID   int64  `json:"location_id"`
	LocationFlag string `json:"location_flag"`
	LocationType string `json:"location_type"`
}

// GetCorporationAssets fetches one page of corporation assets.
// Returns the raw asset records, the total page count (from X-Pages header),
// the ESI cache expiry, and any error.
// Caller is responsible for iterating all pages and filtering by LocationFlag.
// Requires esi-assets.read_corporation_assets.v1 scope.
func (c *httpClient) GetCorporationAssets(ctx context.Context, corpID int64, token string, page int) ([]CorpAsset, int, time.Time, error) {
	url := fmt.Sprintf("%s/corporations/%d/assets/?page=%d", c.baseURL, corpID, page)
	body, headers, cacheUntil, err := c.doWithHeader(ctx, url, token)
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	totalPages := parseXPages(headers.Get("X-Pages"))
	var assets []CorpAsset
	if err := json.Unmarshal(body, &assets); err != nil {
		return nil, 0, time.Time{}, fmt.Errorf("parsing assets response: %w", err)
	}
	return assets, totalPages, cacheUntil, nil
}
