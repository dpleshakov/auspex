package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Blueprint represents a single BPO from the ESI blueprints endpoints.
// BPCs (quantity != -1) are filtered out before being returned to the caller.
type Blueprint struct {
	ItemID       int64
	TypeID       int64
	LocationID   int64
	LocationFlag string
	MELevel      int64
	TELevel      int64
}

// esiBlueprintItem is the raw JSON shape returned by ESI.
type esiBlueprintItem struct {
	ItemID             int64  `json:"item_id"`
	TypeID             int64  `json:"type_id"`
	LocationID         int64  `json:"location_id"`
	LocationFlag       string `json:"location_flag"`
	MaterialEfficiency int64  `json:"material_efficiency"`
	TimeEfficiency     int64  `json:"time_efficiency"`
	Quantity           int64  `json:"quantity"` // -1 = BPO, positive = BPC
}

// GetCharacterBlueprints fetches all BPOs owned by characterID.
// BPCs (quantity != -1) are filtered out.
// When ESI returns X-Pages > 1, all pages are fetched sequentially before filtering.
func (c *httpClient) GetCharacterBlueprints(ctx context.Context, characterID int64, token string) ([]Blueprint, time.Time, error) {
	url := fmt.Sprintf("%s/characters/%d/blueprints", c.baseURL, characterID)
	return c.fetchAllBlueprints(ctx, url, token)
}

// GetCorporationBlueprints fetches all BPOs owned by corporationID.
// BPCs (quantity != -1) are filtered out.
// token must belong to a character with director roles in the corporation.
// When ESI returns X-Pages > 1, all pages are fetched sequentially before filtering.
func (c *httpClient) GetCorporationBlueprints(ctx context.Context, corporationID int64, token string) ([]Blueprint, time.Time, error) {
	url := fmt.Sprintf("%s/corporations/%d/blueprints", c.baseURL, corporationID)
	return c.fetchAllBlueprints(ctx, url, token)
}

// fetchAllBlueprints fetches page 1 from url, reads X-Pages, then fetches pages 2..N
// sequentially. The cacheUntil from the first response is returned unchanged.
// All raw items are collected before the BPC filter is applied.
func (c *httpClient) fetchAllBlueprints(ctx context.Context, url, token string) ([]Blueprint, time.Time, error) {
	body, headers, cacheUntil, err := c.doWithHeader(ctx, url, token)
	if err != nil {
		return nil, cacheUntil, err
	}

	var allRaw []esiBlueprintItem
	if err := json.Unmarshal(body, &allRaw); err != nil {
		return nil, cacheUntil, fmt.Errorf("parsing blueprints response: %w", err)
	}

	totalPages := parseXPages(headers.Get("X-Pages"))
	for page := 2; page <= totalPages; page++ {
		pageURL := fmt.Sprintf("%s?page=%d", url, page)
		pageBody, _, _, pageErr := c.doWithHeader(ctx, pageURL, token)
		if pageErr != nil {
			return nil, cacheUntil, pageErr
		}
		var pageItems []esiBlueprintItem
		if err := json.Unmarshal(pageBody, &pageItems); err != nil {
			return nil, cacheUntil, fmt.Errorf("parsing blueprints page %d: %w", page, err)
		}
		allRaw = append(allRaw, pageItems...)
	}

	return filterBlueprints(allRaw), cacheUntil, nil
}

// filterBlueprints removes BPCs (quantity != -1) and converts the raw ESI items
// to the Blueprint type returned to callers.
func filterBlueprints(raw []esiBlueprintItem) []Blueprint {
	bps := make([]Blueprint, 0, len(raw))
	for _, item := range raw {
		if item.Quantity != -1 {
			continue // BPC — skip
		}
		bps = append(bps, Blueprint{
			ItemID:       item.ItemID,
			TypeID:       item.TypeID,
			LocationID:   item.LocationID,
			LocationFlag: item.LocationFlag,
			MELevel:      item.MaterialEfficiency,
			TELevel:      item.TimeEfficiency,
		})
	}
	return bps
}
