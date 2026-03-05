-- sqlc queries for the characters table.
-- See https://docs.sqlc.dev for query annotation syntax.

-- name: GetCharacter :one
SELECT id, name, access_token, refresh_token, token_expiry, created_at, corporation_id, corporation_name
FROM characters
WHERE id = ?;

-- name: ListCharacters :many
SELECT id, name, access_token, refresh_token, token_expiry, created_at, corporation_id, corporation_name
FROM characters
ORDER BY name;

-- name: ListCharactersByCorporation :many
SELECT id, name, access_token, refresh_token, token_expiry, created_at, corporation_id, corporation_name
FROM characters
WHERE corporation_id = ?
ORDER BY name;

-- name: UpsertCharacter :exec
INSERT INTO characters (id, name, access_token, refresh_token, token_expiry, corporation_id, corporation_name)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name             = excluded.name,
    access_token     = excluded.access_token,
    refresh_token    = excluded.refresh_token,
    token_expiry     = excluded.token_expiry,
    corporation_id   = excluded.corporation_id,
    corporation_name = excluded.corporation_name;

-- name: DeleteCharacter :exec
DELETE FROM characters WHERE id = ?;
