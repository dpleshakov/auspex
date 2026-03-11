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
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

const (
	endpointCorpAssets   = "corp_assets"
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

// Run starts the background sync loop and blocks until ctx is canceled.
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
		for _, endpoint := range []string{endpointCorpAssets, endpointBlueprints, endpointJobs} {
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
	case endpointCorpAssets:
		if ownerType != ownerTypeCorporation {
			log.Printf("sync: corp_assets endpoint requires corporation owner, got %s %d", ownerType, ownerID)
			return
		}
		cacheUntil, err = w.syncCorpAssets(ctx, ownerID)
	case endpointBlueprints:
		cacheUntil, err = w.syncBlueprints(ctx, ownerType, ownerID)
		if err == nil {
			w.resolveTypeIDs(ctx, ownerType, ownerID)
			w.resolveLocationIDs(ctx, ownerType, ownerID)
		}
	case endpointJobs:
		cacheUntil, err = w.syncJobs(ctx, ownerType, ownerID)
	default:
		log.Printf("sync: unknown endpoint %q for %s %d", endpoint, ownerType, ownerID)
		return
	}

	if err != nil {
		log.Printf("sync: %s %s %d: %v", endpoint, ownerType, ownerID, err)
		if uerr := w.store.UpdateSyncStateError(ctx, store.UpdateSyncStateErrorParams{
			LastError: sql.NullString{String: err.Error(), Valid: true},
			OwnerType: ownerType,
			OwnerID:   ownerID,
			Endpoint:  endpoint,
		}); uerr != nil {
			log.Printf("sync: recording error for %s %d %s: %v", ownerType, ownerID, endpoint, uerr)
		}
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
	if err := w.store.UpdateSyncStateError(ctx, store.UpdateSyncStateErrorParams{
		LastError: sql.NullString{},
		OwnerType: ownerType,
		OwnerID:   ownerID,
		Endpoint:  endpoint,
	}); err != nil {
		log.Printf("sync: clearing error for %s %d %s: %v", ownerType, ownerID, endpoint, err)
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
			ID:           bp.ItemID,
			OwnerType:    ownerType,
			OwnerID:      ownerID,
			TypeID:       bp.TypeID,
			LocationID:   bp.LocationID,
			LocationFlag: bp.LocationFlag,
			MeLevel:      bp.MELevel,
			TeLevel:      bp.TELevel,
			UpdatedAt:    now,
		}); err != nil {
			// A blueprint whose type_id is missing from eve_types (e.g. because
			// type resolution failed due to a transient ESI error) fails the FK
			// constraint. Log and skip so the rest of the blueprints are stored
			// and sync_state is still updated. The next tick will retry resolution.
			log.Printf("sync: blueprints %s %d: upserting blueprint %d: %v", ownerType, ownerID, bp.ItemID, err)
			continue
		}
	}

	return cacheUntil, nil
}

// syncCorpAssets fetches all pages of corporation assets from ESI, retains only
// OfficeFolder entries, and stores them in corp_assets after pruning stale rows.
// Returns the ESI cache expiry from page 1.
func (w *Worker) syncCorpAssets(ctx context.Context, corpID int64) (time.Time, error) {
	assets, totalPages, cacheUntil, err := w.esi.GetCorporationAssets(ctx, corpID, "", 1)
	if err != nil {
		return time.Time{}, fmt.Errorf("fetching corp assets page 1: %w", err)
	}

	var officeFolders []esi.CorpAsset
	for _, a := range assets {
		if a.LocationFlag == "OfficeFolder" {
			officeFolders = append(officeFolders, a)
		}
	}

	for page := 2; page <= totalPages; page++ {
		if ctx.Err() != nil {
			return cacheUntil, ctx.Err()
		}
		pageAssets, _, _, err := w.esi.GetCorporationAssets(ctx, corpID, "", page)
		if err != nil {
			return cacheUntil, fmt.Errorf("fetching corp assets page %d: %w", page, err)
		}
		for _, a := range pageAssets {
			if a.LocationFlag == "OfficeFolder" {
				officeFolders = append(officeFolders, a)
			}
		}
	}

	if err := w.store.DeleteCorpAssetsByOwner(ctx, corpID); err != nil {
		return cacheUntil, fmt.Errorf("deleting stale corp assets: %w", err)
	}
	for _, a := range officeFolders {
		if err := w.store.UpsertCorpAsset(ctx, store.UpsertCorpAssetParams{
			ItemID:       a.ItemID,
			OwnerID:      corpID,
			LocationID:   a.LocationID,
			LocationType: a.LocationType,
		}); err != nil {
			log.Printf("sync: corp %d: upserting corp asset %d: %v", corpID, a.ItemID, err)
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

const (
	// npcStationMin and npcStationMax define the inclusive-exclusive range of
	// resolvable NPC station IDs recognized by POST /universe/names/.
	// EVE NPC station IDs are in [60_000_000, 64_000_000).
	npcStationMin = int64(60_000_000)
	npcStationMax = int64(64_000_000)

	// corpHangarSentinel is the display name stored for corporation office/hangar
	// item IDs (range [64_000_000, 1_000_000_000_000)) that cannot be resolved
	// via /universe/names/ and have no dedicated ESI lookup.
	corpHangarSentinel = "Corporation Hangar"
)

// corpHangarFlags are location_flag values that indicate a blueprint is stored
// in a corporation office slot within an NPC station. The location_id in these
// cases is the office item ID, which is not resolvable via any universe endpoint.
var corpHangarFlags = map[string]bool{
	"CorpSAG1": true, "CorpSAG2": true, "CorpSAG3": true,
	"CorpSAG4": true, "CorpSAG5": true, "CorpSAG6": true, "CorpSAG7": true,
	"CorpDeliveries": true,
}

// resolveLocationIDs resolves location_ids for all blueprints owned by ownerType/ownerID
// and populates eve_locations with human-readable names.
//
// For corp blueprint flags (CorpSAG*, CorpDeliveries) the location_id is an office item ID;
// the real station/structure is looked up in corp_assets. If the asset is not yet synced,
// the sentinel "Corporation Hangar" is stored and will be replaced on the next cycle.
//
// For direct location_id entries ("Hangar" flag, character blueprints):
// NPC stations (60 000 000–64 000 000) are resolved via GetStation;
// player structures (all other IDs) via GetUniverseStructure.
// Already-cached IDs are skipped. 403 responses for structures are not cached.
func (w *Worker) resolveLocationIDs(ctx context.Context, ownerType string, ownerID int64) {
	rows, err := w.store.ListBlueprintLocationsByOwner(ctx, store.ListBlueprintLocationsByOwnerParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
	})
	if err != nil {
		log.Printf("sync: listing blueprint locations for %s %d: %v", ownerType, ownerID, err)
		return
	}

	// Lazily fetch a character token only when needed for structure lookups.
	var token string
	getToken := func() string {
		if token == "" {
			token = w.anyCharacterToken(ctx)
		}
		return token
	}

	now := w.now()
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		if corpHangarFlags[row.LocationFlag] {
			w.resolveCorpHangarLocation(ctx, row.LocationID, now, getToken)
			continue
		}
		// Direct location_id ("Hangar" or other flag with a real station/structure ID).
		if _, err := w.store.GetLocation(ctx, row.LocationID); err == nil {
			continue // already cached
		}
		w.resolveDirectLocation(ctx, row.LocationID, now, getToken)
	}
}

// resolveCorpHangarLocation resolves a corp blueprint office item ID to a human-readable name.
// It looks up the real station/structure ID from corp_assets, then fetches the name.
// If the asset is not found (not yet synced), stores corpHangarSentinel.
func (w *Worker) resolveCorpHangarLocation(ctx context.Context, itemID int64, now time.Time, getToken func() string) {
	if _, err := w.store.GetLocation(ctx, itemID); err == nil {
		return // already cached
	}

	asset, err := w.store.GetCorpAsset(ctx, itemID)
	if err != nil {
		// Corp assets not yet synced for this office item — skip and retry next cycle.
		// Do not store a sentinel: it would be cached in eve_locations and block future resolution.
		return
	}

	realID := asset.LocationID
	var name string
	switch {
	case realID >= npcStationMin && realID < npcStationMax:
		n, err := w.esi.GetStation(ctx, realID)
		if err != nil {
			log.Printf("sync: resolving NPC station %d for corp hangar %d: %v", realID, itemID, err)
			return
		}
		name = n
	default: // player structure
		tok := getToken()
		if tok == "" {
			log.Printf("sync: no character token for structure %d (corp hangar %d)", realID, itemID)
			return
		}
		structure, err := w.esi.GetUniverseStructure(ctx, realID, tok)
		if errors.Is(err, esi.ErrForbidden) {
			log.Printf("sync: structure %d: access denied, skipping cache", realID)
			return
		}
		if errors.Is(err, esi.ErrNotFound) {
			log.Printf("sync: structure %d: not found in ESI, storing sentinel", realID)
			_ = w.store.InsertLocation(ctx, store.InsertLocationParams{
				ID:         itemID,
				Name:       corpHangarSentinel,
				ResolvedAt: now,
			})
			return
		}
		if err != nil {
			log.Printf("sync: fetching structure %d for corp hangar %d: %v", realID, itemID, err)
			return
		}
		sysName, err := w.getSystemName(ctx, structure.SolarSystemID)
		if err != nil {
			log.Printf("sync: fetching system %d for structure %d: %v", structure.SolarSystemID, realID, err)
			return
		}
		name = sysName + " \u2014 " + structure.Name
	}

	if err := w.store.InsertLocation(ctx, store.InsertLocationParams{
		ID:         itemID,
		Name:       name,
		ResolvedAt: now,
	}); err != nil {
		log.Printf("sync: inserting corp hangar location %d: %v", itemID, err)
	}
}

// resolveDirectLocation resolves a direct station or structure ID to a human-readable name.
// NPC stations (60 000 000–64 000 000) are fetched via GetStation.
// All other IDs are treated as player structures and fetched via GetUniverseStructure.
func (w *Worker) resolveDirectLocation(ctx context.Context, id int64, now time.Time, getToken func() string) {
	switch {
	case id >= npcStationMin && id < npcStationMax:
		name, err := w.esi.GetStation(ctx, id)
		if err != nil {
			log.Printf("sync: resolving NPC station %d: %v", id, err)
			return
		}
		if err := w.store.InsertLocation(ctx, store.InsertLocationParams{
			ID:         id,
			Name:       name,
			ResolvedAt: now,
		}); err != nil {
			log.Printf("sync: inserting location %d: %v", id, err)
		}
	default: // player structure
		tok := getToken()
		if tok == "" {
			log.Printf("sync: no character token available for structure resolution")
			return
		}
		structure, err := w.esi.GetUniverseStructure(ctx, id, tok)
		if errors.Is(err, esi.ErrForbidden) {
			log.Printf("sync: structure %d: access denied, skipping cache", id)
			return
		}
		if errors.Is(err, esi.ErrNotFound) {
			log.Printf("sync: structure %d: not found in ESI, storing sentinel", id)
			_ = w.store.InsertLocation(ctx, store.InsertLocationParams{
				ID:         id,
				Name:       corpHangarSentinel,
				ResolvedAt: now,
			})
			return
		}
		if err != nil {
			log.Printf("sync: fetching structure %d: %v", id, err)
			return
		}
		sysName, err := w.getSystemName(ctx, structure.SolarSystemID)
		if err != nil {
			log.Printf("sync: fetching system %d for structure %d: %v", structure.SolarSystemID, id, err)
			return
		}
		name := sysName + " \u2014 " + structure.Name
		if err := w.store.InsertLocation(ctx, store.InsertLocationParams{
			ID:         id,
			Name:       name,
			ResolvedAt: now,
		}); err != nil {
			log.Printf("sync: inserting structure location %d: %v", id, err)
		}
	}
}

// getSystemName returns the name of a solar system, using eve_locations as a cache.
// If the system name is not cached, it is fetched from ESI and stored.
func (w *Worker) getSystemName(ctx context.Context, systemID int64) (string, error) {
	if loc, err := w.store.GetLocation(ctx, systemID); err == nil {
		return loc.Name, nil
	}

	name, err := w.esi.GetUniverseSystem(ctx, systemID)
	if err != nil {
		return "", err
	}

	if err := w.store.InsertLocation(ctx, store.InsertLocationParams{
		ID:         systemID,
		Name:       name,
		ResolvedAt: w.now(),
	}); err != nil {
		log.Printf("sync: caching system name %d: %v", systemID, err)
	}
	return name, nil
}

// anyCharacterToken returns the access token of any available character,
// or empty string if no characters are registered.
func (w *Worker) anyCharacterToken(ctx context.Context) string {
	chars, err := w.store.ListCharacters(ctx)
	if err != nil || len(chars) == 0 {
		return ""
	}
	return chars[0].AccessToken
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
