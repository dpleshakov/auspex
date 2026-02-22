package api

import "net/http"

// Handles:
//
//	GET    /api/corporations
//	POST   /api/corporations
//	DELETE /api/corporations/{id}
func (r *router) handleGetCorporations(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-14): implement.
	w.WriteHeader(http.StatusNotImplemented)
}

func (r *router) handleAddCorporation(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-14): implement.
	w.WriteHeader(http.StatusNotImplemented)
}

func (r *router) handleDeleteCorporation(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-14): implement.
	w.WriteHeader(http.StatusNotImplemented)
}
