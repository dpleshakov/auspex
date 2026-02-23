package api

import (
	"net/http"
	"time"
)

type syncStatusItemJSON struct {
	OwnerType  string    `json:"owner_type"`
	OwnerID    int64     `json:"owner_id"`
	OwnerName  string    `json:"owner_name"`
	Endpoint   string    `json:"endpoint"`
	LastSync   time.Time `json:"last_sync"`
	CacheUntil time.Time `json:"cache_until"`
}

// Handles:
//
//	POST /api/sync         (sends force-refresh signal to the sync worker via channel)
//	GET  /api/sync/status
func (r *router) handlePostSync(w http.ResponseWriter, req *http.Request) {
	r.worker.ForceRefresh()
	w.WriteHeader(http.StatusAccepted)
}

func (r *router) handleGetSyncStatus(w http.ResponseWriter, req *http.Request) {
	rows, err := r.q.ListSyncStatus(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query sync status")
		return
	}

	items := make([]syncStatusItemJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, syncStatusItemJSON{
			OwnerType:  row.OwnerType,
			OwnerID:    row.OwnerID,
			OwnerName:  row.OwnerName,
			Endpoint:   row.Endpoint,
			LastSync:   row.LastSync,
			CacheUntil: row.CacheUntil,
		})
	}

	writeJSON(w, http.StatusOK, items)
}
