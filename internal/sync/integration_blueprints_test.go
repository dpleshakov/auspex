package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// charBlueprintRoutes returns the ESI route map for a full character blueprint sync,
// including universe type/group/category resolution and NPC station name lookup.
func charBlueprintRoutes() map[string]string {
	return map[string]string{
		"/latest/characters/90000001/blueprints": "character_blueprints.json",
		"/latest/universe/types/5000":            "universe_type_5000.json",
		"/latest/universe/types/5001":            "universe_type_5001.json",
		"/latest/universe/groups/260":            "universe_group_260.json",
		"/latest/universe/categories/26":         "universe_category_26.json",
		"/latest/universe/names/":                "universe_names_npc_station.json",
	}
}

// TestSyncIntegration_CharacterBlueprints_RowsMatchFixture verifies that after
// a character blueprint sync: two blueprint rows exist, their fields match the
// fixture values, eve_types was populated (type resolution ran), and
// sync_state.cache_until is in the future.
func TestSyncIntegration_CharacterBlueprints_RowsMatchFixture(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	srv := newESIServer(t, charBlueprintRoutes())
	w := newIntegrationWorker(t, sqlDB, srv.URL)
	ctx := context.Background()

	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)

	// Row count.
	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM blueprints WHERE owner_type='character' AND owner_id=90000001`,
	).Scan(&count); err != nil {
		t.Fatalf("querying blueprint count: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 blueprints, got %d", count)
	}

	// Field values — fixture: item 1052548709012 (me=10, te=20), item 1052548712662 (me=5, te=10).
	type want struct{ me, te, loc int64 }
	wantByID := map[int64]want{
		1052548709012: {10, 20, 60003760},
		1052548712662: {5, 10, 60003760},
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT id, me_level, te_level, location_id FROM blueprints
		 WHERE owner_type='character' AND owner_id=90000001`)
	if err != nil {
		t.Fatalf("querying blueprints: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, me, te, loc int64
		if err := rows.Scan(&id, &me, &te, &loc); err != nil {
			t.Fatalf("scanning blueprint row: %v", err)
		}
		exp, ok := wantByID[id]
		if !ok {
			t.Errorf("unexpected blueprint id %d in DB", id)
			continue
		}
		if me != exp.me || te != exp.te || loc != exp.loc {
			t.Errorf("blueprint %d: got me=%d te=%d loc=%d, want me=%d te=%d loc=%d",
				id, me, te, loc, exp.me, exp.te, exp.loc)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating blueprint rows: %v", err)
	}

	// Type resolution — two distinct type_ids in the fixture.
	var typeCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM eve_types`).Scan(&typeCount); err != nil {
		t.Fatalf("querying eve_types count: %v", err)
	}
	if typeCount != 2 {
		t.Errorf("want 2 eve_types rows, got %d", typeCount)
	}

	// sync_state.cache_until must be in the future.
	var cacheUntil time.Time
	if err := sqlDB.QueryRow(
		`SELECT cache_until FROM sync_state
		 WHERE owner_type='character' AND owner_id=90000001 AND endpoint='blueprints'`,
	).Scan(&cacheUntil); err != nil {
		t.Fatalf("querying sync_state: %v", err)
	}
	if !cacheUntil.After(time.Now()) {
		t.Errorf("want cache_until in the future, got %v", cacheUntil)
	}
}

// TestSyncIntegration_CharacterBlueprints_SecondSync_Upserts verifies that
// running blueprint sync a second time with updated fixture data upserts the
// existing row rather than inserting a duplicate.
func TestSyncIntegration_CharacterBlueprints_SecondSync_Upserts(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	ctx := context.Background()

	// First sync: fixture has me_level=10 for item 1052548709012.
	srv1 := newESIServer(t, charBlueprintRoutes())
	w1 := newIntegrationWorker(t, sqlDB, srv1.URL)
	w1.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)

	// Second sync: same item_id, me_level changed to 5.
	// Types and locations are already cached — only the blueprint route is needed.
	updatedJSON := []byte(`[{` +
		`"item_id":1052548709012,` +
		`"location_flag":"Hangar",` +
		`"location_id":60003760,` +
		`"material_efficiency":5,` +
		`"quantity":-1,` +
		`"runs":-1,` +
		`"time_efficiency":20,` +
		`"type_id":5000` +
		`}]`)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Expires", time.Now().Add(10*time.Minute).UTC().Format(http.TimeFormat))
		_, _ = w.Write(updatedJSON)
	}))
	t.Cleanup(srv2.Close)

	w2 := newIntegrationWorker(t, sqlDB, srv2.URL)
	w2.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)

	var meLevel int64
	if err := sqlDB.QueryRow(
		`SELECT me_level FROM blueprints WHERE id=1052548709012`,
	).Scan(&meLevel); err != nil {
		t.Fatalf("querying blueprint after second sync: %v", err)
	}
	if meLevel != 5 {
		t.Errorf("want me_level=5 after upsert, got %d", meLevel)
	}
}

// TestSyncIntegration_CorporationBlueprints_RowsMatchFixture verifies that after
// a corporation blueprint sync followed by a corp assets sync: the blueprint row
// exists, and eve_locations has the exact station name resolved via corp_assets
// (CorpSAG3 location_id → OfficeFolder item → real station name).
func TestSyncIntegration_CorporationBlueprints_RowsMatchFixture(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	seedIntegrationCorporation(t, sqlDB, 99000001, 90000001)
	srv := newESIServer(t, map[string]string{
		"/latest/corporations/99000001/blueprints": "corporation_blueprints.json",
		"/latest/universe/types/5000":              "universe_type_5000.json",
		"/latest/universe/groups/260":              "universe_group_260.json",
		"/latest/universe/categories/26":           "universe_category_26.json",
		"/latest/corporations/99000001/assets/":    "corporation_assets_officefolders.json",
		"/latest/universe/stations/60015146":       "universe_station_60015146.json",
	})
	w := newIntegrationWorker(t, sqlDB, srv.URL)
	ctx := context.Background()

	w.syncSubject(ctx, ownerTypeCorporation, 99000001, endpointCorpAssets)
	w.syncSubject(ctx, ownerTypeCorporation, 99000001, endpointBlueprints)

	// Blueprint row exists.
	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM blueprints WHERE owner_type='corporation' AND owner_id=99000001`,
	).Scan(&count); err != nil {
		t.Fatalf("querying corp blueprint count: %v", err)
	}
	if count != 1 {
		t.Errorf("want 1 corp blueprint, got %d", count)
	}

	// eve_locations has the exact resolved station name via corp_assets lookup.
	const wantName = "Ibura IX - Moon 11 - Spacelane Patrol Testing Facilities"
	var locName string
	if err := sqlDB.QueryRow(
		`SELECT name FROM eve_locations WHERE id=1052718829566`,
	).Scan(&locName); err != nil {
		t.Fatalf("querying eve_locations for corp hangar location ID: %v", err)
	}
	if locName != wantName {
		t.Errorf("eve_locations name: got %q, want %q", locName, wantName)
	}
}

// TestSyncIntegration_CorpAssetsSync_StoresOfficeFolders verifies that after a
// corp_assets sync, the corp_assets table contains the OfficeFolder entry from
// the fixture and sync_state.cache_until is in the future.
func TestSyncIntegration_CorpAssetsSync_StoresOfficeFolders(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	seedIntegrationCorporation(t, sqlDB, 99000001, 90000001)
	srv := newESIServer(t, map[string]string{
		"/latest/corporations/99000001/assets/": "corporation_assets_officefolders.json",
	})
	w := newIntegrationWorker(t, sqlDB, srv.URL)
	ctx := context.Background()

	w.syncSubject(ctx, ownerTypeCorporation, 99000001, endpointCorpAssets)

	// corp_assets row must exist with correct item_id and location_id.
	var itemID, locationID int64
	if err := sqlDB.QueryRow(
		`SELECT item_id, location_id FROM corp_assets WHERE owner_id=99000001`,
	).Scan(&itemID, &locationID); err != nil {
		t.Fatalf("querying corp_assets: %v", err)
	}
	if itemID != 1052718829566 {
		t.Errorf("item_id: got %d, want 1052718829566", itemID)
	}
	if locationID != 60015146 {
		t.Errorf("location_id: got %d, want 60015146", locationID)
	}

	// sync_state.cache_until must be in the future.
	var cacheUntil time.Time
	if err := sqlDB.QueryRow(
		`SELECT cache_until FROM sync_state
		 WHERE owner_type='corporation' AND owner_id=99000001 AND endpoint='corp_assets'`,
	).Scan(&cacheUntil); err != nil {
		t.Fatalf("querying sync_state for corp_assets: %v", err)
	}
	if !cacheUntil.After(time.Now()) {
		t.Errorf("want cache_until in the future, got %v", cacheUntil)
	}
}

// TestSyncIntegration_TypeResolution_FKIntegrity verifies that after a character
// blueprint sync, eve_categories, eve_groups, and eve_types rows all exist and
// foreign-key relationships are intact (a JOIN across all three tables succeeds).
func TestSyncIntegration_TypeResolution_FKIntegrity(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	srv := newESIServer(t, charBlueprintRoutes())
	w := newIntegrationWorker(t, sqlDB, srv.URL)
	ctx := context.Background()

	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)

	// A JOIN across blueprints → eve_types → eve_groups → eve_categories
	// returns the same count as the blueprints table — every FK is satisfied.
	var joinCount int
	if err := sqlDB.QueryRow(`
		SELECT COUNT(*)
		FROM blueprints b
		JOIN eve_types    t ON b.type_id     = t.id
		JOIN eve_groups   g ON t.group_id    = g.id
		JOIN eve_categories c ON g.category_id = c.id
		WHERE b.owner_type='character' AND b.owner_id=90000001
	`).Scan(&joinCount); err != nil {
		t.Fatalf("FK integrity JOIN: %v", err)
	}
	if joinCount != 2 {
		t.Errorf("want 2 rows from FK JOIN (one per blueprint), got %d", joinCount)
	}

	// Also verify expected names from the fixture are present.
	var categoryName, groupName string
	if err := sqlDB.QueryRow(
		`SELECT c.name, g.name
		 FROM eve_categories c
		 JOIN eve_groups g ON g.category_id = c.id
		 WHERE c.id=26 AND g.id=260`,
	).Scan(&categoryName, &groupName); err != nil {
		t.Fatalf("querying category/group names: %v", err)
	}
	if categoryName != "Blueprint" {
		t.Errorf("eve_categories name: got %q, want %q", categoryName, "Blueprint")
	}
	if groupName != "Mineral Blueprint" {
		t.Errorf("eve_groups name: got %q, want %q", groupName, "Mineral Blueprint")
	}
}
