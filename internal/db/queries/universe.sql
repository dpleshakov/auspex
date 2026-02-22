-- sqlc queries for eve_types, eve_groups, eve_categories tables.
-- See https://docs.sqlc.dev for query annotation syntax.

-- name: InsertEveCategory :exec
INSERT OR IGNORE INTO eve_categories (id, name) VALUES (?, ?);

-- name: InsertEveGroup :exec
INSERT OR IGNORE INTO eve_groups (id, category_id, name) VALUES (?, ?, ?);

-- name: InsertEveType :exec
INSERT OR IGNORE INTO eve_types (id, group_id, name) VALUES (?, ?, ?);

-- name: GetEveType :one
SELECT id, group_id, name FROM eve_types WHERE id = ?;
