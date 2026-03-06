package sync

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// --- TestResolveLocationIDs_NPCOnly_BulkResolves ---
// Verifies that NPC station IDs (< 1T) are resolved via PostUniverseNames
// and inserted into eve_locations.
func TestResolveLocationIDs_NPCOnly_BulkResolves(t *testing.T) {
	const stationID int64 = 60003760

	var insertedLocations []store.InsertLocationParams
	var postNamesCalledWith []int64

	q := &mockQuerier{
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{stationID}, nil
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
		postUniverseNamesFunc: func(_ context.Context, ids []int64) ([]esi.UniverseNamesEntry, error) {
			postNamesCalledWith = ids
			return []esi.UniverseNamesEntry{
				{ID: stationID, Name: "Jita IV - Moon 4 - Caldari Navy Assembly Plant", Category: "station"},
			}, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if len(postNamesCalledWith) != 1 || postNamesCalledWith[0] != stationID {
		t.Errorf("PostUniverseNames called with %v, want [%d]", postNamesCalledWith, stationID)
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
	postNamesCalled := false

	q := &mockQuerier{
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{60003760}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			// Already cached.
			return store.EveLocation{ID: id, Name: "Jita IV", ResolvedAt: time.Now()}, nil
		},
	}

	esiMock := &mockESIClient{
		postUniverseNamesFunc: func(_ context.Context, _ []int64) ([]esi.UniverseNamesEntry, error) {
			postNamesCalled = true
			return nil, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if postNamesCalled {
		t.Error("PostUniverseNames must not be called when all locations are already cached")
	}
}

// --- TestResolveLocationIDs_NoIDs_NoESICalls ---
// Verifies that no ESI calls are made when there are no location IDs.
func TestResolveLocationIDs_NoIDs_NoESICalls(t *testing.T) {
	q := &mockQuerier{
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
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
// Verifies that structure IDs (>= 1T) are resolved via GetUniverseStructure +
// GetUniverseSystem, and stored as "SystemName — StructureName".
func TestResolveLocationIDs_Structure_ResolvedWithSystemName(t *testing.T) {
	const structureID int64 = 1_000_000_000_001
	const systemID int64 = 30000142

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok123"}}, nil
		},
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{structureID}, nil
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
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{structureID}, nil
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

// --- TestResolveLocationIDs_CorpOffice_NotSentToPostNames ---
// Verifies that corporation office/hangar IDs ([64M, 1T)) are NOT sent to PostUniverseNames.
func TestResolveLocationIDs_CorpOffice_NotSentToPostNames(t *testing.T) {
	const officeID int64 = 100_000_000 // corp hangar range

	postNamesCalled := false
	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{officeID}, nil
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
		postUniverseNamesFunc: func(_ context.Context, _ []int64) ([]esi.UniverseNamesEntry, error) {
			postNamesCalled = true
			return nil, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if postNamesCalled {
		t.Error("PostUniverseNames must not be called for corporation office/hangar IDs")
	}
}

// --- TestResolveLocationIDs_CorpOffice_StoresSentinel ---
// Verifies that corporation office/hangar IDs get the sentinel "Corporation Hangar" stored.
func TestResolveLocationIDs_CorpOffice_StoresSentinel(t *testing.T) {
	const officeID int64 = 200_000_000 // corp hangar range

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{officeID}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(arg store.InsertLocationParams) error {
			insertedLocations = append(insertedLocations, arg)
			return nil
		},
	}

	esiMock := &mockESIClient{}

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	if len(insertedLocations) != 1 {
		t.Fatalf("expected 1 InsertLocation call for corp hangar, got %d", len(insertedLocations))
	}
	if insertedLocations[0].ID != officeID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, officeID)
	}
	if insertedLocations[0].Name != corpHangarSentinel {
		t.Errorf("InsertLocation Name: got %q, want %q", insertedLocations[0].Name, corpHangarSentinel)
	}
}

// --- TestResolveLocationIDs_PostNames_LogsWarningOnPartialResponse ---
// Verifies that a warning is logged when PostUniverseNames returns fewer entries than requested.
func TestResolveLocationIDs_PostNames_LogsWarningOnPartialResponse(t *testing.T) {
	const stationA int64 = 60003760
	const stationB int64 = 60000004

	q := &mockQuerier{
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{stationA, stationB}, nil
		},
		getLocationFunc: func(id int64) (store.EveLocation, error) {
			return store.EveLocation{}, errors.New("not found")
		},
		insertLocationFunc: func(_ store.InsertLocationParams) error { return nil },
	}

	esiMock := &mockESIClient{
		// ESI only returns one of the two requested IDs.
		postUniverseNamesFunc: func(_ context.Context, _ []int64) ([]esi.UniverseNamesEntry, error) {
			return []esi.UniverseNamesEntry{
				{ID: stationA, Name: "Jita IV - Moon 4 - Caldari Navy Assembly Plant", Category: "station"},
			}, nil
		},
	}

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	w := New(q, esiMock, time.Minute)
	w.resolveLocationIDs(context.Background(), ownerTypeCharacter, 1)

	logged := logBuf.String()
	if !strings.Contains(logged, "missing IDs") {
		t.Errorf("expected warning about missing IDs in log, got: %q", logged)
	}
}

// --- TestResolveLocationIDs_Structure_404_StoresSentinel ---
// Verifies that a 404 from GetUniverseStructure (corp office item ID >= 1T)
// stores the sentinel name rather than leaving the location unresolved.
func TestResolveLocationIDs_Structure_404_StoresSentinel(t *testing.T) {
	const officeItemID int64 = 1_052_718_829_566 // >= 1T but a corp office item

	var insertedLocations []store.InsertLocationParams

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{officeItemID}, nil
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
	if insertedLocations[0].ID != officeItemID {
		t.Errorf("InsertLocation ID: got %d, want %d", insertedLocations[0].ID, officeItemID)
	}
	if insertedLocations[0].Name != corpHangarSentinel {
		t.Errorf("InsertLocation Name: got %q, want %q", insertedLocations[0].Name, corpHangarSentinel)
	}
}

// --- TestResolveLocationIDs_Structure_CachedSystemName ---
// Verifies that the system name is fetched from eve_locations cache rather than ESI
// when it was already resolved for a prior structure in the same system.
func TestResolveLocationIDs_Structure_CachedSystemName(t *testing.T) {
	const structureID int64 = 1_000_000_000_003
	const systemID int64 = 30000142

	systemESICallCount := 0
	var insertedIDs []int64

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) {
			return []store.Character{{ID: 1, Name: "TestChar", AccessToken: "tok"}}, nil
		},
		listBlueprintLocationIDsByOwnerFunc: func(_ store.ListBlueprintLocationIDsByOwnerParams) ([]int64, error) {
			return []int64{structureID}, nil
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
