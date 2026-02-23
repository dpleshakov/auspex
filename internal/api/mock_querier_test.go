package api

import (
	"context"

	"github.com/dpleshakov/auspex/internal/store"
)

// mockQuerier implements store.Querier for tests.
// Each method delegates to the corresponding Fn field if non-nil;
// otherwise it returns the zero value and nil error.
type mockQuerier struct {
	ListCharactersFn          func(ctx context.Context) ([]store.Character, error)
	DeleteCharacterFn         func(ctx context.Context, id int64) error
	DeleteBlueprintsByOwnerFn func(ctx context.Context, arg store.DeleteBlueprintsByOwnerParams) error
	DeleteJobsByOwnerFn       func(ctx context.Context, arg store.DeleteJobsByOwnerParams) error
	DeleteSyncStateByOwnerFn  func(ctx context.Context, arg store.DeleteSyncStateByOwnerParams) error
	GetCharacterFn            func(ctx context.Context, id int64) (store.Character, error)
	ListCorporationsFn        func(ctx context.Context) ([]store.ListCorporationsRow, error)
	InsertCorporationFn       func(ctx context.Context, arg store.InsertCorporationParams) error
	DeleteCorporationFn       func(ctx context.Context, id int64) error

	ListBlueprintsFn         func(ctx context.Context, arg store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error)
	CountIdleBlueprintsFn    func(ctx context.Context) (int64, error)
	CountOverdueJobsFn       func(ctx context.Context) (int64, error)
	CountCompletingTodayFn   func(ctx context.Context) (int64, error)
	ListCharacterSlotUsageFn func(ctx context.Context) ([]store.ListCharacterSlotUsageRow, error)
	ListSyncStatusFn         func(ctx context.Context) ([]store.ListSyncStatusRow, error)
}

func (m *mockQuerier) ListCharacters(ctx context.Context) ([]store.Character, error) {
	if m.ListCharactersFn != nil {
		return m.ListCharactersFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) DeleteCharacter(ctx context.Context, id int64) error {
	if m.DeleteCharacterFn != nil {
		return m.DeleteCharacterFn(ctx, id)
	}
	return nil
}

func (m *mockQuerier) DeleteBlueprintsByOwner(ctx context.Context, arg store.DeleteBlueprintsByOwnerParams) error {
	if m.DeleteBlueprintsByOwnerFn != nil {
		return m.DeleteBlueprintsByOwnerFn(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) DeleteJobsByOwner(ctx context.Context, arg store.DeleteJobsByOwnerParams) error {
	if m.DeleteJobsByOwnerFn != nil {
		return m.DeleteJobsByOwnerFn(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) DeleteSyncStateByOwner(ctx context.Context, arg store.DeleteSyncStateByOwnerParams) error {
	if m.DeleteSyncStateByOwnerFn != nil {
		return m.DeleteSyncStateByOwnerFn(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) GetCharacter(ctx context.Context, id int64) (store.Character, error) {
	if m.GetCharacterFn != nil {
		return m.GetCharacterFn(ctx, id)
	}
	return store.Character{}, nil
}

func (m *mockQuerier) ListCorporations(ctx context.Context) ([]store.ListCorporationsRow, error) {
	if m.ListCorporationsFn != nil {
		return m.ListCorporationsFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) InsertCorporation(ctx context.Context, arg store.InsertCorporationParams) error {
	if m.InsertCorporationFn != nil {
		return m.InsertCorporationFn(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) DeleteCorporation(ctx context.Context, id int64) error {
	if m.DeleteCorporationFn != nil {
		return m.DeleteCorporationFn(ctx, id)
	}
	return nil
}

// ---- Stub implementations for unused Querier methods ----

func (m *mockQuerier) CountCompletingToday(ctx context.Context) (int64, error) {
	if m.CountCompletingTodayFn != nil {
		return m.CountCompletingTodayFn(ctx)
	}
	return 0, nil
}

func (m *mockQuerier) CountIdleBlueprints(ctx context.Context) (int64, error) {
	if m.CountIdleBlueprintsFn != nil {
		return m.CountIdleBlueprintsFn(ctx)
	}
	return 0, nil
}

func (m *mockQuerier) CountOverdueJobs(ctx context.Context) (int64, error) {
	if m.CountOverdueJobsFn != nil {
		return m.CountOverdueJobsFn(ctx)
	}
	return 0, nil
}

func (m *mockQuerier) DeleteJobByID(ctx context.Context, id int64) error { return nil }

func (m *mockQuerier) GetCorporation(ctx context.Context, id int64) (store.Corporation, error) {
	return store.Corporation{}, nil
}

func (m *mockQuerier) GetEveType(ctx context.Context, id int64) (store.EveType, error) {
	return store.EveType{}, nil
}

func (m *mockQuerier) GetSyncState(ctx context.Context, arg store.GetSyncStateParams) (store.SyncState, error) {
	return store.SyncState{}, nil
}

func (m *mockQuerier) InsertEveCategory(ctx context.Context, arg store.InsertEveCategoryParams) error {
	return nil
}

func (m *mockQuerier) InsertEveGroup(ctx context.Context, arg store.InsertEveGroupParams) error {
	return nil
}

func (m *mockQuerier) InsertEveType(ctx context.Context, arg store.InsertEveTypeParams) error {
	return nil
}

func (m *mockQuerier) ListBlueprintTypeIDsByOwner(ctx context.Context, arg store.ListBlueprintTypeIDsByOwnerParams) ([]int64, error) {
	return nil, nil
}

func (m *mockQuerier) ListBlueprints(ctx context.Context, arg store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
	if m.ListBlueprintsFn != nil {
		return m.ListBlueprintsFn(ctx, arg)
	}
	return nil, nil
}

func (m *mockQuerier) ListCharacterSlotUsage(ctx context.Context) ([]store.ListCharacterSlotUsageRow, error) {
	if m.ListCharacterSlotUsageFn != nil {
		return m.ListCharacterSlotUsageFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) ListJobIDsByOwner(ctx context.Context, arg store.ListJobIDsByOwnerParams) ([]int64, error) {
	return nil, nil
}

func (m *mockQuerier) ListSyncStatus(ctx context.Context) ([]store.ListSyncStatusRow, error) {
	if m.ListSyncStatusFn != nil {
		return m.ListSyncStatusFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) UpsertBlueprint(ctx context.Context, arg store.UpsertBlueprintParams) error {
	return nil
}

func (m *mockQuerier) UpsertCharacter(ctx context.Context, arg store.UpsertCharacterParams) error {
	return nil
}

func (m *mockQuerier) UpsertJob(ctx context.Context, arg store.UpsertJobParams) error { return nil }

func (m *mockQuerier) UpsertSyncState(ctx context.Context, arg store.UpsertSyncStateParams) error {
	return nil
}
