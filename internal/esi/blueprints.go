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
	ItemID     int64
	TypeID     int64
	LocationID int64
	MELevel    int64
	TELevel    int64
}

// esiBlueprintItem is the raw JSON shape returned by ESI.
type esiBlueprintItem struct {
	ItemID             int64 `json:"item_id"`
	TypeID             int64 `json:"type_id"`
	LocationID         int64 `json:"location_id"`
	MaterialEfficiency int64 `json:"material_efficiency"`
	TimeEfficiency     int64 `json:"time_efficiency"`
	Quantity           int64 `json:"quantity"` // -1 = BPO, positive = BPC
}

// GetCharacterBlueprints fetches all BPOs owned by characterID.
// BPCs (quantity != -1) are filtered out.
func (c *httpClient) GetCharacterBlueprints(ctx context.Context, characterID int64, token string) ([]Blueprint, time.Time, error) {
	url := fmt.Sprintf("%s/characters/%d/blueprints", c.baseURL, characterID)
	body, cacheUntil, err := c.do(ctx, url, token)
	if err != nil {
		return nil, cacheUntil, err
	}
	bps, err := parseBlueprints(body)
	return bps, cacheUntil, err
}

// GetCorporationBlueprints fetches all BPOs owned by corporationID.
// BPCs (quantity != -1) are filtered out.
// token must belong to a character with director roles in the corporation.
func (c *httpClient) GetCorporationBlueprints(ctx context.Context, corporationID int64, token string) ([]Blueprint, time.Time, error) {
	url := fmt.Sprintf("%s/corporations/%d/blueprints", c.baseURL, corporationID)
	body, cacheUntil, err := c.do(ctx, url, token)
	if err != nil {
		return nil, cacheUntil, err
	}
	bps, err := parseBlueprints(body)
	return bps, cacheUntil, err
}

func parseBlueprints(data []byte) ([]Blueprint, error) {
	var raw []esiBlueprintItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing blueprints response: %w", err)
	}
	bps := make([]Blueprint, 0, len(raw))
	for _, item := range raw {
		if item.Quantity != -1 {
			continue // BPC — skip
		}
		bps = append(bps, Blueprint{
			ItemID:     item.ItemID,
			TypeID:     item.TypeID,
			LocationID: item.LocationID,
			MELevel:    item.MaterialEfficiency,
			TELevel:    item.TimeEfficiency,
		})
	}
	return bps, nil
}
