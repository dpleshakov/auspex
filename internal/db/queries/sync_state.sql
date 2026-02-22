-- sqlc queries for the sync_state table.
-- See https://docs.sqlc.dev for query annotation syntax.

-- name: GetSyncState :one
SELECT owner_type, owner_id, endpoint, last_sync, cache_until
FROM sync_state
WHERE owner_type = ? AND owner_id = ? AND endpoint = ?;

-- name: UpsertSyncState :exec
INSERT INTO sync_state (owner_type, owner_id, endpoint, last_sync, cache_until)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(owner_type, owner_id, endpoint) DO UPDATE SET
    last_sync   = excluded.last_sync,
    cache_until = excluded.cache_until;

-- name: ListSyncStatus :many
SELECT
    ss.owner_type,
    ss.owner_id,
    COALESCE(c.name, corp.name, '') AS owner_name,
    ss.endpoint,
    ss.last_sync,
    ss.cache_until
FROM sync_state ss
LEFT JOIN characters c ON ss.owner_type = 'character' AND c.id = ss.owner_id
LEFT JOIN corporations corp ON ss.owner_type = 'corporation' AND corp.id = ss.owner_id
ORDER BY ss.owner_type, ss.owner_id, ss.endpoint;

-- name: DeleteSyncStateByOwner :exec
DELETE FROM sync_state WHERE owner_type = ? AND owner_id = ?;
