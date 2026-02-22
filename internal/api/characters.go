package api

import "net/http"

// Handles:
//
//	GET    /api/characters
//	DELETE /api/characters/{id}
func (r *router) handleGetCharacters(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-14): implement.
	w.WriteHeader(http.StatusNotImplemented)
}

func (r *router) handleDeleteCharacter(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-14): implement.
	w.WriteHeader(http.StatusNotImplemented)
}
