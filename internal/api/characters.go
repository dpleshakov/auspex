package api

import (
	"net/http"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

type characterJSON struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Handles:
//
//	GET    /api/characters
//	DELETE /api/characters/{id}
func (r *router) handleGetCharacters(w http.ResponseWriter, req *http.Request) {
	chars, err := r.q.ListCharacters(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list characters")
		return
	}
	resp := make([]characterJSON, len(chars))
	for i, c := range chars {
		resp[i] = characterJSON{ID: c.ID, Name: c.Name, CreatedAt: c.CreatedAt}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *router) handleDeleteCharacter(w http.ResponseWriter, req *http.Request) {
	id, err := parseID(req, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return
	}
	ctx := req.Context()
	if err := r.q.DeleteBlueprintsByOwner(ctx, store.DeleteBlueprintsByOwnerParams{
		OwnerType: "character", OwnerID: id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete blueprints")
		return
	}
	if err := r.q.DeleteJobsByOwner(ctx, store.DeleteJobsByOwnerParams{
		OwnerType: "character", OwnerID: id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete jobs")
		return
	}
	if err := r.q.DeleteSyncStateByOwner(ctx, store.DeleteSyncStateByOwnerParams{
		OwnerType: "character", OwnerID: id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete sync state")
		return
	}
	if err := r.q.DeleteCharacter(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete character")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
