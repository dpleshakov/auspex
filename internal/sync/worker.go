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
