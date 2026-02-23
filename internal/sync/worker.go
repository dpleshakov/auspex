// Package sync is the background sync worker and scheduler.
// Responsibilities: know when and what to update; coordinate auth/esi and store.
//
// The worker runs as a goroutine on startup. A ticker fires every N minutes (from config).
// On each tick it iterates all subjects (characters + corporations), checks
// sync_state.cache_until, skips if still fresh, otherwise fetches from ESI and upserts to DB.
//
// Accepts a force-refresh signal via a channel from the api package.
// After a successful sync, triggers lazy resolution of any new type_ids via esi.
//
// Note: this package is named "sync" matching the architecture. If stdlib sync is needed
// inside this package, import it as: import stdsync "sync"
package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

const (
	endpointBlueprints   = "blueprints"
	endpointJobs         = "jobs"
	ownerTypeCharacter   = "character"
	ownerTypeCorporation = "corporation"
)

// Worker is the background sync worker.
// It runs a ticker loop, checks ESI cache freshness per subject+endpoint,
// and calls syncFn for subjects whose cache has expired.
type Worker struct {
	store           store.Querier
	esi             esi.Client
	refreshInterval time.Duration
	now             func() time.Time // injectable for testing; defaults to time.Now
	force           chan struct{}    // signals an immediate full sync, ignoring cache_until

	// syncFn is called when a subject needs syncing.
	// Defaults to w.syncSubject (a no-op placeholder until TASK-10).
	// Replace in tests to observe sync calls without executing real ESI fetches.
	syncFn func(ctx context.Context, ownerType string, ownerID int64, endpoint string)
}

// New creates a Worker. interval is the ticker period (typically from config.RefreshInterval).
func New(q store.Querier, esiClient esi.Client, interval time.Duration) *Worker {
	w := &Worker{
		store:           q,
		esi:             esiClient,
		refreshInterval: interval,
		now:             time.Now,
		force:           make(chan struct{}, 1),
	}
	w.syncFn = w.syncSubject
	return w
}

// Run starts the background sync loop and blocks until ctx is cancelled.
// Intended to be called in a goroutine:
//
//	go worker.Run(ctx)
//
// An initial cycle runs immediately on startup so data is available before
// the first ticker fires.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.refreshInterval)
	defer ticker.Stop()

	// Run once immediately so the dashboard has data right away.
	w.runCycle(ctx, false)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runCycle(ctx, false)
		case <-w.force:
			w.runCycle(ctx, true)
		}
	}
}

// ForceRefresh signals the worker to run a full sync cycle immediately,
// bypassing cache_until for all subjects. Safe to call from any goroutine.
// If a force-refresh is already pending, the duplicate signal is discarded.
func (w *Worker) ForceRefresh() {
	select {
	case w.force <- struct{}{}:
	default: // already queued; discard duplicate
	}
}

// runCycle iterates all characters and corporations.
// For each subject+endpoint pair it checks freshness (unless force is true)
// and calls w.syncFn for subjects that need syncing.
func (w *Worker) runCycle(ctx context.Context, force bool) {
	chars, err := w.store.ListCharacters(ctx)
	if err != nil {
		log.Printf("sync: listing characters: %v", err)
		return
	}

	corps, err := w.store.ListCorporations(ctx)
	if err != nil {
		log.Printf("sync: listing corporations: %v", err)
		return
	}

	for _, char := range chars {
		for _, endpoint := range []string{endpointBlueprints, endpointJobs} {
			if ctx.Err() != nil {
				return
			}
			if !force && w.isFresh(ctx, ownerTypeCharacter, char.ID, endpoint) {
				continue
			}
			w.syncFn(ctx, ownerTypeCharacter, char.ID, endpoint)
		}
	}

	for _, corp := range corps {
		for _, endpoint := range []string{endpointBlueprints, endpointJobs} {
			if ctx.Err() != nil {
				return
			}
			if !force && w.isFresh(ctx, ownerTypeCorporation, corp.ID, endpoint) {
				continue
			}
			w.syncFn(ctx, ownerTypeCorporation, corp.ID, endpoint)
		}
	}
}

// isFresh returns true when the ESI cache for (ownerType, ownerID, endpoint)
// is still valid (cache_until is in the future). Returns false if no sync_state
// record exists (never synced) or if cache_until has passed.
func (w *Worker) isFresh(ctx context.Context, ownerType string, ownerID int64, endpoint string) bool {
	state, err := w.store.GetSyncState(ctx, store.GetSyncStateParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
		Endpoint:  endpoint,
	})
	if err != nil {
		// No record means never synced → treat as expired.
		return false
	}
	return state.CacheUntil.After(w.now())
}

// syncSubject fetches and stores ESI data for one (ownerType, ownerID, endpoint) tuple.
// On ESI or store error the error is logged and sync_state is NOT updated,
// so the next tick will retry the subject.
func (w *Worker) syncSubject(ctx context.Context, ownerType string, ownerID int64, endpoint string) {
	var cacheUntil time.Time
	var err error

	switch endpoint {
	case endpointBlueprints:
		cacheUntil, err = w.syncBlueprints(ctx, ownerType, ownerID)
		if err == nil {
			w.resolveTypeIDs(ctx, ownerType, ownerID)
		}
	case endpointJobs:
		cacheUntil, err = w.syncJobs(ctx, ownerType, ownerID)
	default:
		log.Printf("sync: unknown endpoint %q for %s %d", endpoint, ownerType, ownerID)
		return
	}

	if err != nil {
		log.Printf("sync: %s %s %d: %v", endpoint, ownerType, ownerID, err)
		return
	}

	if err := w.store.UpsertSyncState(ctx, store.UpsertSyncStateParams{
		OwnerType:  ownerType,
		OwnerID:    ownerID,
		Endpoint:   endpoint,
		LastSync:   w.now(),
		CacheUntil: cacheUntil,
	}); err != nil {
		log.Printf("sync: updating sync_state for %s %d %s: %v", ownerType, ownerID, endpoint, err)
	}
}

// syncBlueprints fetches blueprints from ESI and upserts them into the store.
// Returns the ESI cache expiry time on success.
func (w *Worker) syncBlueprints(ctx context.Context, ownerType string, ownerID int64) (time.Time, error) {
	var bps []esi.Blueprint
	var cacheUntil time.Time
	var err error

	switch ownerType {
	case ownerTypeCharacter:
		bps, cacheUntil, err = w.esi.GetCharacterBlueprints(ctx, ownerID, "")
	case ownerTypeCorporation:
		bps, cacheUntil, err = w.esi.GetCorporationBlueprints(ctx, ownerID, "")
	default:
		return time.Time{}, fmt.Errorf("unknown owner type %q", ownerType)
	}
	if err != nil {
		return cacheUntil, fmt.Errorf("fetching blueprints: %w", err)
	}

	// Resolve type_ids from the ESI response before upserting blueprints.
	// blueprints.type_id has a FK to eve_types.id, so eve_types rows must
	// exist before the INSERT or the statement fails with a constraint error.
	seen := make(map[int64]bool, len(bps))
	typeIDs := make([]int64, 0, len(bps))
	for _, bp := range bps {
		if !seen[bp.TypeID] {
			seen[bp.TypeID] = true
			typeIDs = append(typeIDs, bp.TypeID)
		}
	}
	w.resolveTypeIDsList(ctx, typeIDs)

	now := w.now()
	for _, bp := range bps {
		if err := w.store.UpsertBlueprint(ctx, store.UpsertBlueprintParams{
			ID:         bp.ItemID,
			OwnerType:  ownerType,
			OwnerID:    ownerID,
			TypeID:     bp.TypeID,
			LocationID: bp.LocationID,
			MeLevel:    bp.MELevel,
			TeLevel:    bp.TELevel,
			UpdatedAt:  now,
		}); err != nil {
			return cacheUntil, fmt.Errorf("upserting blueprint %d: %w", bp.ItemID, err)
		}
	}

	return cacheUntil, nil
}

// resolveTypeIDsList ensures that every type_id in the provided slice has a
// corresponding row in eve_types, eve_groups, and eve_categories.
// Unknown type_ids are fetched from ESI and inserted in FK order:
// category → group → type. Known type_ids are skipped.
// Errors per type_id are logged and skipped; they are non-fatal so that a
// single bad type_id does not block resolution of the rest.
func (w *Worker) resolveTypeIDsList(ctx context.Context, typeIDs []int64) {
	for _, typeID := range typeIDs {
		if ctx.Err() != nil {
			return
		}

		// Skip if already in eve_types — no ESI call needed.
		if _, err := w.store.GetEveType(ctx, typeID); err == nil {
			continue
		}

		ut, err := w.esi.GetUniverseType(ctx, typeID)
		if err != nil {
			log.Printf("sync: fetching universe type %d: %v", typeID, err)
			continue
		}

		// Insert in FK order: category → group → type.
		if err := w.store.InsertEveCategory(ctx, store.InsertEveCategoryParams{
			ID:   ut.CategoryID,
			Name: ut.CategoryName,
		}); err != nil {
			log.Printf("sync: inserting eve_category %d: %v", ut.CategoryID, err)
			continue
		}
		if err := w.store.InsertEveGroup(ctx, store.InsertEveGroupParams{
			ID:         ut.GroupID,
			CategoryID: ut.CategoryID,
			Name:       ut.GroupName,
		}); err != nil {
			log.Printf("sync: inserting eve_group %d: %v", ut.GroupID, err)
			continue
		}
		if err := w.store.InsertEveType(ctx, store.InsertEveTypeParams{
			ID:      typeID,
			GroupID: ut.GroupID,
			Name:    ut.TypeName,
		}); err != nil {
			log.Printf("sync: inserting eve_type %d: %v", typeID, err)
		}
	}
}

// resolveTypeIDs loads type_ids already stored for the given owner and calls
// resolveTypeIDsList to fill any gaps in eve_types.
// Called after a successful blueprint sync as a safety net for any type_ids
// that slipped through (e.g. interrupted resolution on a previous tick).
func (w *Worker) resolveTypeIDs(ctx context.Context, ownerType string, ownerID int64) {
	typeIDs, err := w.store.ListBlueprintTypeIDsByOwner(ctx, store.ListBlueprintTypeIDsByOwnerParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
	})
	if err != nil {
		log.Printf("sync: listing blueprint type_ids for %s %d: %v", ownerType, ownerID, err)
		return
	}
	w.resolveTypeIDsList(ctx, typeIDs)
}

// syncJobs fetches active/ready jobs from ESI, upserts them into the store,
// and deletes any jobs that were previously stored but are no longer in the ESI response.
// Returns the ESI cache expiry time on success.
func (w *Worker) syncJobs(ctx context.Context, ownerType string, ownerID int64) (time.Time, error) {
	var jobs []esi.Job
	var cacheUntil time.Time
	var err error

	switch ownerType {
	case ownerTypeCharacter:
		jobs, cacheUntil, err = w.esi.GetCharacterJobs(ctx, ownerID, "")
	case ownerTypeCorporation:
		jobs, cacheUntil, err = w.esi.GetCorporationJobs(ctx, ownerID, "")
	default:
		return time.Time{}, fmt.Errorf("unknown owner type %q", ownerType)
	}
	if err != nil {
		return cacheUntil, fmt.Errorf("fetching jobs: %w", err)
	}

	// Build set of job IDs returned by ESI.
	incoming := make(map[int64]bool, len(jobs))
	for _, j := range jobs {
		incoming[j.JobID] = true
	}

	// Get job IDs currently in the store for this owner so we can prune stale ones.
	existing, err := w.store.ListJobIDsByOwner(ctx, store.ListJobIDsByOwnerParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
	})
	if err != nil {
		return cacheUntil, fmt.Errorf("listing existing jobs: %w", err)
	}

	// Upsert all incoming jobs.
	// A job whose blueprint_id is not in the blueprints table (e.g. a
	// manufacturing job using a BPC we do not track) will fail the FK
	// constraint. Log and skip rather than aborting the whole sync so that
	// research jobs on tracked BPOs are still stored.
	now := w.now()
	for _, j := range jobs {
		if err := w.store.UpsertJob(ctx, store.UpsertJobParams{
			ID:          j.JobID,
			BlueprintID: j.BlueprintID,
			OwnerType:   ownerType,
			OwnerID:     ownerID,
			InstallerID: j.InstallerID,
			Activity:    j.Activity,
			Status:      j.Status,
			StartDate:   j.StartDate,
			EndDate:     j.EndDate,
			UpdatedAt:   now,
		}); err != nil {
			log.Printf("sync: jobs %s %d: upserting job %d: %v", ownerType, ownerID, j.JobID, err)
			continue
		}
	}

	// Delete stale jobs (in store but no longer in ESI response).
	for _, id := range existing {
		if !incoming[id] {
			if err := w.store.DeleteJobByID(ctx, id); err != nil {
				return cacheUntil, fmt.Errorf("deleting stale job %d: %w", id, err)
			}
		}
	}

	return cacheUntil, nil
}
