-- sqlc queries for the blueprints table.
-- See https://docs.sqlc.dev for query annotation syntax.

-- name: UpsertBlueprint :exec
INSERT INTO blueprints (id, owner_type, owner_id, type_id, location_id, me_level, te_level, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    owner_type  = excluded.owner_type,
    owner_id    = excluded.owner_id,
    type_id     = excluded.type_id,
    location_id = excluded.location_id,
    me_level    = excluded.me_level,
    te_level    = excluded.te_level,
    updated_at  = excluded.updated_at;

-- name: DeleteBlueprintsByOwner :exec
DELETE FROM blueprints WHERE owner_type = ? AND owner_id = ?;

-- name: ListBlueprintTypeIDsByOwner :many
SELECT DISTINCT type_id
FROM blueprints
WHERE owner_type = ? AND owner_id = ?;

-- name: ListBlueprints :many
SELECT
    b.id,
    b.owner_type,
    b.owner_id,
    COALESCE(c.name, corp.name, '') AS owner_name,
    b.type_id,
    t.name AS type_name,
    t.group_id,
    g.name AS group_name,
    g.category_id,
    cat.name AS category_name,
    b.location_id,
    b.me_level,
    b.te_level,
    b.updated_at,
    j.id           AS job_id,
    j.activity     AS job_activity,
    j.status       AS job_status,
    j.start_date   AS job_start_date,
    j.end_date     AS job_end_date,
    j.installer_id AS job_installer_id,
    ic.name        AS job_installer_name
FROM blueprints b
JOIN eve_types t ON t.id = b.type_id
JOIN eve_groups g ON g.id = t.group_id
JOIN eve_categories cat ON cat.id = g.category_id
LEFT JOIN jobs j ON j.blueprint_id = b.id
LEFT JOIN characters c ON b.owner_type = 'character' AND c.id = b.owner_id
LEFT JOIN corporations corp ON b.owner_type = 'corporation' AND corp.id = b.owner_id
LEFT JOIN characters ic ON ic.id = j.installer_id
WHERE
    (sqlc.narg('owner_type') IS NULL OR b.owner_type = sqlc.narg('owner_type'))
    AND (sqlc.narg('owner_id') IS NULL OR b.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('category_id') IS NULL OR g.category_id = sqlc.narg('category_id'))
    AND (
        sqlc.narg('status') IS NULL
        OR (sqlc.narg('status') = 'idle' AND j.id IS NULL)
        OR (sqlc.narg('status') = 'active' AND j.status = 'active')
        OR (sqlc.narg('status') = 'ready' AND j.status = 'ready')
    )
ORDER BY b.id;
