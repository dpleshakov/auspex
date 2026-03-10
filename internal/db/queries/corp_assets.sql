-- sqlc queries for the corp_assets table.

-- name: UpsertCorpAsset :exec
INSERT OR REPLACE INTO corp_assets (item_id, owner_id, location_id, location_type)
VALUES (?, ?, ?, ?);

-- name: GetCorpAsset :one
SELECT location_id, location_type FROM corp_assets WHERE item_id = ?;

-- name: DeleteCorpAssetsByOwner :exec
DELETE FROM corp_assets WHERE owner_id = ?;
