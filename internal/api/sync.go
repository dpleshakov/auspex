package api

import "net/http"

// Handles:
//
//	POST /api/sync         (sends force-refresh signal to the sync worker via channel)
//	GET  /api/sync/status
func (r *router) handlePostSync(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-16): implement.
	w.WriteHeader(http.StatusNotImplemented)
}

func (r *router) handleGetSyncStatus(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-16): implement.
	w.WriteHeader(http.StatusNotImplemented)
}
