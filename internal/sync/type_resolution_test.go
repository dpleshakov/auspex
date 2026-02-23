package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// --- TestResolveTypeIDs_NewTypeTriggersESIAndInserts ---
// Verifies that a type_id absent from eve_types triggers an ESI fetch and
// all three inserts (category → group → type) with correct field values.
func TestResolveTypeIDs_NewTypeTriggersESIAndInserts(t *testing.T) {
	const (
		ownerID int64 = 42
		typeID  int64 = 500
	)

	ut := esi.UniverseType{
		TypeID:       typeID,
		TypeName:     "Rifter",
		GroupID:      25,
		GroupName:    "Frigate",
		CategoryID:   6,
		CategoryName: "Ship",
	}

	var insertedCategory store.InsertEveCategoryParams
	var insertedGroup store.InsertEveGroupParams
	var insertedType store.InsertEveTypeParams

	q := &mockQuerier{
		listBlueprintTypeIDsByOwnerFunc: func(_ store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
			return []int64{typeID}, nil
		},
		getEveTypeFunc: func(_ int64) (store.EveType, error) {
			return store.EveType{}, errors.New("not found")
		},
		insertEveCategoryFunc: func(arg store.InsertEveCategoryParams) error {
			insertedCategory = arg
			return nil
		},
		insertEveGroupFunc: func(arg store.InsertEveGroupParams) error {
			insertedGroup = arg
			return nil
		},
		insertEveTypeFunc: func(arg store.InsertEveTypeParams) error {
			insertedType = arg
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseTypeFunc: func(_ context.Context, id int64) (esi.UniverseType, error) {
			if id != typeID {
				t.Errorf("GetUniverseType: unexpected typeID %d", id)
			}
			return ut, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveTypeIDs(context.Background(), ownerTypeCharacter, ownerID)

	// Verify category insert.
	if insertedCategory.ID != ut.CategoryID {
		t.Errorf("InsertEveCategory ID: got %d, want %d", insertedCategory.ID, ut.CategoryID)
	}
	if insertedCategory.Name != ut.CategoryName {
		t.Errorf("InsertEveCategory Name: got %q, want %q", insertedCategory.Name, ut.CategoryName)
	}

	// Verify group insert.
	if insertedGroup.ID != ut.GroupID {
		t.Errorf("InsertEveGroup ID: got %d, want %d", insertedGroup.ID, ut.GroupID)
	}
	if insertedGroup.CategoryID != ut.CategoryID {
		t.Errorf("InsertEveGroup CategoryID: got %d, want %d", insertedGroup.CategoryID, ut.CategoryID)
	}
	if insertedGroup.Name != ut.GroupName {
		t.Errorf("InsertEveGroup Name: got %q, want %q", insertedGroup.Name, ut.GroupName)
	}

	// Verify type insert.
	if insertedType.ID != typeID {
		t.Errorf("InsertEveType ID: got %d, want %d", insertedType.ID, typeID)
	}
	if insertedType.GroupID != ut.GroupID {
		t.Errorf("InsertEveType GroupID: got %d, want %d", insertedType.GroupID, ut.GroupID)
	}
	if insertedType.Name != ut.TypeName {
		t.Errorf("InsertEveType Name: got %q, want %q", insertedType.Name, ut.TypeName)
	}
}

// --- TestResolveTypeIDs_KnownTypeSkipped ---
// Verifies that a type_id already present in eve_types does not trigger an
// ESI call (GetUniverseType must not be called).
func TestResolveTypeIDs_KnownTypeSkipped(t *testing.T) {
	const typeID int64 = 500

	esiCalled := false

	q := &mockQuerier{
		listBlueprintTypeIDsByOwnerFunc: func(_ store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
			return []int64{typeID}, nil
		},
		getEveTypeFunc: func(id int64) (store.EveType, error) {
			// Type is known.
			return store.EveType{ID: id, Name: "Rifter"}, nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseTypeFunc: func(_ context.Context, _ int64) (esi.UniverseType, error) {
			esiCalled = true
			return esi.UniverseType{}, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveTypeIDs(context.Background(), ownerTypeCharacter, 1)

	if esiCalled {
		t.Error("GetUniverseType must not be called for known type_ids")
	}
}

// --- TestResolveTypeIDs_MultipleTypes_MixedKnownAndNew ---
// Verifies that with multiple type_ids, known ones are skipped and only
// new ones trigger ESI calls.
func TestResolveTypeIDs_MultipleTypes_MixedKnownAndNew(t *testing.T) {
	const (
		knownTypeID int64 = 500
		newTypeID   int64 = 501
	)

	var esiCallIDs []int64

	q := &mockQuerier{
		listBlueprintTypeIDsByOwnerFunc: func(_ store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
			return []int64{knownTypeID, newTypeID}, nil
		},
		getEveTypeFunc: func(id int64) (store.EveType, error) {
			if id == knownTypeID {
				return store.EveType{ID: id, Name: "Rifter"}, nil // known
			}
			return store.EveType{}, errors.New("not found") // new
		},
		insertEveCategoryFunc: func(_ store.InsertEveCategoryParams) error { return nil },
		insertEveGroupFunc:    func(_ store.InsertEveGroupParams) error { return nil },
		insertEveTypeFunc:     func(_ store.InsertEveTypeParams) error { return nil },
	}

	esiMock := &mockESIClient{
		getUniverseTypeFunc: func(_ context.Context, id int64) (esi.UniverseType, error) {
			esiCallIDs = append(esiCallIDs, id)
			return esi.UniverseType{TypeID: id, TypeName: "New Ship", GroupID: 1, GroupName: "G", CategoryID: 1, CategoryName: "C"}, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveTypeIDs(context.Background(), ownerTypeCharacter, 1)

	if len(esiCallIDs) != 1 {
		t.Fatalf("expected 1 ESI call, got %d: %v", len(esiCallIDs), esiCallIDs)
	}
	if esiCallIDs[0] != newTypeID {
		t.Errorf("ESI called with typeID %d, want %d", esiCallIDs[0], newTypeID)
	}
}

// --- TestResolveTypeIDs_CalledAfterBlueprintSync ---
// Verifies that resolveTypeIDs is invoked after a successful blueprint sync
// (detected via ListBlueprintTypeIDsByOwner being called).
func TestResolveTypeIDs_CalledAfterBlueprintSync(t *testing.T) {
	const charID int64 = 42

	typeIDsResolved := false

	q := &mockQuerier{
		// Pre-upsert resolveTypeIDsList checks eve_types; return "known" so no ESI fetch is needed.
		getEveTypeFunc: func(id int64) (store.EveType, error) {
			return store.EveType{ID: id}, nil
		},
		upsertBlueprintFunc: func(_ store.UpsertBlueprintParams) error { return nil },
		upsertSyncStateFunc: func(_ store.UpsertSyncStateParams) error { return nil },
		listBlueprintTypeIDsByOwnerFunc: func(_ store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
			typeIDsResolved = true
			return nil, nil // no type_ids to resolve; just confirms the call happened
		},
	}

	esiMock := &mockESIClient{
		charBlueprintsFunc: func(_ context.Context, _ int64, _ string) ([]esi.Blueprint, time.Time, error) {
			return []esi.Blueprint{{ItemID: 1, TypeID: 500}}, time.Now().Add(time.Minute), nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCharacter, charID, endpointBlueprints)

	if !typeIDsResolved {
		t.Error("resolveTypeIDs (ListBlueprintTypeIDsByOwner) was not called after successful blueprint sync")
	}
}

// --- TestResolveTypeIDs_NotCalledAfterBlueprintError ---
// Verifies that resolveTypeIDs is NOT called when the blueprint ESI fetch fails.
func TestResolveTypeIDs_NotCalledAfterBlueprintError(t *testing.T) {
	typeIDsResolved := false

	q := &mockQuerier{
		listBlueprintTypeIDsByOwnerFunc: func(_ store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
			typeIDsResolved = true
			return nil, nil
		},
	}

	esiMock := &mockESIClient{
		charBlueprintsFunc: func(_ context.Context, _ int64, _ string) ([]esi.Blueprint, time.Time, error) {
			return nil, time.Time{}, errors.New("ESI 503: service unavailable")
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCharacter, 1, endpointBlueprints)

	if typeIDsResolved {
		t.Error("resolveTypeIDs must not be called when blueprint sync fails")
	}
}

// --- TestResolveTypeIDs_ESIErrorSkipsType ---
// Verifies that an ESI error for one type_id does not abort resolution of
// subsequent type_ids (errors are non-fatal per type).
func TestResolveTypeIDs_ESIErrorSkipsType(t *testing.T) {
	const (
		badTypeID  int64 = 500
		goodTypeID int64 = 501
	)

	var resolvedTypes []int64

	q := &mockQuerier{
		listBlueprintTypeIDsByOwnerFunc: func(_ store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
			return []int64{badTypeID, goodTypeID}, nil
		},
		getEveTypeFunc: func(_ int64) (store.EveType, error) {
			return store.EveType{}, errors.New("not found")
		},
		insertEveCategoryFunc: func(_ store.InsertEveCategoryParams) error { return nil },
		insertEveGroupFunc:    func(_ store.InsertEveGroupParams) error { return nil },
		insertEveTypeFunc: func(arg store.InsertEveTypeParams) error {
			resolvedTypes = append(resolvedTypes, arg.ID)
			return nil
		},
	}

	esiMock := &mockESIClient{
		getUniverseTypeFunc: func(_ context.Context, id int64) (esi.UniverseType, error) {
			if id == badTypeID {
				return esi.UniverseType{}, errors.New("ESI 404: type not found")
			}
			return esi.UniverseType{TypeID: id, TypeName: "Good Ship", GroupID: 1, GroupName: "G", CategoryID: 1, CategoryName: "C"}, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.resolveTypeIDs(context.Background(), ownerTypeCharacter, 1)

	if len(resolvedTypes) != 1 {
		t.Fatalf("expected 1 resolved type, got %d: %v", len(resolvedTypes), resolvedTypes)
	}
	if resolvedTypes[0] != goodTypeID {
		t.Errorf("resolved type ID: got %d, want %d", resolvedTypes[0], goodTypeID)
	}
}
