package esi

import (
	"context"
	"encoding/json"
	"fmt"
)

// Compile-time assertion: *httpClient implements Client.
var _ Client = (*httpClient)(nil)

// UniverseType holds fully resolved EVE type data including group and category.
// It is returned by GetUniverseType, which internally chains three ESI calls:
//
//	GET /universe/types/{type_id}       → TypeName, GroupID
//	GET /universe/groups/{group_id}     → GroupName, CategoryID
//	GET /universe/categories/{cat_id}  → CategoryName
type UniverseType struct {
	TypeID       int64
	TypeName     string
	GroupID      int64
	GroupName    string
	CategoryID   int64
	CategoryName string
}

// esiTypeResponse is the raw JSON from GET /universe/types/{type_id}.
type esiTypeResponse struct {
	TypeID  int64  `json:"type_id"`
	Name    string `json:"name"`
	GroupID int64  `json:"group_id"`
}

// esiGroupResponse is the raw JSON from GET /universe/groups/{group_id}.
type esiGroupResponse struct {
	GroupID    int64  `json:"group_id"`
	Name       string `json:"name"`
	CategoryID int64  `json:"category_id"`
}

// esiCategoryResponse is the raw JSON from GET /universe/categories/{category_id}.
type esiCategoryResponse struct {
	CategoryID int64  `json:"category_id"`
	Name       string `json:"name"`
}

// GetUniverseType resolves a type_id into a fully populated UniverseType by
// chaining three ESI calls. Universe endpoints are public — no token is sent.
func (c *httpClient) GetUniverseType(ctx context.Context, typeID int64) (UniverseType, error) {
	// Step 1: fetch type.
	typeURL := fmt.Sprintf("%s/universe/types/%d", c.baseURL, typeID)
	body, _, err := c.do(ctx, typeURL, "")
	if err != nil {
		return UniverseType{}, fmt.Errorf("fetching type %d: %w", typeID, err)
	}
	var typeResp esiTypeResponse
	if err := json.Unmarshal(body, &typeResp); err != nil {
		return UniverseType{}, fmt.Errorf("parsing type %d response: %w", typeID, err)
	}

	// Step 2: fetch group.
	groupURL := fmt.Sprintf("%s/universe/groups/%d", c.baseURL, typeResp.GroupID)
	body, _, err = c.do(ctx, groupURL, "")
	if err != nil {
		return UniverseType{}, fmt.Errorf("fetching group %d: %w", typeResp.GroupID, err)
	}
	var groupResp esiGroupResponse
	if err := json.Unmarshal(body, &groupResp); err != nil {
		return UniverseType{}, fmt.Errorf("parsing group %d response: %w", typeResp.GroupID, err)
	}

	// Step 3: fetch category.
	catURL := fmt.Sprintf("%s/universe/categories/%d", c.baseURL, groupResp.CategoryID)
	body, _, err = c.do(ctx, catURL, "")
	if err != nil {
		return UniverseType{}, fmt.Errorf("fetching category %d: %w", groupResp.CategoryID, err)
	}
	var catResp esiCategoryResponse
	if err := json.Unmarshal(body, &catResp); err != nil {
		return UniverseType{}, fmt.Errorf("parsing category %d response: %w", groupResp.CategoryID, err)
	}

	return UniverseType{
		TypeID:       typeID,
		TypeName:     typeResp.Name,
		GroupID:      typeResp.GroupID,
		GroupName:    groupResp.Name,
		CategoryID:   groupResp.CategoryID,
		CategoryName: catResp.Name,
	}, nil
}
