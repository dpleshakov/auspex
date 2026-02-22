package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// mockESIClient implements esi.Client for syncSubject tests.
// Methods not relevant to a specific test panic if called.
type mockESIClient struct {
	charBlueprintsFunc  func(context.Context, int64, string) ([]esi.Blueprint, time.Time, error)
	corpBlueprintsFunc  func(context.Context, int64, string) ([]esi.Blueprint, time.Time, error)
	charJobsFunc        func(context.Context, int64, string) ([]esi.Job, time.Time, error)
	corpJobsFunc        func(context.Context, int64, string) ([]esi.Job, time.Time, error)
	getUniverseTypeFunc func(context.Context, int64) (esi.UniverseType, error)
}

func (m *mockESIClient) GetCharacterBlueprints(ctx context.Context, id int64, token string) ([]esi.Blueprint, time.Time, error) {
	if m.charBlueprintsFunc != nil {
		return m.charBlueprintsFunc(ctx, id, token)
	}
	panic("unexpected call to GetCharacterBlueprints")
}

func (m *mockESIClient) GetCorporationBlueprints(ctx context.Context, id int64, token string) ([]esi.Blueprint, time.Time, error) {
	if m.corpBlueprintsFunc != nil {
		return m.corpBlueprintsFunc(ctx, id, token)
	}
	panic("unexpected call to GetCorporationBlueprints")
}

func (m *mockESIClient) GetCharacterJobs(ctx context.Context, id int64, token string) ([]esi.Job, time.Time, error) {
	if m.charJobsFunc != nil {
		return m.charJobsFunc(ctx, id, token)
	}
	panic("unexpected call to GetCharacterJobs")
}

func (m *mockESIClient) GetCorporationJobs(ctx context.Context, id int64, token string) ([]esi.Job, time.Time, error) {
	if m.corpJobsFunc != nil {
		return m.corpJobsFunc(ctx, id, token)
	}
	panic("unexpected call to GetCorporationJobs")
}

func (m *mockESIClient) GetUniverseType(ctx context.Context, typeID int64) (esi.UniverseType, error) {
	if m.getUniverseTypeFunc != nil {
		return m.getUniverseTypeFunc(ctx, typeID)
	}
	panic("unexpected call to GetUniverseType")
}

// Compile-time assertion: *mockESIClient must satisfy esi.Client.
var _ esi.Client = (*mockESIClient)(nil)

// --- TestSyncBlueprints_UpsertsAll ---
// Verifies that syncSubject upserts every blueprint returned by ESI and
// updates sync_state with the correct owner, endpoint, and cache_until.
func TestSyncBlueprints_UpsertsAll(t *testing.T) {
	const charID int64 = 42
	expiry := time.Now().Add(10 * time.Minute).Truncate(time.Second)

	bps := []esi.Blueprint{
		{ItemID: 1001, TypeID: 500, LocationID: 60000004, MELevel: 5, TELevel: 10},
		{ItemID: 1002, TypeID: 501, LocationID: 60000004, MELevel: 3, TELevel: 8},
	}

	var upsertedBPs []store.UpsertBlueprintParams
	var syncStateArg store.UpsertSyncStateParams

	q := &mockQuerier{
		upsertBlueprintFunc: func(arg store.UpsertBlueprintParams) error {
			upsertedBPs = append(upsertedBPs, arg)
			return nil
		},
		upsertSyncStateFunc: func(arg store.UpsertSyncStateParams) error {
			syncStateArg = arg
			return nil
		},
	}

	esiMock := &mockESIClient{
		charBlueprintsFunc: func(_ context.Context, id int64, _ string) ([]esi.Blueprint, time.Time, error) {
			if id != charID {
				t.Errorf("GetCharacterBlueprints: unexpected characterID %d", id)
			}
			return bps, expiry, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCharacter, charID, endpointBlueprints)

	if len(upsertedBPs) != len(bps) {
		t.Fatalf("expected %d blueprint upserts, got %d", len(bps), len(upsertedBPs))
	}
	for i, got := range upsertedBPs {
		want := bps[i]
		if got.ID != want.ItemID {
			t.Errorf("blueprint[%d]: ID got %d, want %d", i, got.ID, want.ItemID)
		}
		if got.TypeID != want.TypeID {
			t.Errorf("blueprint[%d]: TypeID got %d, want %d", i, got.TypeID, want.TypeID)
		}
		if got.MeLevel != want.MELevel {
			t.Errorf("blueprint[%d]: MeLevel got %d, want %d", i, got.MeLevel, want.MELevel)
		}
		if got.TeLevel != want.TELevel {
			t.Errorf("blueprint[%d]: TeLevel got %d, want %d", i, got.TeLevel, want.TELevel)
		}
		if got.OwnerType != ownerTypeCharacter {
			t.Errorf("blueprint[%d]: OwnerType got %q, want %q", i, got.OwnerType, ownerTypeCharacter)
		}
		if got.OwnerID != charID {
			t.Errorf("blueprint[%d]: OwnerID got %d, want %d", i, got.OwnerID, charID)
		}
	}

	if syncStateArg.Endpoint != endpointBlueprints {
		t.Errorf("sync_state Endpoint: got %q, want %q", syncStateArg.Endpoint, endpointBlueprints)
	}
	if syncStateArg.OwnerType != ownerTypeCharacter {
		t.Errorf("sync_state OwnerType: got %q, want %q", syncStateArg.OwnerType, ownerTypeCharacter)
	}
	if syncStateArg.OwnerID != charID {
		t.Errorf("sync_state OwnerID: got %d, want %d", syncStateArg.OwnerID, charID)
	}
	if !syncStateArg.CacheUntil.Equal(expiry) {
		t.Errorf("sync_state CacheUntil: got %v, want %v", syncStateArg.CacheUntil, expiry)
	}
}

// --- TestSyncJobs_UpsertAndPruneStale ---
// Verifies that syncSubject upserts incoming jobs, deletes stale jobs (present
// in store but absent from ESI response), and updates sync_state.
func TestSyncJobs_UpsertAndPruneStale(t *testing.T) {
	const charID int64 = 7
	expiry := time.Now().Add(5 * time.Minute).Truncate(time.Second)

	start := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	end := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	incomingJobs := []esi.Job{
		{
			JobID: 201, BlueprintID: 1001, InstallerID: charID,
			Activity: "me_research", Status: "active",
			StartDate: start, EndDate: end,
		},
	}
	// Job 202 is in the store but not in the ESI response → stale, must be deleted.
	existingIDs := []int64{201, 202}

	var upsertedJobs []store.UpsertJobParams
	var deletedIDs []int64
	var syncStateArg store.UpsertSyncStateParams

	q := &mockQuerier{
		listJobIDsByOwnerFunc: func(_ store.ListJobIDsByOwnerParams) ([]int64, error) {
			return existingIDs, nil
		},
		upsertJobFunc: func(arg store.UpsertJobParams) error {
			upsertedJobs = append(upsertedJobs, arg)
			return nil
		},
		deleteJobByIDFunc: func(id int64) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		},
		upsertSyncStateFunc: func(arg store.UpsertSyncStateParams) error {
			syncStateArg = arg
			return nil
		},
	}

	esiMock := &mockESIClient{
		charJobsFunc: func(_ context.Context, _ int64, _ string) ([]esi.Job, time.Time, error) {
			return incomingJobs, expiry, nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCharacter, charID, endpointJobs)

	// Verify upsert.
	if len(upsertedJobs) != 1 {
		t.Fatalf("expected 1 job upsert, got %d", len(upsertedJobs))
	}
	got := upsertedJobs[0]
	want := incomingJobs[0]
	if got.ID != want.JobID {
		t.Errorf("upserted job ID: got %d, want %d", got.ID, want.JobID)
	}
	if got.BlueprintID != want.BlueprintID {
		t.Errorf("upserted job BlueprintID: got %d, want %d", got.BlueprintID, want.BlueprintID)
	}
	if got.Activity != want.Activity {
		t.Errorf("upserted job Activity: got %q, want %q", got.Activity, want.Activity)
	}
	if got.Status != want.Status {
		t.Errorf("upserted job Status: got %q, want %q", got.Status, want.Status)
	}
	if got.OwnerType != ownerTypeCharacter {
		t.Errorf("upserted job OwnerType: got %q, want %q", got.OwnerType, ownerTypeCharacter)
	}
	if got.OwnerID != charID {
		t.Errorf("upserted job OwnerID: got %d, want %d", got.OwnerID, charID)
	}

	// Verify stale deletion.
	if len(deletedIDs) != 1 {
		t.Fatalf("expected 1 stale job deletion, got %d: %v", len(deletedIDs), deletedIDs)
	}
	if deletedIDs[0] != 202 {
		t.Errorf("deleted job ID: got %d, want 202", deletedIDs[0])
	}

	// Verify sync_state.
	if syncStateArg.Endpoint != endpointJobs {
		t.Errorf("sync_state Endpoint: got %q, want %q", syncStateArg.Endpoint, endpointJobs)
	}
	if !syncStateArg.CacheUntil.Equal(expiry) {
		t.Errorf("sync_state CacheUntil: got %v, want %v", syncStateArg.CacheUntil, expiry)
	}
}

// --- TestSyncSubject_ESIError_SkipsSyncState ---
// Verifies that when ESI returns an error, sync_state is NOT updated.
func TestSyncSubject_ESIError_SkipsSyncState(t *testing.T) {
	syncStateUpdated := false

	q := &mockQuerier{
		upsertSyncStateFunc: func(_ store.UpsertSyncStateParams) error {
			syncStateUpdated = true
			return nil
		},
	}

	esiMock := &mockESIClient{
		charBlueprintsFunc: func(_ context.Context, _ int64, _ string) ([]esi.Blueprint, time.Time, error) {
			return nil, time.Time{}, errors.New("ESI 503: service unavailable")
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCharacter, 1, endpointBlueprints)

	if syncStateUpdated {
		t.Error("sync_state must not be updated when ESI returns an error")
	}
}

// --- TestSyncBlueprints_CorporationOwner ---
// Verifies that GetCorporationBlueprints is called (not character) for corporation subjects.
func TestSyncBlueprints_CorporationOwner(t *testing.T) {
	const corpID int64 = 99

	corpCalled := false
	q := &mockQuerier{
		upsertSyncStateFunc: func(_ store.UpsertSyncStateParams) error { return nil },
	}
	esiMock := &mockESIClient{
		corpBlueprintsFunc: func(_ context.Context, id int64, _ string) ([]esi.Blueprint, time.Time, error) {
			corpCalled = true
			if id != corpID {
				t.Errorf("GetCorporationBlueprints: unexpected corpID %d", id)
			}
			return nil, time.Now().Add(time.Minute), nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCorporation, corpID, endpointBlueprints)

	if !corpCalled {
		t.Error("GetCorporationBlueprints was not called for corporation owner")
	}
}

// --- TestSyncJobs_CorporationOwner ---
// Verifies that GetCorporationJobs is called for corporation subjects.
func TestSyncJobs_CorporationOwner(t *testing.T) {
	const corpID int64 = 99

	corpCalled := false
	q := &mockQuerier{
		listJobIDsByOwnerFunc: func(_ store.ListJobIDsByOwnerParams) ([]int64, error) { return nil, nil },
		upsertSyncStateFunc:   func(_ store.UpsertSyncStateParams) error { return nil },
	}
	esiMock := &mockESIClient{
		corpJobsFunc: func(_ context.Context, id int64, _ string) ([]esi.Job, time.Time, error) {
			corpCalled = true
			if id != corpID {
				t.Errorf("GetCorporationJobs: unexpected corpID %d", id)
			}
			return nil, time.Now().Add(time.Minute), nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCorporation, corpID, endpointJobs)

	if !corpCalled {
		t.Error("GetCorporationJobs was not called for corporation owner")
	}
}

// --- TestSyncJobs_NoStaleJobs ---
// Verifies that no deletion calls occur when all stored job IDs are still in the ESI response.
func TestSyncJobs_NoStaleJobs(t *testing.T) {
	deleteCallCount := 0

	q := &mockQuerier{
		listJobIDsByOwnerFunc: func(_ store.ListJobIDsByOwnerParams) ([]int64, error) {
			return []int64{301}, nil // one existing job, same as incoming
		},
		upsertJobFunc: func(_ store.UpsertJobParams) error { return nil },
		deleteJobByIDFunc: func(_ int64) error {
			deleteCallCount++
			return nil
		},
		upsertSyncStateFunc: func(_ store.UpsertSyncStateParams) error { return nil },
	}

	esiMock := &mockESIClient{
		charJobsFunc: func(_ context.Context, _ int64, _ string) ([]esi.Job, time.Time, error) {
			return []esi.Job{
				{JobID: 301, BlueprintID: 1001, InstallerID: 1, Activity: "copying", Status: "active",
					StartDate: time.Now().Add(-time.Hour), EndDate: time.Now().Add(time.Hour)},
			}, time.Now().Add(time.Minute), nil
		},
	}

	w := New(q, esiMock, time.Minute)
	w.syncSubject(context.Background(), ownerTypeCharacter, 1, endpointJobs)

	if deleteCallCount != 0 {
		t.Errorf("expected no deletions when no stale jobs, got %d", deleteCallCount)
	}
}
