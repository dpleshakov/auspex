package sync

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

// mockQuerier implements store.Querier for worker unit tests.
// Methods exercised by the worker are wired through function fields so each
// test can control their behavior. Unset fields panic — an unexpected call
// indicates a test bug.
type mockQuerier struct {
	// TASK-09: scheduling loop
	listCharsFunc func() ([]store.Character, error)
	listCorpsFunc func() ([]store.ListCorporationsRow, error)
	getSyncFunc   func(store.GetSyncStateParams) (store.SyncState, error)

	// TASK-10: syncSubject
	upsertBlueprintFunc   func(store.UpsertBlueprintParams) error
	upsertJobFunc         func(store.UpsertJobParams) error
	listJobIDsByOwnerFunc func(store.ListJobIDsByOwnerParams) ([]int64, error)
	deleteJobByIDFunc     func(int64) error
	upsertSyncStateFunc   func(store.UpsertSyncStateParams) error

	// TASK-11: type resolution
	listBlueprintTypeIDsByOwnerFunc func(store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error)
	getEveTypeFunc                  func(int64) (store.EveType, error)
	insertEveCategoryFunc           func(store.InsertEveCategoryParams) error
	insertEveGroupFunc              func(store.InsertEveGroupParams) error
	insertEveTypeFunc               func(store.InsertEveTypeParams) error
}

func (m *mockQuerier) ListCharacters(_ context.Context) ([]store.Character, error) {
	if m.listCharsFunc != nil {
		return m.listCharsFunc()
	}
	return nil, nil
}

func (m *mockQuerier) ListCorporations(_ context.Context) ([]store.ListCorporationsRow, error) {
	if m.listCorpsFunc != nil {
		return m.listCorpsFunc()
	}
	return nil, nil
}

func (m *mockQuerier) GetSyncState(_ context.Context, arg store.GetSyncStateParams) (store.SyncState, error) {
	if m.getSyncFunc != nil {
		return m.getSyncFunc(arg)
	}
	return store.SyncState{}, errors.New("not found")
}

// Stub implementations for methods not exercised in TASK-09 tests.
func (m *mockQuerier) CountCompletingToday(_ context.Context) (int64, error) {
	panic("unexpected call to CountCompletingToday")
}
func (m *mockQuerier) CountIdleBlueprints(_ context.Context) (int64, error) {
	panic("unexpected call to CountIdleBlueprints")
}
func (m *mockQuerier) CountOverdueJobs(_ context.Context) (int64, error) {
	panic("unexpected call to CountOverdueJobs")
}
func (m *mockQuerier) DeleteBlueprintsByOwner(_ context.Context, _ store.DeleteBlueprintsByOwnerParams) error {
	panic("unexpected call to DeleteBlueprintsByOwner")
}
func (m *mockQuerier) DeleteCharacter(_ context.Context, _ int64) error {
	panic("unexpected call to DeleteCharacter")
}
func (m *mockQuerier) DeleteCorporation(_ context.Context, _ int64) error {
	panic("unexpected call to DeleteCorporation")
}
func (m *mockQuerier) DeleteJobByID(_ context.Context, id int64) error {
	if m.deleteJobByIDFunc != nil {
		return m.deleteJobByIDFunc(id)
	}
	panic("unexpected call to DeleteJobByID")
}
func (m *mockQuerier) DeleteJobsByOwner(_ context.Context, _ store.DeleteJobsByOwnerParams) error {
	panic("unexpected call to DeleteJobsByOwner")
}
func (m *mockQuerier) DeleteSyncStateByOwner(_ context.Context, _ store.DeleteSyncStateByOwnerParams) error {
	panic("unexpected call to DeleteSyncStateByOwner")
}
func (m *mockQuerier) GetCharacter(_ context.Context, _ int64) (store.Character, error) {
	panic("unexpected call to GetCharacter")
}
func (m *mockQuerier) GetCorporation(_ context.Context, _ int64) (store.Corporation, error) {
	panic("unexpected call to GetCorporation")
}
func (m *mockQuerier) GetEveType(_ context.Context, id int64) (store.EveType, error) {
	if m.getEveTypeFunc != nil {
		return m.getEveTypeFunc(id)
	}
	panic("unexpected call to GetEveType")
}
func (m *mockQuerier) InsertCorporation(_ context.Context, _ store.InsertCorporationParams) error {
	panic("unexpected call to InsertCorporation")
}
func (m *mockQuerier) InsertEveCategory(_ context.Context, arg store.InsertEveCategoryParams) error {
	if m.insertEveCategoryFunc != nil {
		return m.insertEveCategoryFunc(arg)
	}
	panic("unexpected call to InsertEveCategory")
}
func (m *mockQuerier) InsertEveGroup(_ context.Context, arg store.InsertEveGroupParams) error {
	if m.insertEveGroupFunc != nil {
		return m.insertEveGroupFunc(arg)
	}
	panic("unexpected call to InsertEveGroup")
}
func (m *mockQuerier) InsertEveType(_ context.Context, arg store.InsertEveTypeParams) error {
	if m.insertEveTypeFunc != nil {
		return m.insertEveTypeFunc(arg)
	}
	panic("unexpected call to InsertEveType")
}
func (m *mockQuerier) ListBlueprintTypeIDsByOwner(_ context.Context, arg store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
	if m.listBlueprintTypeIDsByOwnerFunc != nil {
		return m.listBlueprintTypeIDsByOwnerFunc(arg)
	}
	// Default: empty list — resolveTypeIDs becomes a no-op, existing tests unaffected.
	return nil, nil
}
func (m *mockQuerier) ListBlueprints(_ context.Context, _ store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
	panic("unexpected call to ListBlueprints")
}
func (m *mockQuerier) ListCharacterSlotUsage(_ context.Context) ([]store.ListCharacterSlotUsageRow, error) {
	panic("unexpected call to ListCharacterSlotUsage")
}
func (m *mockQuerier) ListJobIDsByOwner(_ context.Context, arg store.ListJobIDsByOwnerParams) ([]int64, error) {
	if m.listJobIDsByOwnerFunc != nil {
		return m.listJobIDsByOwnerFunc(arg)
	}
	panic("unexpected call to ListJobIDsByOwner")
}
func (m *mockQuerier) ListSyncStatus(_ context.Context) ([]store.ListSyncStatusRow, error) {
	panic("unexpected call to ListSyncStatus")
}
func (m *mockQuerier) UpsertBlueprint(_ context.Context, arg store.UpsertBlueprintParams) error {
	if m.upsertBlueprintFunc != nil {
		return m.upsertBlueprintFunc(arg)
	}
	panic("unexpected call to UpsertBlueprint")
}
func (m *mockQuerier) UpsertCharacter(_ context.Context, _ store.UpsertCharacterParams) error {
	panic("unexpected call to UpsertCharacter")
}
func (m *mockQuerier) UpsertJob(_ context.Context, arg store.UpsertJobParams) error {
	if m.upsertJobFunc != nil {
		return m.upsertJobFunc(arg)
	}
	panic("unexpected call to UpsertJob")
}
func (m *mockQuerier) UpsertSyncState(_ context.Context, arg store.UpsertSyncStateParams) error {
	if m.upsertSyncStateFunc != nil {
		return m.upsertSyncStateFunc(arg)
	}
	panic("unexpected call to UpsertSyncState")
}

// Compile-time assertion: *mockQuerier must satisfy store.Querier.
var _ store.Querier = (*mockQuerier)(nil)

// --- helpers ---

func oneChar(id int64) func() ([]store.Character, error) {
	return func() ([]store.Character, error) {
		return []store.Character{{ID: id, Name: "TestChar"}}, nil
	}
}

func noCorps() func() ([]store.ListCorporationsRow, error) {
	return func() ([]store.ListCorporationsRow, error) { return nil, nil }
}

func freshState(until time.Time) func(store.GetSyncStateParams) (store.SyncState, error) {
	return func(p store.GetSyncStateParams) (store.SyncState, error) {
		return store.SyncState{CacheUntil: until}, nil
	}
}

func expiredState(until time.Time) func(store.GetSyncStateParams) (store.SyncState, error) {
	return freshState(until)
}

// --- tests ---

// TestCacheFresh_SubjectSkipped verifies that a subject with a future cache_until
// does not trigger a sync call.
func TestCacheFresh_SubjectSkipped(t *testing.T) {
	q := &mockQuerier{
		listCharsFunc: oneChar(1),
		listCorpsFunc: noCorps(),
		getSyncFunc:   freshState(time.Now().Add(time.Hour)),
	}

	var syncCalls []string
	w := New(q, nil, time.Minute)
	w.syncFn = func(_ context.Context, ownerType string, ownerID int64, endpoint string) {
		syncCalls = append(syncCalls, fmt.Sprintf("%s:%d:%s", ownerType, ownerID, endpoint))
	}

	w.runCycle(context.Background(), false)

	if len(syncCalls) != 0 {
		t.Errorf("expected no sync calls for fresh cache, got %v", syncCalls)
	}
}

// TestCacheExpired_SubjectSynced verifies that a subject with a past cache_until
// triggers a sync call for each endpoint.
func TestCacheExpired_SubjectSynced(t *testing.T) {
	const charID int64 = 42

	q := &mockQuerier{
		listCharsFunc: oneChar(charID),
		listCorpsFunc: noCorps(),
		getSyncFunc:   expiredState(time.Now().Add(-time.Hour)),
	}

	var synced []string
	w := New(q, nil, time.Minute)
	w.syncFn = func(_ context.Context, ownerType string, ownerID int64, endpoint string) {
		synced = append(synced, fmt.Sprintf("%s:%d:%s", ownerType, ownerID, endpoint))
	}

	w.runCycle(context.Background(), false)

	// Expect blueprints + jobs for the one character.
	want := []string{
		fmt.Sprintf("%s:%d:%s", ownerTypeCharacter, charID, endpointBlueprints),
		fmt.Sprintf("%s:%d:%s", ownerTypeCharacter, charID, endpointJobs),
	}
	if len(synced) != len(want) {
		t.Fatalf("expected %d sync calls, got %d: %v", len(want), len(synced), synced)
	}
	for i, s := range synced {
		if s != want[i] {
			t.Errorf("sync call %d: got %q, want %q", i, s, want[i])
		}
	}
}

// TestNeverSynced_TreatedAsExpired verifies that a subject with no sync_state record
// (GetSyncState returns an error) is treated as expired and synced.
func TestNeverSynced_TreatedAsExpired(t *testing.T) {
	q := &mockQuerier{
		listCharsFunc: oneChar(7),
		listCorpsFunc: noCorps(),
		getSyncFunc: func(_ store.GetSyncStateParams) (store.SyncState, error) {
			return store.SyncState{}, errors.New("sql: no rows")
		},
	}

	var syncCalls int
	w := New(q, nil, time.Minute)
	w.syncFn = func(_ context.Context, _ string, _ int64, _ string) { syncCalls++ }

	w.runCycle(context.Background(), false)

	if syncCalls != 2 {
		t.Errorf("expected 2 sync calls for never-synced subject, got %d", syncCalls)
	}
}

// TestForceRefresh_IgnoresFreshCache verifies that force=true causes all subjects
// to be synced even when their cache is still valid.
func TestForceRefresh_IgnoresFreshCache(t *testing.T) {
	q := &mockQuerier{
		listCharsFunc: oneChar(1),
		listCorpsFunc: noCorps(),
		getSyncFunc:   freshState(time.Now().Add(time.Hour)), // definitely fresh
	}

	var syncCalls int
	w := New(q, nil, time.Minute)
	w.syncFn = func(_ context.Context, _ string, _ int64, _ string) { syncCalls++ }

	w.runCycle(context.Background(), true) // force=true

	// blueprints + jobs for the one character, despite fresh cache.
	if syncCalls != 2 {
		t.Errorf("expected 2 sync calls with force=true, got %d", syncCalls)
	}
}

// TestCorporation_CacheExpired_Synced verifies that corporations are also iterated
// and synced when their cache is expired.
func TestCorporation_CacheExpired_Synced(t *testing.T) {
	const corpID int64 = 99

	q := &mockQuerier{
		listCharsFunc: func() ([]store.Character, error) { return nil, nil },
		listCorpsFunc: func() ([]store.ListCorporationsRow, error) {
			return []store.ListCorporationsRow{{ID: corpID, Name: "TestCorp"}}, nil
		},
		getSyncFunc: expiredState(time.Now().Add(-time.Hour)),
	}

	var synced []string
	w := New(q, nil, time.Minute)
	w.syncFn = func(_ context.Context, ownerType string, ownerID int64, endpoint string) {
		synced = append(synced, fmt.Sprintf("%s:%d:%s", ownerType, ownerID, endpoint))
	}

	w.runCycle(context.Background(), false)

	want := []string{
		fmt.Sprintf("%s:%d:%s", ownerTypeCorporation, corpID, endpointBlueprints),
		fmt.Sprintf("%s:%d:%s", ownerTypeCorporation, corpID, endpointJobs),
	}
	if len(synced) != len(want) {
		t.Fatalf("expected %d sync calls, got %d: %v", len(want), len(synced), synced)
	}
	for i, s := range synced {
		if s != want[i] {
			t.Errorf("sync call %d: got %q, want %q", i, s, want[i])
		}
	}
}

// TestRun_StopsOnContextCancel verifies that Run returns promptly when ctx is canceled.
func TestRun_StopsOnContextCancel(t *testing.T) {
	q := &mockQuerier{
		// Use a long interval so the ticker doesn't fire during the test.
		listCharsFunc: func() ([]store.Character, error) { return nil, nil },
		listCorpsFunc: noCorps(),
	}

	w := New(q, nil, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// success: Run exited after cancel
	case <-time.After(time.Second):
		t.Fatal("Run did not stop within 1s after context cancellation")
	}
}

// TestForceRefresh_Channel verifies that calling ForceRefresh unblocks a waiting
// Run loop and causes a cycle to execute.
func TestForceRefresh_Channel(t *testing.T) {
	synced := make(chan struct{}, 10)

	q := &mockQuerier{
		listCharsFunc: oneChar(1),
		listCorpsFunc: noCorps(),
		getSyncFunc:   freshState(time.Now().Add(time.Hour)), // fresh — only fire on force
	}

	// Use a very long ticker so only the force-refresh triggers a cycle after startup.
	w := New(q, nil, time.Hour)
	// Override now so the initial cycle sees everything as expired — we only care
	// about the force-refresh cycle triggering; use sync calls as the signal.
	var callCount int
	w.syncFn = func(_ context.Context, _ string, _ int64, _ string) {
		callCount++
		synced <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// The initial runCycle fires but cache is fresh, so no sync calls expected there.
	// Now send a force-refresh; it should bypass the fresh cache.
	w.ForceRefresh()

	// Wait for at least one sync call triggered by the force-refresh.
	select {
	case <-synced:
		// success
	case <-time.After(time.Second):
		t.Fatal("ForceRefresh did not trigger a sync within 1s")
	}
}
