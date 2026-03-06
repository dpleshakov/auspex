package esi

import (
	"context"
	"encoding/json"
	"fmt"
)

// CorporationOffice represents one entry from GET /corporations/{id}/offices/.
type CorporationOffice struct {
	OfficeID  int64 `json:"office_id"`
	StationID int64 `json:"station_id"`
}

// GetCorporationOffices returns all offices rented by the corporation.
// Requires esi-corporations.read_facilities.v1 scope.
func (c *httpClient) GetCorporationOffices(ctx context.Context, corporationID int64, token string) ([]CorporationOffice, error) {
	url := fmt.Sprintf("%s/corporations/%d/offices/", c.baseURL, corporationID)
	body, _, err := c.do(ctx, url, token)
	if err != nil {
		return nil, err
	}
	var offices []CorporationOffice
	if err := json.Unmarshal(body, &offices); err != nil {
		return nil, fmt.Errorf("parsing offices response: %w", err)
	}
	return offices, nil
}
