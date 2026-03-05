package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

type characterJSON struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	CorporationID   int64     `json:"corporation_id"`
	CorporationName string    `json:"corporation_name"`
	IsDelegate      bool      `json:"is_delegate"`
	SyncError       *string   `json:"sync_error"`
	CreatedAt       time.Time `json:"created_at"`
}

// Handles:
//
//	GET    /api/characters
//	DELETE /api/characters/{id}
func (r *router) handleGetCharacters(w http.ResponseWriter, req *http.Request) {
	chars, err := r.q.ListCharactersWithMeta(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list characters")
		return
	}
	resp := make([]characterJSON, len(chars))
	for i, c := range chars {
		var syncErr *string
		if s, ok := c.SyncError.(string); ok && s != "" {
			syncErr = &s
		}
		resp[i] = characterJSON{
			ID:              c.ID,
			Name:            c.Name,
			CorporationID:   c.CorporationID,
			CorporationName: c.CorporationName,
			IsDelegate:      c.IsDelegate != 0,
			SyncError:       syncErr,
			CreatedAt:       c.CreatedAt,
		}
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

	char, err := r.q.GetCharacter(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get character")
		return
	}

	corpID := char.CorporationID
	// Apply corporation logic only for player corporations (not unset, not NPC range 1000000–2000000).
	if corpID > 0 && (corpID < 1_000_000 || corpID > 2_000_000) {
		corpChars, err := r.q.ListCharactersByCorporation(ctx, corpID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list corporation characters")
			return
		}
		var others []store.Character
		for _, c := range corpChars {
			if c.ID != id {
				others = append(others, c)
			}
		}
		if len(others) == 0 {
			// Last member — delete corporation and all its data.
			if err := r.q.DeleteBlueprintsByOwner(ctx, store.DeleteBlueprintsByOwnerParams{
				OwnerType: "corporation", OwnerID: corpID,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete corporation blueprints")
				return
			}
			if err := r.q.DeleteJobsByOwner(ctx, store.DeleteJobsByOwnerParams{
				OwnerType: "corporation", OwnerID: corpID,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete corporation jobs")
				return
			}
			if err := r.q.DeleteSyncStateByOwner(ctx, store.DeleteSyncStateByOwnerParams{
				OwnerType: "corporation", OwnerID: corpID,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete corporation sync state")
				return
			}
			if err := r.q.DeleteCorporation(ctx, corpID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete corporation")
				return
			}
		} else {
			// Others remain — reassign delegate if this character holds it.
			corp, err := r.q.GetCorporation(ctx, corpID)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusInternalServerError, "failed to get corporation")
					return
				}
			} else if corp.DelegateID == id {
				if err := r.q.UpdateCorporationDelegate(ctx, store.UpdateCorporationDelegateParams{
					DelegateID: others[0].ID,
					ID:         corpID,
				}); err != nil {
					writeError(w, http.StatusInternalServerError, "failed to reassign delegate")
					return
				}
			}
		}
	}

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
