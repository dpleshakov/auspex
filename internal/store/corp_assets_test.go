package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/db"
	"github.com/dpleshakov/auspex/internal/store"
)

// openTestDB opens an in-memory SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func TestUpsertAndGetCorpAsset(t *testing.T) {
	sqlDB := openTestDB(t)
	q := store.New(sqlDB)
	ctx := context.Background()

	err := q.UpsertCorpAsset(ctx, store.UpsertCorpAssetParams{
		ItemID:       1052718829566,
		OwnerID:      99000001,
		LocationID:   60015146,
		LocationType: "station",
	})
	if err != nil {
		t.Fatalf("UpsertCorpAsset: %v", err)
	}

	row, err := q.GetCorpAsset(ctx, 1052718829566)
	if err != nil {
		t.Fatalf("GetCorpAsset: %v", err)
	}
	if row.LocationID != 60015146 {
		t.Errorf("LocationID: got %d, want 60015146", row.LocationID)
	}
	if row.LocationType != "station" {
		t.Errorf("LocationType: got %q, want %q", row.LocationType, "station")
	}
}

func TestUpsertCorpAsset_Replaces(t *testing.T) {
	sqlDB := openTestDB(t)
	q := store.New(sqlDB)
	ctx := context.Background()

	q.UpsertCorpAsset(ctx, store.UpsertCorpAssetParams{ //nolint:errcheck
		ItemID: 100, OwnerID: 1, LocationID: 60000001, LocationType: "station",
	})
	// Replace with updated location.
	err := q.UpsertCorpAsset(ctx, store.UpsertCorpAssetParams{
		ItemID: 100, OwnerID: 1, LocationID: 60000002, LocationType: "station",
	})
	if err != nil {
		t.Fatalf("UpsertCorpAsset replace: %v", err)
	}

	row, err := q.GetCorpAsset(ctx, 100)
	if err != nil {
		t.Fatalf("GetCorpAsset: %v", err)
	}
	if row.LocationID != 60000002 {
		t.Errorf("LocationID after replace: got %d, want 60000002", row.LocationID)
	}
}

func TestDeleteCorpAssetsByOwner(t *testing.T) {
	sqlDB := openTestDB(t)
	q := store.New(sqlDB)
	ctx := context.Background()

	for _, item := range []store.UpsertCorpAssetParams{
		{ItemID: 1, OwnerID: 10, LocationID: 60000001, LocationType: "station"},
		{ItemID: 2, OwnerID: 10, LocationID: 60000002, LocationType: "station"},
		{ItemID: 3, OwnerID: 20, LocationID: 60000003, LocationType: "station"},
	} {
		if err := q.UpsertCorpAsset(ctx, item); err != nil {
			t.Fatalf("UpsertCorpAsset: %v", err)
		}
	}

	if err := q.DeleteCorpAssetsByOwner(ctx, 10); err != nil {
		t.Fatalf("DeleteCorpAssetsByOwner: %v", err)
	}

	// owner 10 assets should be gone
	if _, err := q.GetCorpAsset(ctx, 1); err == nil {
		t.Error("item_id=1 (owner 10) should be deleted")
	}
	if _, err := q.GetCorpAsset(ctx, 2); err == nil {
		t.Error("item_id=2 (owner 10) should be deleted")
	}
	// owner 20 asset should remain
	if _, err := q.GetCorpAsset(ctx, 3); err != nil {
		t.Errorf("item_id=3 (owner 20) should still exist: %v", err)
	}
}

// seedBlueprintPrereqs inserts the minimal eve_types/groups/categories rows
// required to satisfy blueprint FK constraints.
func seedBlueprintPrereqs(t *testing.T, sqlDB *sql.DB, typeID int64) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT OR IGNORE INTO eve_categories (id, name) VALUES (1, 'TestCat')`,
	)
	if err != nil {
		t.Fatalf("insert eve_category: %v", err)
	}
	_, err = sqlDB.Exec(
		`INSERT OR IGNORE INTO eve_groups (id, category_id, name) VALUES (1, 1, 'TestGroup')`,
	)
	if err != nil {
		t.Fatalf("insert eve_group: %v", err)
	}
	_, err = sqlDB.Exec(
		`INSERT OR IGNORE INTO eve_types (id, group_id, name) VALUES (?, 1, 'TestType')`, typeID,
	)
	if err != nil {
		t.Fatalf("insert eve_type %d: %v", typeID, err)
	}
}

func TestUpsertBlueprint_LocationFlagStored(t *testing.T) {
	sqlDB := openTestDB(t)
	q := store.New(sqlDB)
	ctx := context.Background()

	const typeID = int64(5000)
	seedBlueprintPrereqs(t, sqlDB, typeID)

	err := q.UpsertBlueprint(ctx, store.UpsertBlueprintParams{
		ID:           1052548174037,
		OwnerType:    "corporation",
		OwnerID:      99000001,
		TypeID:       typeID,
		LocationID:   1052718829566,
		LocationFlag: "CorpSAG3",
		MeLevel:      10,
		TeLevel:      20,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("UpsertBlueprint: %v", err)
	}

	rows, err := q.ListBlueprintLocationsByOwner(ctx, store.ListBlueprintLocationsByOwnerParams{
		OwnerType: "corporation",
		OwnerID:   99000001,
	})
	if err != nil {
		t.Fatalf("ListBlueprintLocationsByOwner: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].LocationID != 1052718829566 {
		t.Errorf("LocationID: got %d, want 1052718829566", rows[0].LocationID)
	}
	if rows[0].LocationFlag != "CorpSAG3" {
		t.Errorf("LocationFlag: got %q, want %q", rows[0].LocationFlag, "CorpSAG3")
	}
}

func TestListBlueprintLocationsByOwner_DeduplicatesLocations(t *testing.T) {
	sqlDB := openTestDB(t)
	q := store.New(sqlDB)
	ctx := context.Background()

	seedBlueprintPrereqs(t, sqlDB, 5000)
	seedBlueprintPrereqs(t, sqlDB, 5001)

	for _, bp := range []store.UpsertBlueprintParams{
		{ID: 1, OwnerType: "corporation", OwnerID: 1, TypeID: 5000, LocationID: 60000001, LocationFlag: "CorpSAG1", UpdatedAt: time.Now()},
		{ID: 2, OwnerType: "corporation", OwnerID: 1, TypeID: 5001, LocationID: 60000001, LocationFlag: "CorpSAG1", UpdatedAt: time.Now()},
	} {
		if err := q.UpsertBlueprint(ctx, bp); err != nil {
			t.Fatalf("UpsertBlueprint %d: %v", bp.ID, err)
		}
	}

	rows, err := q.ListBlueprintLocationsByOwner(ctx, store.ListBlueprintLocationsByOwnerParams{
		OwnerType: "corporation", OwnerID: 1,
	})
	if err != nil {
		t.Fatalf("ListBlueprintLocationsByOwner: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("got %d rows, want 1 (DISTINCT)", len(rows))
	}
}
