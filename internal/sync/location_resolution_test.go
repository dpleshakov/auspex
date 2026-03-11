package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// --- TestResolveLocationIDs_NPCStation_Hangar_ResolvesViaGetStation ---
// Verifies that NPC station IDs with "Hangar" flag are resolved via GetStation.
func TestResolveLocationIDs_NPCStation_Hangar_ResolvesViaGetStation(t *testing.T) {
	const stationID int64 = 60003760

	var insertedLocations []store.InsertLocationParams
	stationCalled := false

	q := &mockQuerier{
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: stationID, LocationFlag: "Hangar"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getStationFunc: func(_ context.Context, id int64) (string, error) {
			stationCalled = true
			if id != stationID {
				t.Errorf("GetStation: unexpected id %d", id)
			}
			return "Jita IV - Moon 4 - Caldari Navy Assembly Plant", nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if !stationCalled {
		t.Error("GetStation must be called for NPC station with Hangar flag")
	}
	if len(insertedLocations) != 1 {
		t.Fatalf("expected 1 InsertLocation call, got %d", len(insertedLocations))
	}
	if insertedLocations[0].ID != stationID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, stationID)
	}
	if insertedLocations[0].Name != "Jita IV - Moon 4 - Caldari Navy Assembly Plant" {
		t.Errorf("InsertLocation Name: got %q", insertedLocations[0].Name)
	}
}

// --- TestResolveLocationIDs_SkipsAlreadyCached ---
// Verifies that location IDs already in eve_locations are not re-fetched.
func TestResolveLocationIDs_SkipsAlreadyCached(t *testing.T) {
	stationCalled := false

	q := &mockQuerier{
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: 60003760, LocationFlag: "Hangar"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			// Already cached.
			return store.EveLocation{ID: id, Name: "Jita IV", ResolvedAt: time.Now()}, nil
		},
	}

	esiMock := &mockESIClient{
		getStationFunc: func(_ context.Context, _ int64) (string, error) {
			stationCalled = true
			return "", nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if stationCalled {
		t.Error("GetStation must not be called when location is already cached")
	}
}

// --- TestResolveLocationIDs_NoIDs_NoESICalls ---
// Verifies that no ESI calls are made when there are no location IDs.
func TestResolveLocationIDs_NoIDs_NoESICalls(t *testing.T) {
	q := &mockQuerier{
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return nil, nil
		},
	}

	// ESI mock with panicking methods — any call would fail the test.
	esiMock := &mockESIClient{}

	w := New(q, esiMock, time.Minute)
	// Should complete without panicking.
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)
}

// --- TestResolveLocationIDs_Structure_ResolvedWithSystemName ---
// Verifies that structure IDs (>= 1T) with "Hangar" flag are resolved via
// GetUniverseStructure + GetUniverseSystem, stored as "SystemName — StructureName".
func TestResolveLocationIDs_Structure_ResolvedWithSystemName(t *testing.T) {
	const structureID int64 = 1_000_000_000_001
	const systemID int64 = 30000142

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok123"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: structureID, LocationFlag: "Hangar"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, id int64, token string) (esi.UniverseStructure, error) {
			if id != structureID {
				t.Errorf("GetUniverseStructure: unexpected id %d", id)
			}
			if token != "tok123" {
				t.Errorf("GetUniverseStructure: unexpected token %q", token)
			}
			return esi.UniverseStructure{Name: "Tranquility Trading Tower", SolarSystemID: systemID}, nil
		},
		getUniverseSystemFunc: func(_ context.Context, id int64) (string, error) {
			if id != systemID {
				t.Errorf("GetUniverseSystem: unexpected id %d", id)
			}
			return "Perimeter", nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	// Expect two inserts: one for the system name, one for the structure.
	if len(insertedLocations) != 2 {
		t.Fatalf("expected 2 InsertLocation calls (system + structure), got %d", len(insertedLocations))
	}

	// System name cached first.
	if insertedLocations[0].ID != systemID {
		t.Errorf("first insert: expected system ID %d, got %d", systemID, insertedLocations[0].ID)
	}
	if insertedLocations[0].Name != "Perimeter" {
		t.Errorf("system name: got %q, want Perimeter", insertedLocations[0].Name)
	}

	// Structure stored as "System — Structure".
	if insertedLocations[1].ID != structureID {
		t.Errorf("second insert: expected structure ID %d, got %d", structureID, insertedLocations[1].ID)
	}
	wantName := "Perimeter \u2014 Tranquility Trading Tower"
	if insertedLocations[1].Name != wantName {
		t.Errorf("structure name: got %q, want %q", insertedLocations[1].Name, wantName)
	}
}

// --- TestResolveLocationIDs_Structure_403_Skipped ---
// Verifies that a 403 from GetUniverseStructure does not insert a location record.
func TestResolveLocationIDs_Structure_403_Skipped(t *testing.T) {
	const structureID int64 = 1_000_000_000_002

	insertCalled := false

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: structureID, LocationFlag: "Hangar"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(_ store.InsertLocationParams) error {
			insertCalled = true
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, _ int64, _ string) (esi.UniverseStructure, error) {
			return esi.UniverseStructure{}, esi.ErrForbidden
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if insertCalled {
		t.Error("InsertLocation must not be called for a 403 structure response")
	}
}

// --- TestResolveLocationIDs_Structure_404_StoresSentinel ---
// Verifies that a 404 from GetUniverseStructure stores the sentinel name.
func TestResolveLocationIDs_Structure_404_StoresSentinel(t *testing.T) {
	const structureID int64 = 1_000_000_000_003

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: structureID, LocationFlag: "Hangar"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, _ int64, _ string) (esi.UniverseStructure, error) {
			return esi.UniverseStructure{}, esi.ErrNotFound
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if len(insertedLocations) != 1 {
		t.Fatalf("expected 1 InsertLocation call for 404 structure, got %d", len(insertedLocations))
	}
	if insertedLocations[0].ID != structureID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, structureID)
	}
	if insertedLocations[0].Name != corpHangarSentinel {
		t.Errorf("InsertLocation Name: got %q, want %q", insertedLocations[0].Name, corpHangarSentinel)
	}
}

// --- TestResolveLocationIDs_Structure_CachedSystemName ---
// Verifies that the system name is fetched from eve_locations cache rather than ESI
// when it was already resolved for a prior structure in the same system.
func TestResolveLocationIDs_Structure_CachedSystemName(t *testing.T) {
	const structureID int64 = 1_000_000_000_004
	const systemID int64 = 30000142

	systemESICallCount := 0
	var insertedIDs []int64

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: structureID, LocationFlag: "Hangar"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			if id == systemID {
				// System name already cached.
				return store.EveLocation{ID: id, Name: "Jita", ResolvedAt: time.Now()}, nil
			}
			// Structure not yet cached.
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedIDs = append(insertedIDs, arg.ID)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, _ int64, _ string) (esi.UniverseStructure, error) {
			return esi.UniverseStructure{Name: "My Fortizar", SolarSystemID: systemID}, nil
		},
		getUniverseSystemFunc: func(_ context.Context, _ int64) (string, error) {
			systemESICallCount++
			return "Jita", nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if systemESICallCount != 0 {
		t.Errorf("GetUniverseSystem must not be called when system name is already cached, got %d calls", systemESICallCount)
	}

	// Only the structure should be inserted (system was cached).
	if len(insertedIDs) != 1 || insertedIDs[0] != structureID {
		t.Errorf("expected only structure %d to be inserted, got %v", structureID, insertedIDs)
	}
}

// --- TestResolveLocationIDs_CorpHangar_ResolvesViaCorpAssets_NPCStation ---
// Verifies that a corp blueprint with CorpSAG flag resolves to station name via corp_assets + GetStation.
func TestResolveLocationIDs_CorpHangar_ResolvesViaCorpAssets_NPCStation(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_566
	const stationID int64 = 60015146
	const stationName = "Ibura IX - Moon 11 - Spacelane Patrol Testing Facilities"

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: officeItemID, LocationFlag: "CorpSAG3"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		getCorpAssetFunc: func(itemID int64) (store.GetCorpAssetRow, error) {
			if itemID != officeItemID {
				t.Errorf("GetCorpAsset: unexpected itemID %d", itemID)
			}
			return store.GetCorpAssetRow{LocationID: stationID, LocationType: "station"}, nil
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getStationFunc: func(_ context.Context, id int64) (string, error) {
			if id != stationID {
				t.Errorf("GetStation: unexpected id %d", id)
			}
			return stationName, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCorporation, 99000001)

	if len(insertedLocations) != 1 {
		t.Fatalf("expected 1 InsertLocation call, got %d", len(insertedLocations))
	}
	if insertedLocations[0].ID != officeItemID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, officeItemID)
	}
	if insertedLocations[0].Name != stationName {
		t.Errorf("InsertLocation Name: got %q, want %q", insertedLocations[0].Name, stationName)
	}
}

// --- TestResolveLocationIDs_CorpHangar_AssetNotFound_NoInsert ---
// Verifies that when corp_assets are not yet synced, no location record is stored.
// The location stays unresolved so the next cycle can retry after assets are populated.
func TestResolveLocationIDs_CorpHangar_AssetNotFound_NoInsert(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_567

	insertCalled := false

	q := &mockQuerier{
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: officeItemID, LocationFlag: "CorpSAG1"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		getCorpAssetFunc: func(_ int64) (store.GetCorpAssetRow, error) {
			return store.GetCorpAssetRow{}, errors.New("not found")
		},
		insertLocationFunc: func(_ store.InsertLocationParams) error {
			insertCalled = true
			return nil
		},
	}

	esiMock := &mockESIClient{}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCorporation, 99000001)

	if insertCalled {
		t.Error("InsertLocation must not be called when corp_assets not yet synced — location must stay unresolved so the next cycle can retry")
	}
}

// --- TestResolveLocationIDs_CorpHangar_AlreadyCached_NoESICalls ---
// Verifies that a corp hangar office item ID already in eve_locations is not re-resolved.
func TestResolveLocationIDs_CorpHangar_AlreadyCached_NoESICalls(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_568

	getCorpAssetCalled := false

	q := &mockQuerier{
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: officeItemID, LocationFlag: "CorpSAG2"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			// Already cached.
			return store.EveLocation{ID: id, Name: "Some Station", ResolvedAt: time.Now()}, nil
		},
		getCorpAssetFunc: func(_ int64) (store.GetCorpAssetRow, error) {
			getCorpAssetCalled = true
			return store.GetCorpAssetRow{}, nil
		},
	}

	esiMock := &mockESIClient{}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCorporation, 99000001)

	if getCorpAssetCalled {
		t.Error("GetCorpAsset must not be called when corp hangar location is already cached")
	}
}

// --- TestResolveLocationIDs_CorpHangar_ResolvesViaCorpAssets_Structure ---
// Verifies that a corp hangar flag with a structure ID in corp_assets resolves
// via GetUniverseStructure and stores "System — StructureName".
func TestResolveLocationIDs_CorpHangar_ResolvesViaCorpAssets_Structure(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_569
	const structureID int64 = 1_000_000_000_005
	const systemID int64 = 30000142

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: officeItemID, LocationFlag: "CorpSAG4"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			if id == systemID {
				return store.EveLocation{ID: id, Name: "Jita"}, nil
			}
			return store.EveLocation{}, errors.New("not found")
		},
		getCorpAssetFunc: func(itemID int64) (store.GetCorpAssetRow, error) {
			if itemID != officeItemID {
				t.Errorf("GetCorpAsset: unexpected itemID %d", itemID)
			}
			return store.GetCorpAssetRow{LocationID: structureID, LocationType: "structure"}, nil
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, id int64, _ string) (esi.UniverseStructure, error) {
			if id != structureID {
				t.Errorf("GetUniverseStructure: unexpected id %d", id)
			}
			return esi.UniverseStructure{Name: "Keepstar", SolarSystemID: systemID}, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCorporation, 99000001)

	if len(insertedLocations) != 1 {
		t.Fatalf("expected 1 InsertLocation call, got %d", len(insertedLocations))
	}
	if insertedLocations[0].ID != officeItemID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, officeItemID)
	}
	wantName := "Jita \u2014 Keepstar"
	if insertedLocations[0].Name != wantName {
		t.Errorf("InsertLocation Name: got %q, want %q", insertedLocations[0].Name, wantName)
	}
}

// --- TestResolveLocationIDs_CorpHangar_Structure_403_Skipped ---
// Verifies that a 403 from GetUniverseStructure for a corp hangar structure does not insert.
func TestResolveLocationIDs_CorpHangar_Structure_403_Skipped(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_570
	const structureID int64 = 1_000_000_000_006

	insertCalled := false

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: officeItemID, LocationFlag: "CorpDeliveries"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		getCorpAssetFunc: func(_ int64) (store.GetCorpAssetRow, error) {
			return store.GetCorpAssetRow{LocationID: structureID, LocationType: "structure"}, nil
		},
		insertLocationFunc: func(_ store.InsertLocationParams) error {
			insertCalled = true
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, _ int64, _ string) (esi.UniverseStructure, error) {
			return esi.UniverseStructure{}, esi.ErrForbidden
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCorporation, 99000001)

	if insertCalled {
		t.Error("InsertLocation must not be called for a 403 structure in corp hangar resolution")
	}
}

// --- TestResolveLocationIDs_CorpHangar_Structure_404_StoresSentinel ---
// Verifies that a 404 from GetUniverseStructure for a corp hangar stores sentinel.
func TestResolveLocationIDs_CorpHangar_Structure_404_StoresSentinel(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_571
	const structureID int64 = 1_000_000_000_007

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationsByOwnerFunc: func(_ store.ListBlueprintLocationsByOwnerParams) ([]store.ListBlueprintLocationsByOwnerRow, error) {
			return []store.ListBlueprintLocationsByOwnerRow{
				{LocationID: officeItemID, LocationFlag: "CorpSAG5"},
			}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		getCorpAssetFunc: func(_ int64) (store.GetCorpAssetRow, error) {
			return store.GetCorpAssetRow{LocationID: structureID, LocationType: "structure"}, nil
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseStructFunc: func(_ context.Context, _ int64, _ string) (esi.UniverseStructure, error) {
			return esi.UniverseStructure{}, esi.ErrNotFound
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCorporation, 99000001)

	if len(insertedLocations) != 1 {
		t.Fatalf("expected 1 InsertLocation call for sentinel, got %d", len(insertedLocations))
	}
	if insertedLocations[0].ID != officeItemID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, officeItemID)
	}
	if insertedLocations[0].Name != corpHangarSentinel {
		t.Errorf("InsertLocation Name (404): got %q, want %q", insertedLocations[0].Name, corpHangarSentinel)
	}
}
