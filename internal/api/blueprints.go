package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

type jobJSON struct {
	ID        int64     `json:"id"`
	Activity  string    `json:"activity"`
	Status    string    `json:"status"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type blueprintJSON struct {
	ID           int64    `json:"id"`
	OwnerType    string   `json:"owner_type"`
	OwnerID      int64    `json:"owner_id"`
	OwnerName    string   `json:"owner_name"`
	TypeID       int64    `json:"type_id"`
	TypeName     string   `json:"type_name"`
	CategoryID   int64    `json:"category_id"`
	CategoryName string   `json:"category_name"`
	LocationID   int64    `json:"location_id"`
	MeLevel      int64    `json:"me_level"`
	TeLevel      int64    `json:"te_level"`
	Job          *jobJSON `json:"job"`
}

type characterSlotJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UsedSlots int64  `json:"used_slots"`
}

type summaryJSON struct {
	IdleBlueprints    int64               `json:"idle_blueprints"`
	OverdueJobs       int64               `json:"overdue_jobs"`
	CompletingToday   int64               `json:"completing_today"`
	FreeResearchSlots int64               `json:"free_research_slots"`
	Characters        []characterSlotJSON `json:"characters"`
}

// Handles:
//
//	GET /api/blueprints  (query params: status, owner_id, owner_type, category_id)
func (r *router) handleGetBlueprints(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()

	params := store.ListBlueprintsParams{}

	if v := q.Get("owner_type"); v != "" {
		params.OwnerType = v
	}
	if v := q.Get("owner_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid owner_id")
			return
		}
		params.OwnerID = id
	}
	if v := q.Get("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid category_id")
			return
		}
		params.CategoryID = id
	}
	if v := q.Get("status"); v != "" {
		params.Status = v
	}

	rows, err := r.q.ListBlueprints(req.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list blueprints")
		return
	}

	resp := make([]blueprintJSON, len(rows))
	for i, row := range rows {
		bp := blueprintJSON{
			ID:           row.ID,
			OwnerType:    row.OwnerType,
			OwnerID:      row.OwnerID,
			OwnerName:    row.OwnerName,
			TypeID:       row.TypeID,
			TypeName:     row.TypeName,
			CategoryID:   row.CategoryID,
			CategoryName: row.CategoryName,
			LocationID:   row.LocationID,
			MeLevel:      row.MeLevel,
			TeLevel:      row.TeLevel,
		}
		if row.JobID.Valid {
			bp.Job = &jobJSON{
				ID:        row.JobID.Int64,
				Activity:  row.JobActivity.String,
				Status:    row.JobStatus.String,
				StartDate: row.JobStartDate.Time,
				EndDate:   row.JobEndDate.Time,
			}
		}
		resp[i] = bp
	}
	writeJSON(w, http.StatusOK, resp)
}

// Handles:
//
//	GET /api/jobs/summary
func (r *router) handleGetJobsSummary(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idle, err := r.q.CountIdleBlueprints(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count idle blueprints")
		return
	}

	overdue, err := r.q.CountOverdueJobs(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count overdue jobs")
		return
	}

	completing, err := r.q.CountCompletingToday(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count completing today")
		return
	}

	slotRows, err := r.q.ListCharacterSlotUsage(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list slot usage")
		return
	}

	chars := make([]characterSlotJSON, len(slotRows))
	for i, row := range slotRows {
		chars[i] = characterSlotJSON{
			ID:        row.ID,
			Name:      row.Name,
			UsedSlots: row.UsedSlots,
		}
	}

	// FreeResearchSlots requires per-character skill data from ESI (not stored in MVP).
	writeJSON(w, http.StatusOK, summaryJSON{
		IdleBlueprints:    idle,
		OverdueJobs:       overdue,
		CompletingToday:   completing,
		FreeResearchSlots: 0,
		Characters:        chars,
	})
}
