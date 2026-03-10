package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// charJobAllRoutes returns the ESI route map for a character blueprint sync followed
// by a character job sync. Both sets of routes live in the same map so a single
// test server can serve both syncs.
func charJobAllRoutes() map[string]string {
	return map[string]string{
		"/latest/characters/90000001/blueprints":    "character_blueprints.json",
		"/latest/universe/types/5000":               "universe_type_5000.json",
		"/latest/universe/types/5001":               "universe_type_5001.json",
		"/latest/universe/groups/260":               "universe_group_260.json",
		"/latest/universe/categories/26":            "universe_category_26.json",
		"/latest/universe/names/":                   "universe_names_npc_station.json",
		"/latest/characters/90000001/industry/jobs": "character_jobs.json",
	}
}

// TestSyncIntegration_CharacterJobs_OnlyActiveAndReadyStored verifies that after
// a character job sync, only active and ready jobs are stored in the DB.
// The delivered job in the fixture is filtered by the ESI client and never inserted.
func TestSyncIntegration_CharacterJobs_OnlyActiveAndReadyStored(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	srv := newESIServer(t, charJobAllRoutes())
	w := newIntegrationWorker(t, sqlDB, srv.URL)
	ctx := context.Background()

	// Blueprints must exist before jobs (FK constraint).
	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)
	// Fixture: 1 active + 1 ready + 1 delivered job.
	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointJobs)

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE owner_type='character' AND owner_id=90000001`,
	).Scan(&count); err != nil {
		t.Fatalf("querying job count: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 jobs (active + ready only), got %d", count)
	}
}

// TestSyncIntegration_CharacterJobs_SyncStateUpdated verifies that after a character
// job sync, sync_state.cache_until for endpoint='jobs' is in the future.
func TestSyncIntegration_CharacterJobs_SyncStateUpdated(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	srv := newESIServer(t, charJobAllRoutes())
	w := newIntegrationWorker(t, sqlDB, srv.URL)
	ctx := context.Background()

	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)
	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointJobs)

	var cacheUntil time.Time
	if err := sqlDB.QueryRow(
		`SELECT cache_until FROM sync_state
		 WHERE owner_type='character' AND owner_id=90000001 AND endpoint='jobs'`,
	).Scan(&cacheUntil); err != nil {
		t.Fatalf("querying sync_state for jobs: %v", err)
	}
	if !cacheUntil.After(time.Now()) {
		t.Errorf("want cache_until in the future, got %v", cacheUntil)
	}
}

// TestSyncIntegration_StaleJobsDeleted verifies that after a job sync, jobs present
// in the DB but absent from the ESI response are deleted.
func TestSyncIntegration_StaleJobsDeleted(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	ctx := context.Background()

	// Seed blueprint rows (FK dependency) via a preceding blueprint sync.
	bpSrv := newESIServer(t, charBlueprintRoutes())
	w := newIntegrationWorker(t, sqlDB, bpSrv.URL)
	w.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointBlueprints)

	// Insert two job rows directly into the DB.
	now := time.Now().UTC()
	for _, job := range []struct {
		id, blueprintID  int64
		activity, status string
	}{
		{700000001, 1052548709012, "me_research", "active"},
		{700000002, 1052548712662, "te_research", "ready"},
	} {
		_, err := sqlDB.ExecContext(ctx,
			`INSERT INTO jobs
			 (id, blueprint_id, owner_type, owner_id, installer_id, activity, status, start_date, end_date, updated_at)
			 VALUES (?, ?, 'character', 90000001, 90000001, ?, ?, ?, ?, ?)`,
			job.id, job.blueprintID, job.activity, job.status,
			now.Add(-24*time.Hour), now.Add(7*24*time.Hour), now,
		)
		if err != nil {
			t.Fatalf("seeding job %d: %v", job.id, err)
		}
	}

	// ESI returns only job 700000001; job 700000002 is stale and must be deleted.
	onlyJob1 := []byte(`[{` +
		`"activity_id":4,` +
		`"blueprint_id":1052548709012,` +
		`"blueprint_location_id":60003760,` +
		`"blueprint_type_id":5000,` +
		`"cost":123000,` +
		`"duration":864000,` +
		`"end_date":"2026-03-20T00:00:00Z",` +
		`"facility_id":60003760,` +
		`"installer_id":90000001,` +
		`"job_id":700000001,` +
		`"licensed_runs":1,` +
		`"location_id":60003760,` +
		`"output_location_id":60003760,` +
		`"probability":1.0,` +
		`"product_type_id":5000,` +
		`"runs":1,` +
		`"start_date":"2026-03-01T00:00:00Z",` +
		`"status":"active"` +
		`}]`)
	staleSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Expires", time.Now().Add(10*time.Minute).UTC().Format(http.TimeFormat))
		_, _ = w.Write(onlyJob1)
	}))
	t.Cleanup(staleSrv.Close)

	w2 := newIntegrationWorker(t, sqlDB, staleSrv.URL)
	w2.syncSubject(ctx, ownerTypeCharacter, 90000001, endpointJobs)

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE owner_type='character'`,
	).Scan(&count); err != nil {
		t.Fatalf("querying job count after sync: %v", err)
	}
	if count != 1 {
		t.Errorf("want 1 job after stale deletion, got %d", count)
	}

	var staleCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM jobs WHERE id=700000002`).Scan(&staleCount); err != nil {
		t.Fatalf("checking for stale job: %v", err)
	}
	if staleCount != 0 {
		t.Error("job 700000002 should have been deleted as stale, but still exists")
	}
}

// TestSyncIntegration_CorporationJobs_Stored verifies that after a corporation job
// sync, the job row exists with owner_type='corporation' and correct field values.
func TestSyncIntegration_CorporationJobs_Stored(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	seedIntegrationCorporation(t, sqlDB, 99000001, 90000001)
	ctx := context.Background()

	// One server handles both the blueprint sync (FK seed) and the job sync.
	srv := newESIServer(t, map[string]string{
		"/latest/corporations/99000001/blueprints":    "corporation_blueprints.json",
		"/latest/universe/types/5000":                 "universe_type_5000.json",
		"/latest/universe/groups/260":                 "universe_group_260.json",
		"/latest/universe/categories/26":              "universe_category_26.json",
		"/latest/corporations/99000001/offices/":      "corporation_offices.json",
		"/latest/universe/names/":                     "universe_names_corp_station.json",
		"/latest/corporations/99000001/industry/jobs": "corporation_jobs.json",
	})
	w := newIntegrationWorker(t, sqlDB, srv.URL)

	// Seed corp blueprint row (FK dependency for corp jobs).
	w.syncSubject(ctx, ownerTypeCorporation, 99000001, endpointBlueprints)
	// Sync corp jobs.
	w.syncSubject(ctx, ownerTypeCorporation, 99000001, endpointJobs)

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE owner_type='corporation' AND owner_id=99000001`,
	).Scan(&count); err != nil {
		t.Fatalf("querying corp job count: %v", err)
	}
	if count != 1 {
		t.Errorf("want 1 corp job, got %d", count)
	}

	// Verify field values from the fixture (job_id=700000010, activity_id=4 → me_research, active).
	var jobID int64
	var activity, status string
	if err := sqlDB.QueryRow(
		`SELECT id, activity, status FROM jobs WHERE owner_type='corporation' AND owner_id=99000001`,
	).Scan(&jobID, &activity, &status); err != nil {
		t.Fatalf("querying corp job fields: %v", err)
	}
	if jobID != 700000010 {
		t.Errorf("corp job id: got %d, want 700000010", jobID)
	}
	if activity != "me_research" {
		t.Errorf("corp job activity: got %q, want %q", activity, "me_research")
	}
	if status != "active" {
		t.Errorf("corp job status: got %q, want %q", status, "active")
	}
}
