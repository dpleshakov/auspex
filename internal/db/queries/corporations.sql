-- sqlc queries for the corporations table.
-- See https://docs.sqlc.dev for query annotation syntax.

-- name: GetCorporation :one
SELECT id, name, delegate_id, created_at
FROM corporations
WHERE id = ?;

-- name: ListCorporations :many
SELECT c.id, c.name, c.delegate_id, ch.name AS delegate_name, c.created_at
FROM corporations c
JOIN characters ch ON ch.id = c.delegate_id
ORDER BY c.name;

-- name: InsertCorporation :exec
INSERT INTO corporations (id, name, delegate_id)
VALUES (?, ?, ?);

-- name: DeleteCorporation :exec
DELETE FROM corporations WHERE id = ?;
