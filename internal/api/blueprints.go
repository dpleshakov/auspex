package api

import "net/http"

// Handles:
//
//	GET /api/blueprints       (query params: status, owner_id, owner_type, category_id)
//	GET /api/jobs/summary
func (r *router) handleGetBlueprints(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-15): implement.
	w.WriteHeader(http.StatusNotImplemented)
}

func (r *router) handleGetJobsSummary(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-15): implement.
	w.WriteHeader(http.StatusNotImplemented)
}
