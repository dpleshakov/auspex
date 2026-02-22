-- sqlc queries for the jobs table.
-- See https://docs.sqlc.dev for query annotation syntax.

-- name: UpsertJob :exec
INSERT INTO jobs (id, blueprint_id, owner_type, owner_id, installer_id, activity, status, start_date, end_date, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    blueprint_id = excluded.blueprint_id,
    owner_type   = excluded.owner_type,
    owner_id     = excluded.owner_id,
    installer_id = excluded.installer_id,
    activity     = excluded.activity,
    status       = excluded.status,
    start_date   = excluded.start_date,
    end_date     = excluded.end_date,
    updated_at   = excluded.updated_at;

-- name: DeleteJobsByOwner :exec
DELETE FROM jobs WHERE owner_type = ? AND owner_id = ?;

-- name: DeleteJobByID :exec
DELETE FROM jobs WHERE id = ?;

-- name: ListJobIDsByOwner :many
SELECT id FROM jobs WHERE owner_type = ? AND owner_id = ?;

-- name: CountIdleBlueprints :one
SELECT COUNT(*) FROM blueprints b
WHERE NOT EXISTS (
    SELECT 1 FROM jobs j WHERE j.blueprint_id = b.id
);

-- name: CountOverdueJobs :one
SELECT COUNT(*) FROM jobs
WHERE status = 'ready' AND end_date < datetime('now');

-- name: CountCompletingToday :one
SELECT COUNT(*) FROM jobs
WHERE status = 'active' AND date(end_date) = date('now');

-- name: ListCharacterSlotUsage :many
SELECT
    c.id,
    c.name,
    COUNT(j.id) AS used_slots
FROM characters c
LEFT JOIN jobs j ON j.installer_id = c.id
GROUP BY c.id, c.name
ORDER BY c.name;
