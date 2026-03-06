package esi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Compile-time assertion: *httpClient implements Client.
var _ Client = (*httpClient)(nil)

// ErrForbidden is returned by GetUniverseStructure when the authenticated
// character does not have access to the structure (ESI 403).
var ErrForbidden = errors.New("ESI: 403 Forbidden")

// ErrNotFound is returned by GetUniverseStructure when ESI returns 404,
// indicating the ID is not a player structure (e.g. a corp office item ID).
var ErrNotFound = errors.New("ESI: 404 Not Found")

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

// UniverseNamesEntry is one item returned by POST /universe/names/.
type UniverseNamesEntry struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// PostUniverseNames resolves a batch of EVE IDs to names via POST /universe/names/.
// This is a public endpoint (no auth required). Returns an empty slice for an empty input.
func (c *httpClient) PostUniverseNames(ctx context.Context, ids []int64) ([]UniverseNamesEntry, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	reqBody, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("marshaling ids: %w", err)
	}

	url := fmt.Sprintf("%s/universe/names/", c.baseURL)
	body, err := c.doPost(ctx, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("posting universe/names: %w", err)
	}

	var entries []UniverseNamesEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing universe/names response: %w", err)
	}
	return entries, nil
}

// UniverseStructure holds the fields we need from GET /universe/structures/{id}/.
type UniverseStructure struct {
	Name          string `json:"name"`
	SolarSystemID int64  `json:"solar_system_id"`
}

// GetUniverseStructure fetches a player-owned structure by ID using an authenticated token.
// Returns ErrForbidden if the character does not have docking access (403).
func (c *httpClient) GetUniverseStructure(ctx context.Context, structureID int64, token string) (UniverseStructure, error) {
	url := fmt.Sprintf("%s/universe/structures/%d/", c.baseURL, structureID)
	body, _, err := c.do(ctx, url, token)
	if err != nil {
		if strings.Contains(err.Error(), "ESI status 403") || strings.Contains(err.Error(), "ESI status 401") {
			return UniverseStructure{}, ErrForbidden
		}
		if strings.Contains(err.Error(), "ESI status 404") {
			return UniverseStructure{}, ErrNotFound
		}
		return UniverseStructure{}, fmt.Errorf("fetching structure %d: %w", structureID, err)
	}

	var s UniverseStructure
	if err := json.Unmarshal(body, &s); err != nil {
		return UniverseStructure{}, fmt.Errorf("parsing structure %d response: %w", structureID, err)
	}
	return s, nil
}

// esiSystemResponse is the minimal subset of GET /universe/systems/{id}/ we need.
type esiSystemResponse struct {
	Name string `json:"name"`
}

// GetUniverseSystem fetches the name of a solar system. Public endpoint, no token required.
func (c *httpClient) GetUniverseSystem(ctx context.Context, systemID int64) (string, error) {
	url := fmt.Sprintf("%s/universe/systems/%d/", c.baseURL, systemID)
	body, _, err := c.do(ctx, url, "")
	if err != nil {
		return "", fmt.Errorf("fetching system %d: %w", systemID, err)
	}

	var s esiSystemResponse
	if err := json.Unmarshal(body, &s); err != nil {
		return "", fmt.Errorf("parsing system %d response: %w", systemID, err)
	}
	return s.Name, nil
}
