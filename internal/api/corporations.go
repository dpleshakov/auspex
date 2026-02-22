package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

type corporationJSON struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	DelegateID   int64     `json:"delegate_id"`
	DelegateName string    `json:"delegate_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type addCorporationRequest struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	DelegateID int64  `json:"delegate_id"`
}

// Handles:
//
//	GET    /api/corporations
//	POST   /api/corporations
//	DELETE /api/corporations/{id}
func (r *router) handleGetCorporations(w http.ResponseWriter, req *http.Request) {
	corps, err := r.q.ListCorporations(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list corporations")
		return
	}
	resp := make([]corporationJSON, len(corps))
	for i, c := range corps {
		resp[i] = corporationJSON{
			ID:           c.ID,
			Name:         c.Name,
			DelegateID:   c.DelegateID,
			DelegateName: c.DelegateName,
			CreatedAt:    c.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *router) handleAddCorporation(w http.ResponseWriter, req *http.Request) {
	var body addCorporationRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ID == 0 || body.Name == "" || body.DelegateID == 0 {
		writeError(w, http.StatusBadRequest, "id, name, and delegate_id are required")
		return
	}
	ctx := req.Context()
	if _, err := r.q.GetCharacter(ctx, body.DelegateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "delegate_id does not refer to a known character")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to validate delegate")
		return
	}
	if err := r.q.InsertCorporation(ctx, store.InsertCorporationParams{
		ID:         body.ID,
		Name:       body.Name,
		DelegateID: body.DelegateID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to insert corporation")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (r *router) handleDeleteCorporation(w http.ResponseWriter, req *http.Request) {
	id, err := parseID(req, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid corporation id")
		return
	}
	ctx := req.Context()
	if err := r.q.DeleteBlueprintsByOwner(ctx, store.DeleteBlueprintsByOwnerParams{
		OwnerType: "corporation", OwnerID: id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete blueprints")
		return
	}
	if err := r.q.DeleteJobsByOwner(ctx, store.DeleteJobsByOwnerParams{
		OwnerType: "corporation", OwnerID: id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete jobs")
		return
	}
	if err := r.q.DeleteSyncStateByOwner(ctx, store.DeleteSyncStateByOwnerParams{
		OwnerType: "corporation", OwnerID: id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete sync state")
		return
	}
	if err := r.q.DeleteCorporation(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete corporation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
