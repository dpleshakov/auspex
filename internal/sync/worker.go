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
// TASK-10 implements the full body; this stub exists so the package compiles in TASK-09.
func (w *Worker) syncSubject(_ context.Context, _ string, _ int64, _ string) {
	// Implemented in TASK-10.
}
