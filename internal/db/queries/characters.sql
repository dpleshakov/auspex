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

-- name: ListCharactersWithMeta :many
SELECT
  ch.id,
  ch.name,
  ch.corporation_id,
  ch.corporation_name,
  ch.created_at,
  CASE WHEN corp.id IS NOT NULL THEN 1 ELSE 0 END AS is_delegate,
  CASE WHEN corp.id IS NOT NULL THEN (
    SELECT last_error FROM sync_state
    WHERE owner_type = 'corporation' AND owner_id = ch.corporation_id AND last_error IS NOT NULL
    LIMIT 1
  ) ELSE NULL END AS sync_error
FROM characters ch
LEFT JOIN corporations corp ON corp.delegate_id = ch.id
ORDER BY ch.name;

-- name: DeleteCharacter :exec
DELETE FROM characters WHERE id = ?;
